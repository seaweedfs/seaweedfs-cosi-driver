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

package driver

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
	"github.com/seaweedfs/seaweedfs/weed/s3api/s3_constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

/* -------------------------------------------------------------------------- */
/*                               type & helpers                               */
/* -------------------------------------------------------------------------- */

// provisionerServer implements cosi.ProvisionerServer.
type provisionerServer struct {
	provisioner      string
	filerBucketsPath string
	endpoint         string
	region           string
	filerEndpoint    string
	grpcDialOption   grpc.DialOption
	mu               sync.Mutex // protects IAM read-modify-write
}

var _ cosispec.ProvisionerServer = (*provisionerServer)(nil)

const (
	paramDisk        = "disk"
	paramReplication = "replication"
)

// replicationPattern matches a valid SeaweedFS replication string: exactly 3 digits.
// Each digit encodes the number of replicas at a given topology level (DC, rack, node).
// See https://github.com/seaweedfs/seaweedfs/wiki/Replication
var replicationPattern = regexp.MustCompile(`^\d{3}$`)

// needsFilerConf reports whether any FilerConf-related parameters are set.
func needsFilerConf(params map[string]string) bool {
	return params[paramDisk] != "" || params[paramReplication] != ""
}

// validateBucketParams checks that replication value is a valid 3-digit string.
// The disk parameter is a free-format tag accepted as-is by SeaweedFS.
func validateBucketParams(params map[string]string) error {
	if v := params[paramReplication]; v != "" {
		if !replicationPattern.MatchString(v) {
			return fmt.Errorf("invalid replication %q: must be exactly 3 digits", v)
		}
	}
	return nil
}

// createFilerClient returns a fresh gRPC conn + typed client.
func createFilerClient(ctx context.Context, ep string, opt grpc.DialOption) (*grpc.ClientConn, filer_pb.SeaweedFilerClient, error) {
	conn, err := grpc.DialContext(ctx, ep, opt)
	if err != nil {
		return nil, nil, err
	}
	return conn, filer_pb.NewSeaweedFilerClient(conn), nil
}

// getFilerBucketsPath returns the directory path used for buckets.
func getFilerBucketsPath() string { return "/buckets" }

/* -------------------------------------------------------------------------- */
/*                         constructor & connection pool                      */
/* -------------------------------------------------------------------------- */

func NewProvisionerServer(prov, filerEP, endpoint, region string, opt grpc.DialOption) (cosispec.ProvisionerServer, error) {
	return &provisionerServer{
		provisioner:      prov,
		filerBucketsPath: getFilerBucketsPath(),
		endpoint:         endpoint,
		region:           region,
		filerEndpoint:    filerEP,
		grpcDialOption:   opt,
	}, nil
}

// withFilerClient opens a short-lived conn, executes fn, closes conn.
func (s *provisionerServer) withFilerClient(ctx context.Context, fn func(filer_pb.SeaweedFilerClient) error) error {
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // hard cap
	defer cancel()

	conn, cli, err := createFilerClient(dialCtx, s.filerEndpoint, s.grpcDialOption)
	if err != nil {
		return fmt.Errorf("dial filer: %w", err)
	}
	defer conn.Close()

	return fn(cli)
}

/* -------------------------------------------------------------------------- */
/*                           bucket primitives                                */
/* -------------------------------------------------------------------------- */

