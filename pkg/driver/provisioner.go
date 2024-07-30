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

package driver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

// provisionerServer implements cosi.ProvisionerServer interface.
type provisionerServer struct {
	provisioner      string
	filerClient      filer_pb.SeaweedFilerClient
	filerBucketsPath string
}

// Interface guards.
var _ cosispec.ProvisionerServer = &provisionerServer{}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create a new SeaweedFS Filer client for interacting with the Filer.
func createFilerClient(filerEndpoint, accessKey, secretKey, caCertPath, clientCertPath, clientKeyPath string) (filer_pb.SeaweedFilerClient, error) {
	// Load the CA certificate
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Create a certificate pool from the CA certificate
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	// Load the client certificates
	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificates: %w", err)
	}

	// Create the credentials
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	})

	conn, err := grpc.Dial(filerEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to filer: %w", err)
	}
	return filer_pb.NewSeaweedFilerClient(conn), nil
}

// Get the directory path in the Filer where buckets are stored.
func getFilerBucketsPath(filerClient filer_pb.SeaweedFilerClient) (string, error) {
	// Assuming a default bucket storage directory path or fetching it from Filer configuration
	return "/buckets", nil
}

// NewProvisionerServer returns provisioner.Server with initialized clients.
func NewProvisionerServer(provisioner, filerEndpoint, accessKey, secretKey, caCertPath, clientCertPath, clientKeyPath string) (cosispec.ProvisionerServer, error) {
	// Create filer client here
	filerClient, err := createFilerClient(filerEndpoint, accessKey, secretKey, caCertPath, clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	// Get filer buckets path
	filerBucketsPath, err := getFilerBucketsPath(filerClient)
	if err != nil {
		return nil, err
	}

	return &provisionerServer{
		provisioner:      provisioner,
		filerClient:      filerClient,
		filerBucketsPath: filerBucketsPath,
	}, nil
}

// Create a bucket in SeaweedFS using the Filer.
func (s *provisionerServer) createBucket(ctx context.Context, bucketName string) error {
	req := &filer_pb.CreateEntryRequest{
		Directory: s.filerBucketsPath,
		Entry: &filer_pb.Entry{
			Name:        bucketName,
			IsDirectory: true,
		},
	}
	_, err := s.filerClient.CreateEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create bucket in filer: %w", err)
	}
	return nil
}

// Delete a bucket in SeaweedFS using the Filer.
func (s *provisionerServer) deleteBucket(ctx context.Context, bucketId string) error {
	req := &filer_pb.DeleteEntryRequest{
		Directory: s.filerBucketsPath,
		Name:      bucketId,
	}
	_, err := s.filerClient.DeleteEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete bucket in filer: %w", err)
	}
	return nil
}

// DriverCreateBucket call is made to create the bucket in the backend.
func (s *provisionerServer) DriverCreateBucket(
	ctx context.Context,
	req *cosispec.DriverCreateBucketRequest,
) (*cosispec.DriverCreateBucketResponse, error) {
	klog.InfoS("creating bucket", "name", req.GetName())

	// Implement bucket creation logic using SeaweedFS filer client
	err := s.createBucket(ctx, req.GetName())
	if err != nil {
		klog.ErrorS(err, "failed to create bucket", "name", req.GetName())
		return nil, status.Error(codes.Internal, "failed to create bucket")
	}

	klog.InfoS("successfully created bucket", "name", req.GetName())
	return &cosispec.DriverCreateBucketResponse{
		BucketId: req.GetName(),
	}, nil
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
func (s *provisionerServer) DriverDeleteBucket(
	ctx context.Context,
	req *cosispec.DriverDeleteBucketRequest,
) (*cosispec.DriverDeleteBucketResponse, error) {
	klog.InfoS("deleting bucket", "id", req.GetBucketId())

	// Implement bucket deletion logic using SeaweedFS filer client
	err := s.deleteBucket(ctx, req.GetBucketId())
	if err != nil {
		klog.ErrorS(err, "failed to delete bucket", "id", req.GetBucketId())
		return nil, status.Error(codes.Internal, "failed to delete bucket")
	}

	klog.InfoS("successfully deleted bucket", "id", req.GetBucketId())
	return &cosispec.DriverDeleteBucketResponse{}, nil
}

// Grant access to a bucket.
func (s *provisionerServer) grantBucketAccess(ctx context.Context, bucketId, userId string) error {
	accessKey, err := randomHex(16)
	if err != nil {
		return fmt.Errorf("failed to generate access key: %w", err)
	}
	secretKey, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("failed to generate secret key: %w", err)
	}

	actions := []string{"Read", "Write", "List", "Tagging"}
	var cmdActions []string
	for _, action := range actions {
		cmdActions = append(cmdActions, fmt.Sprintf("%s:%s", action, bucketId))
	}

	err = s.configureS3Access(ctx, userId, accessKey, secretKey, cmdActions, false)
	if err != nil {
		return err
	}

	return nil
}

// Revoke access to a bucket.
func (s *provisionerServer) revokeBucketAccess(ctx context.Context, userId string) error {
	err := s.configureS3Access(ctx, userId, "", "", nil, true)
	if err != nil {
		return err
	}

	return nil
}

