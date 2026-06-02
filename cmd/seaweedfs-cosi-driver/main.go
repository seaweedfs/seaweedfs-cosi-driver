/*
Copyright 2023 SUSE, LLC.
Copyright 2024 s3gw contributors.
Copyright 2024 SeaweedFS contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seaweedfs/seaweedfs-cosi-driver/pkg/driver"
	"github.com/seaweedfs/seaweedfs-cosi-driver/pkg/envflag"
	"github.com/seaweedfs/seaweedfs/weed/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
	"sigs.k8s.io/container-object-storage-interface-provisioner-sidecar/pkg/provisioner"
)

type runOptions struct {
	driverName    string
	cosiEndpoint  string
	filerEndpoint string
	endpoint      string
	region        string
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	opts := runOptions{
		driverName:    envflag.String("DRIVERNAME", "seaweedfs.objectstorage.k8s.io"),
		cosiEndpoint:  envflag.String("COSI_ENDPOINT", "unix:///var/lib/cosi/cosi.sock"),
		filerEndpoint: envflag.String("SEAWEEDFS_FILER", ""),
		endpoint:      envflag.String("ENDPOINT", ""),
		region:        envflag.String("REGION", ""),
	}

	if err := run(context.Background(), opts); err != nil {
		klog.ErrorS(err, "driver exited with error")
		os.Exit(1)
	}
}

// run constructs the SeaweedFS COSI driver and serves the provisioner gRPC API
// on the configured COSI endpoint until the context is cancelled.
func run(ctx context.Context, opts runOptions) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// TLS creds for the client
	util.LoadConfiguration("security", false)
	dialOpt := loadClientTLS()

	idSrv, provSrv, err := driver.NewDriver(ctx,
		opts.driverName,
		opts.filerEndpoint,
		opts.endpoint,
		opts.region,
		dialOpt,
	)
	if err != nil {
		return err
	}

	// A non-graceful exit (SIGKILL/OOM/panic) leaves the COSI unix socket file
	// behind — Go's net package only unlinks it on a graceful listener close.
	// The socket lives on an emptyDir that survives container restarts, so the
	// next bind fails with EADDRINUSE and wedges the pod in CrashLoopBackOff.
	// Clear only a stale (dead) socket so the driver self-heals on restart.
	if err := removeStaleSocket(opts.cosiEndpoint); err != nil {
		return err
	}

	cosiSrv, err := provisioner.NewDefaultCOSIProvisionerServer(opts.cosiEndpoint, idSrv, provSrv)
	if err != nil {
		return err
	}
	return cosiSrv.Run(ctx)
}

// removeStaleSocket clears a leftover COSI unix socket so the gRPC server can
// bind on restart. It parses the endpoint exactly like the provisioner sidecar
// (net/url), so the path it removes is the path that will be bound. It removes
// the path only if it exists, is a socket, and has no live listener — it never
// deletes a regular file/directory or a socket that is still being served.
func removeStaleSocket(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil
	}
	if u.Scheme != "unix" {
		return nil
	}
	path := u.Path
	if path == "" {
		return nil
	}

	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat COSI socket %q: %w", path, err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		// Not a socket (misconfigured endpoint) — never remove it.
		return nil
	}
	if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		// A live listener is still bound — leave it. The bind that follows will
		// surface the real "address already in use" if another instance runs.
		_ = c.Close()
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale COSI socket %q: %w", path, err)
	}
	return nil
}

func loadClientTLS() grpc.DialOption {
	certFileName := os.Getenv("WEED_GRPC_CLIENT_CERT")
	keyFileName := os.Getenv("WEED_GRPC_CLIENT_KEY")
	caFileName := os.Getenv("WEED_GRPC_CA")

	if certFileName == "" || keyFileName == "" || caFileName == "" {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// client certificate
	cert, err := tls.LoadX509KeyPair(certFileName, keyFileName)
	if err != nil {
		log.Fatalf("failed to load client cert/key: %v", err)
	}

	// root CA
	caCertPool := x509.NewCertPool()
	if caFileName != "" {
		caCert, err := os.ReadFile(caFileName)
		if err != nil {
			log.Fatalf("failed to load CA: %v", err)
		}
		caCertPool.AppendCertsFromPEM(caCert)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
}