func (s *provisionerServer) createBucket(ctx context.Context, name string, params map[string]string) error {
	return s.withFilerClient(ctx, func(c filer_pb.SeaweedFilerClient) error {
		now := time.Now().Unix()
		entry := &filer_pb.Entry{
			Name:        name,
			IsDirectory: true,
			Attributes: &filer_pb.FuseAttributes{
				FileMode: uint32(0777 | os.ModeDir),
				Crtime:   now,
				Mtime:    now,
			},
		}

		if params["objectLockEnabled"] == "true" {
			entry.Extended = map[string][]byte{
				s3_constants.ExtVersioningKey:        []byte(s3_constants.VersioningEnabled),
				s3_constants.ExtObjectLockEnabledKey: []byte(s3_constants.ObjectLockEnabled),
			}
			if mode := params["objectLockRetentionMode"]; mode != "" {
				entry.Extended[s3_constants.ExtObjectLockDefaultModeKey] = []byte(mode)
				if days := params["objectLockRetentionDays"]; days != "" {
					entry.Extended[s3_constants.ExtObjectLockDefaultDaysKey] = []byte(days)
				}
				if years := params["objectLockRetentionYears"]; years != "" {
					entry.Extended[s3_constants.ExtObjectLockDefaultYearsKey] = []byte(years)
				}
			}
		}

		_, err := c.CreateEntry(ctx, &filer_pb.CreateEntryRequest{
			Directory: s.filerBucketsPath,
			Entry:     entry,
		})
		if err != nil {
			return err
		}
		if !needsFilerConf(params) {
			return nil
		}
		fc, err := readFilerConf(ctx, c)
		if err != nil {
			return err
		}
		if err := fc.SetLocationConf(&filer_pb.FilerConf_PathConf{
			LocationPrefix: s.filerBucketsPath + "/" + name + "/",
			DiskType:       params[paramDisk],
			Replication:    params[paramReplication],
		}); err != nil {
			return fmt.Errorf("set location conf: %w", err)
		}
		return saveFilerConf(ctx, c, fc)
	})
}

func (s *provisionerServer) deleteBucket(ctx context.Context, id string) error {
	return s.withFilerClient(ctx, func(c filer_pb.SeaweedFilerClient) error {
		_, err := c.DeleteEntry(ctx, &filer_pb.DeleteEntryRequest{
			Directory:            s.filerBucketsPath,
			Name:                 id,
			IsDeleteData:         true,
			IsRecursive:          true,
			IgnoreRecursiveError: true,
		})
		if err != nil {
			if isNotFoundError(err) {
				return nil // already gone — idempotent delete
			}
			return err
		}
		fc, err := readFilerConf(ctx, c)
		if err != nil {
			return err
		}
		prefix := s.filerBucketsPath + "/" + id + "/"
		if _, found := fc.GetLocationConf(prefix); !found {
			return nil
		}
		fc.DeleteLocationConf(prefix)
		return saveFilerConf(ctx, c, fc)
	})
}

/* -------------------------------------------------------------------------- */
/*                                 COSI RPCs                                  */
/* -------------------------------------------------------------------------- */