// Configure S3 access in SeaweedFS.
func (s *provisionerServer) configureS3Access(ctx context.Context, user, accessKey, secretKey string, actions []string, isDelete bool) error {
	var buf bytes.Buffer
	if err := s.readS3Configuration(ctx, &buf); err != nil {
		return err
	}

	s3cfg := &iam_pb.S3ApiConfiguration{}
	if buf.Len() > 0 {
		if err := filer.ParseS3ConfigurationFromBytes(buf.Bytes(), s3cfg); err != nil {
			return err
		}
	}

	idx := -1
	for i, identity := range s3cfg.Identities {
		if user == identity.Name {
			idx = i
			break
		}
	}

	if idx == -1 && isDelete {
		// User not found and trying to delete, nothing to do
		return nil
	}

	if idx == -1 {
		// Add new user
		identity := iam_pb.Identity{
			Name:        user,
			Actions:     actions,
			Credentials: []*iam_pb.Credential{},
		}
		if accessKey != "" && secretKey != "" {
			identity.Credentials = append(identity.Credentials, &iam_pb.Credential{
				AccessKey: accessKey,
				SecretKey: secretKey,
			})
		}
		s3cfg.Identities = append(s3cfg.Identities, &identity)
	} else {
		// Update existing user
		if isDelete {
			s3cfg.Identities = append(s3cfg.Identities[:idx], s3cfg.Identities[idx+1:]...)
		} else {
			if accessKey != "" && secretKey != "" {
				s3cfg.Identities[idx].Credentials = append(s3cfg.Identities[idx].Credentials, &iam_pb.Credential{
					AccessKey: accessKey,
					SecretKey: secretKey,
				})
			}
			for _, action := range actions {
				if !contains(s3cfg.Identities[idx].Actions, action) {
					s3cfg.Identities[idx].Actions = append(s3cfg.Identities[idx].Actions, action)
				}
			}
		}
	}

	buf.Reset()
	filer.ProtoToText(&buf, s3cfg)

	if err := s.saveS3Configuration(ctx, buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// Read the S3 configuration from the SeaweedFS Filer.
func (s *provisionerServer) readS3Configuration(ctx context.Context, buf *bytes.Buffer) error {
	entry, err := s.filerClient.LookupDirectoryEntry(ctx, &filer_pb.LookupDirectoryEntryRequest{
		Directory: filer.IamConfigDirectory,
		Name:      filer.IamIdentityFile,
	})
	if err != nil {
		return err
	}

	if entry.Entry != nil && entry.Entry.Content != nil {
		buf.Write(entry.Entry.Content)
	}

	return nil
}

// Save the S3 configuration to the SeaweedFS Filer.
func (s *provisionerServer) saveS3Configuration(ctx context.Context, data []byte) error {
	_, err := s.filerClient.UpdateEntry(ctx, &filer_pb.UpdateEntryRequest{
		Directory: filer.IamConfigDirectory,
		Entry: &filer_pb.Entry{
			Name:        filer.IamIdentityFile,
			Content:     data,
			IsDirectory: false,
		},
	})
	return err
}

// Helper function to check if a string slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// DriverGrantBucketAccess grants access to a bucket.
func (s *provisionerServer) DriverGrantBucketAccess(
	ctx context.Context,
	req *cosispec.DriverGrantBucketAccessRequest,
) (*cosispec.DriverGrantBucketAccessResponse, error) {
	userName := req.GetName()
	bucketName := req.GetBucketId()
	klog.V(5).Infof("req %v", req)
	klog.Info("Granting user accessPolicy to bucket ", "userName", userName, "bucketName", bucketName)

	// Generate random access and secret keys
	accessKey, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access key: %w", err)
	}
	secretKey, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret key: %w", err)
	}

	// Update IAM configuration
	err = s.configureS3Access(ctx, userName, accessKey, secretKey, []string{
		fmt.Sprintf("Read:%s", bucketName),
		fmt.Sprintf("Write:%s", bucketName),
		fmt.Sprintf("List:%s", bucketName),
		fmt.Sprintf("Tagging:%s", bucketName),
	}, false)
	if err != nil {
		klog.ErrorS(err, "failed to configure S3 access")
		return nil, status.Error(codes.Internal, "failed to configure S3 access")
	}

	// Create user and grant bucket access
	err = s.grantBucketAccess(ctx, bucketName, userName)
	if err != nil {
		klog.ErrorS(err, "failed to grant bucket access", "bucketName", bucketName, "userName", userName)
		return nil, status.Error(codes.Internal, "failed to grant bucket access")
	}

	// Prepare the response with generated credentials
	credentials := map[string]string{
		"accessKey": accessKey,
		"secretKey": secretKey,
	}

	klog.InfoS("Successfully granted bucket access", "bucketName", bucketName, "userName", userName)

	return &cosispec.DriverGrantBucketAccessResponse{
		AccountId: userName,
		Credentials: map[string]*cosispec.CredentialDetails{
			"s3": {
				Secrets: credentials,
			},
		},
	}, nil
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket.
func (s *provisionerServer) DriverRevokeBucketAccess(
	ctx context.Context,
	req *cosispec.DriverRevokeBucketAccessRequest,
) (*cosispec.DriverRevokeBucketAccessResponse, error) {
	klog.InfoS("revoking bucket access", "user", req.GetAccountId())

	// Implement access revoke logic using SeaweedFS filer client
	err := s.revokeBucketAccess(ctx, req.GetAccountId())
	if err != nil {
		klog.ErrorS(err, "failed to revoke access", "user", req.GetAccountId())
		return nil, status.Error(codes.Internal, "failed to revoke bucket access")
	}

	return &cosispec.DriverRevokeBucketAccessResponse{}, nil
}
