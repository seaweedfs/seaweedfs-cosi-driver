/*
Copyright 2023 SUSE, LLC.
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
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/seaweedfs/seaweedfs-cosi-driver/pkg/driver"
	"github.com/seaweedfs/seaweedfs-cosi-driver/pkg/envflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/container-object-storage-interface-provisioner-sidecar/pkg/provisioner"
)

type runOptions struct {
	driverName    string
	cosiEndpoint  string
	accessKey     string
	secretKey     string
	filerEndpoint string
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	opts := runOptions{
		driverName:    envflag.String("DRIVERNAME", "seaweedfs.objectstorage.k8s.io"),
		cosiEndpoint:  envflag.String("COSI_ENDPOINT", "unix:///var/lib/cosi/cosi.sock"),
		accessKey:     envflag.String("ACCESSKEY", ""),
		secretKey:     envflag.String("SECRETKEY", ""),
		filerEndpoint: envflag.String("ENDPOINT", ""),
	}

	if err := run(context.Background(), opts); err != nil {
		klog.ErrorS(err, "exiting on error")
		os.Exit(1)
	}
}

func run(ctx context.Context, opts runOptions) error {
	ctx, stop := signal.NotifyContext(ctx,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	identityServer, provisionerServer, err := driver.NewDriver(ctx,
		opts.driverName,
		opts.filerEndpoint,
		opts.accessKey,
		opts.secretKey,
	)
	if err != nil {
		return err
	}

	server, err := provisioner.NewDefaultCOSIProvisionerServer(
		opts.cosiEndpoint,
		identityServer,
		provisionerServer,
	)
	if err != nil {
		return err
	}

	return server.Run(ctx)
}