func (s *provisionerServer) DriverCreateBucket(ctx context.Context, req *cosispec.DriverCreateBucketRequest) (*cosispec.DriverCreateBucketResponse, error) {
	klog.InfoS("creating bucket", "name", req.GetName())
	params := req.GetParameters()
	if err := validateObjectLockParams(params); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := validateBucketParams(params); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.createBucket(ctx, req.GetName(), params); err != nil {
		klog.ErrorS(err, "create bucket failed")
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.InfoS("created bucket", "name", req.GetName())
	return &cosispec.DriverCreateBucketResponse{BucketId: req.GetName()}, nil
}

func (s *provisionerServer) DriverDeleteBucket(ctx context.Context, req *cosispec.DriverDeleteBucketRequest) (*cosispec.DriverDeleteBucketResponse, error) {
	klog.InfoS("deleting bucket", "name", req.GetBucketId())
	if err := s.deleteBucket(ctx, req.GetBucketId()); err != nil {
		klog.ErrorS(err, "delete bucket failed")
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.InfoS("deleted bucket", "name", req.GetBucketId())
	return &cosispec.DriverDeleteBucketResponse{}, nil
}

func (s *provisionerServer) DriverGrantBucketAccess(ctx context.Context, req *cosispec.DriverGrantBucketAccessRequest) (*cosispec.DriverGrantBucketAccessResponse, error) {
	klog.InfoS("granting bucket access", "user", req.GetName(), "bucket", req.GetBucketId())
	user, bucket := req.GetName(), req.GetBucketId()
	if user == "" || bucket == "" {
		return nil, status.Error(codes.InvalidArgument, "user or bucket empty")
	}

	// determine IAM actions based on accessPolicy parameter
	actions, err := actionsForPolicy(req.GetParameters()["accessPolicy"])
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accessKey, _ := GenerateAccessKeyID()
	secretKey, _ := GenerateSecretAccessKey()

	// atomic read-modify-write IAM configuration
	s.mu.Lock()
	defer s.mu.Unlock()

	var cfgBuf bytes.Buffer
	if err := s.readS3Configuration(ctx, &cfgBuf); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	cfg := &iam_pb.S3ApiConfiguration{}
	if cfgBuf.Len() > 0 {
		if err := filer.ParseS3ConfigurationFromBytes(cfgBuf.Bytes(), cfg); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// ensure identity exists
	var id *iam_pb.Identity
	for _, i := range cfg.Identities {
		if i.Name == user {
			id = i
			break
		}
	}
	if id == nil {
		id = &iam_pb.Identity{Name: user}
		cfg.Identities = append(cfg.Identities, id)
	}
	id.Credentials = append(id.Credentials, &iam_pb.Credential{AccessKey: accessKey, SecretKey: secretKey})
	for _, a := range actions {
		action := fmt.Sprintf("%s:%s", a, bucket)
		if !contains(id.Actions, action) {
			id.Actions = append(id.Actions, action)
		}
	}

	// write back
	cfgBuf.Reset()
	filer.ProtoToText(&cfgBuf, cfg)
	if err := s.saveS3Configuration(ctx, cfgBuf.Bytes()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	klog.InfoS("granted bucket access", "user", user, "bucket", bucket, "accessKey", accessKey)
	return &cosispec.DriverGrantBucketAccessResponse{
		AccountId: user,
		Credentials: map[string]*cosispec.CredentialDetails{
			"s3": {Secrets: map[string]string{
				"accessKeyID":     accessKey,
				"accessSecretKey": secretKey,
				"endpoint":        s.endpoint,
				"region":          s.region,
			}},
		},
	}, nil
}

func (s *provisionerServer) DriverRevokeBucketAccess(ctx context.Context, req *cosispec.DriverRevokeBucketAccessRequest) (*cosispec.DriverRevokeBucketAccessResponse, error) {
	klog.InfoS("revoking bucket access", "user", req.GetAccountId(), "bucket", req.GetBucketId())
	user := req.GetAccountId()
	if user == "" {
		return nil, status.Error(codes.InvalidArgument, "user empty")
	}
	if err := s.revokeBucketAccess(ctx, user); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.InfoS("revoked bucket access", "user", user, "bucket", req.GetBucketId())
	return &cosispec.DriverRevokeBucketAccessResponse{}, nil
}

/* -------------------------------------------------------------------------- */
/*                       FilerConf read / write helpers                       */
/* -------------------------------------------------------------------------- */

// readFilerConf loads FilerConf from the filer, returning an empty conf if
// the file does not exist yet.
//
// TODO: the read-modify-write cycle (readFilerConf -> modify -> saveFilerConf)
// is not atomic. Concurrent bucket operations may overwrite each other's
// PathConf entries. This is the same limitation as the IAM read-modify-write
// in configureS3Access. Consider filer-side locking or optimistic concurrency
// if concurrent bucket creation becomes a concern.
func readFilerConf(ctx context.Context, c filer_pb.SeaweedFilerClient) (*filer.FilerConf, error) {
	fc := filer.NewFilerConf()
	resp, err := c.LookupDirectoryEntry(ctx, &filer_pb.LookupDirectoryEntryRequest{
		Directory: filer.DirectoryEtcSeaweedFS,
		Name:      filer.FilerConfName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "no entry is found") {
			return fc, nil
		}
		return nil, fmt.Errorf("read filer conf: %w", err)
	}
	if resp.Entry != nil && resp.Entry.Content != nil {
		if err := fc.LoadFromBytes(resp.Entry.Content); err != nil {
			return nil, fmt.Errorf("parse filer conf: %w", err)
		}
	}
	return fc, nil
}

// saveFilerConf serialises fc and writes it back to the filer (update or
// create, same pattern as saveS3Configuration).
func saveFilerConf(ctx context.Context, c filer_pb.SeaweedFilerClient, fc *filer.FilerConf) error {
	var buf bytes.Buffer
	if err := fc.ToText(&buf); err != nil {
		return fmt.Errorf("serialize filer conf: %w", err)
	}
	data := buf.Bytes()

	_, err := c.UpdateEntry(ctx, &filer_pb.UpdateEntryRequest{
		Directory: filer.DirectoryEtcSeaweedFS,
		Entry: &filer_pb.Entry{
			Name:        filer.FilerConfName,
			Content:     data,
			IsDirectory: false,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no entry is found") {
		return err
	}
	_, err = c.CreateEntry(ctx, &filer_pb.CreateEntryRequest{
		Directory: filer.DirectoryEtcSeaweedFS,
		Entry: &filer_pb.Entry{
			Name:        filer.FilerConfName,
			Content:     data,
			IsDirectory: false,
		},
	})
	return err
}

/* -------------------------------------------------------------------------- */
/*                          IAM read / write helpers                          */
/* -------------------------------------------------------------------------- */

func (s *provisionerServer) readS3Configuration(ctx context.Context, buf *bytes.Buffer) error {
	return s.withFilerClient(ctx, func(c filer_pb.SeaweedFilerClient) error {
		resp, err := c.LookupDirectoryEntry(ctx, &filer_pb.LookupDirectoryEntryRequest{
			Directory: filer.IamConfigDirectory,
			Name:      filer.IamIdentityFile,
		})
		if err != nil && !isNotFoundError(err) {
			return err
		}
		if resp != nil && resp.Entry != nil && resp.Entry.Content != nil {
			buf.Write(resp.Entry.Content)
		}
		return nil
	})
}

func (s *provisionerServer) saveS3Configuration(ctx context.Context, data []byte) error {
	return s.withFilerClient(ctx, func(c filer_pb.SeaweedFilerClient) error {
		_, err := c.UpdateEntry(ctx, &filer_pb.UpdateEntryRequest{
			Directory: filer.IamConfigDirectory,
			Entry: &filer_pb.Entry{
				Name:        filer.IamIdentityFile,
				Content:     data,
				IsDirectory: false,
			},
		})
		if err == nil || !isNotFoundError(err) {
			return err
		}
		_, err = c.CreateEntry(ctx, &filer_pb.CreateEntryRequest{
			Directory: filer.IamConfigDirectory,
			Entry: &filer_pb.Entry{
				Name:        filer.IamIdentityFile,
				Content:     data,
				IsDirectory: false,
			},
		})
		return err
	})
}

func (s *provisionerServer) revokeBucketAccess(ctx context.Context, user string) error {
	return s.configureS3Access(ctx, user, "", "", nil, true)
}

func (s *provisionerServer) configureS3Access(ctx context.Context, user, ak, sk string, actions []string, del bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var buf bytes.Buffer
	if err := s.readS3Configuration(ctx, &buf); err != nil {
		return err
	}

	cfg := &iam_pb.S3ApiConfiguration{}
	if buf.Len() > 0 {
		if err := filer.ParseS3ConfigurationFromBytes(buf.Bytes(), cfg); err != nil {
			return err
		}
	}

	// update cfg …
	idx := -1
	for i, id := range cfg.Identities {
		if id.Name == user {
			idx = i
			break
		}
	}

	if del {
		if idx >= 0 {
			cfg.Identities = append(cfg.Identities[:idx], cfg.Identities[idx+1:]...)
		}
	} else {
		if idx == -1 {
			cfg.Identities = append(cfg.Identities, &iam_pb.Identity{Name: user})
			idx = len(cfg.Identities) - 1
		}
		id := cfg.Identities[idx]
		if ak != "" && sk != "" {
			id.Credentials = append(id.Credentials, &iam_pb.Credential{AccessKey: ak, SecretKey: sk})
		}
		for _, a := range actions {
			if !contains(id.Actions, a) {
				id.Actions = append(id.Actions, a)
			}
		}
	}

	buf.Reset()
	filer.ProtoToText(&buf, cfg)
	return s.saveS3Configuration(ctx, buf.Bytes())
}

/* -------------------------------------------------------------------------- */
/*                               utilities                                    */
/* -------------------------------------------------------------------------- */

// isNotFoundError checks whether err represents a "not found" condition.
// It first checks for a proper gRPC NotFound status code, then falls back to
// string matching for SeaweedFS filer which returns plain error strings.
func isNotFoundError(err error) bool {
	if status.Code(err) == codes.NotFound {
		return true
	}
	return strings.Contains(err.Error(), "no entry is found")
}

// validateObjectLockParams checks BucketClass parameters related to Object Lock.
func validateObjectLockParams(params map[string]string) error {
	rawEnabled := params["objectLockEnabled"]
	var enabled bool
	switch rawEnabled {
	case "true":
		enabled = true
	case "false", "":
		enabled = false
	default:
		return fmt.Errorf("objectLockEnabled must be \"true\" or \"false\", got %q", rawEnabled)
	}

	mode := params["objectLockRetentionMode"]
	days := params["objectLockRetentionDays"]
	years := params["objectLockRetentionYears"]

	if !enabled {
		if mode != "" || days != "" || years != "" {
			return fmt.Errorf("objectLockEnabled must be true when retention parameters are set")
		}
		return nil
	}

	if mode != "" && mode != "GOVERNANCE" && mode != "COMPLIANCE" {
		return fmt.Errorf("objectLockRetentionMode must be GOVERNANCE or COMPLIANCE, got %q", mode)
	}

	if days != "" && years != "" {
		return fmt.Errorf("objectLockRetentionDays and objectLockRetentionYears are mutually exclusive")
	}

	if days != "" {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return fmt.Errorf("objectLockRetentionDays must be a positive integer (greater than 0), got %q", days)
		}
	}

	if years != "" {
		n, err := strconv.Atoi(years)
		if err != nil || n <= 0 {
			return fmt.Errorf("objectLockRetentionYears must be a positive integer (greater than 0), got %q", years)
		}
	}

	hasMode := mode != ""
	hasPeriod := days != "" || years != ""
	if hasMode != hasPeriod {
		if hasMode {
			return fmt.Errorf("objectLockRetentionMode requires objectLockRetentionDays or objectLockRetentionYears")
		}
		return fmt.Errorf("objectLockRetentionDays/Years requires objectLockRetentionMode")
	}

	return nil
}

// actionsForPolicy returns the IAM action names for the given access policy.
// Supported values: "readonly", "readwrite", "" (defaults to readwrite).
func actionsForPolicy(policy string) ([]string, error) {
	switch policy {
	case "readonly":
		return []string{"Read", "List"}, nil
	case "readwrite", "":
		return []string{"Read", "Write", "List", "Tagging"}, nil
	default:
		return nil, fmt.Errorf("unsupported accessPolicy %q, must be \"readonly\" or \"readwrite\"", policy)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func GenerateAccessKeyID() (string, error) {
	return randomString(20, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

func GenerateSecretAccessKey() (string, error) {
	return randomString(40, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789/+")
}

func randomString(n int, charset string) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}
