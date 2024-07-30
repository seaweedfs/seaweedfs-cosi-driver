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
	"context"
	"fmt"

	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"google.golang.org/grpc/codes"
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

// NewProvisionerServer returns provisioner.Server with initialized clients.
func NewProvisionerServer(provisioner, filerEndpoint, accessKey, secretKey string) (cosispec.ProvisionerServer, error) {
	// Create filer client here
	filerClient, err := createFilerClient(filerEndpoint, accessKey, secretKey)
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


// Create a new SeaweedFS Filer client for interacting with the Filer.
func createFilerClient(filerEndpoint, accessKey, secretKey string) (filer_pb.SeaweedFilerClient, error) {
	conn, err := grpc.Dial(filerEndpoint, grpc.WithInsecure()) // Assuming no TLS for simplicity
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

// DriverGrantBucketAccess call grants access to a bucket.
func (s *provisionerServer) DriverGrantBucketAccess(
	ctx context.Context,
	req *cosispec.DriverGrantBucketAccessRequest,
) (*cosispec.DriverGrantBucketAccessResponse, error) {
	klog.InfoS("granting bucket access", "bucket", req.GetBucketId(), "user", req.GetName())

	// Implement access grant logic using SeaweedFS filer client
	err := s.grantBucketAccess(ctx, req.GetBucketId(), req.GetName())
	if err != nil {
		klog.ErrorS(err, "failed to grant access", "bucket", req.GetBucketId(), "user", req.GetName())
		return nil, status.Error(codes.Internal, "failed to grant bucket access")
	}

	// Placeholder for generating access credentials
	credentials := map[string]string{
		"accessKey": "exampleAccessKey",
		"secretKey": "exampleSecretKey",
	}

	return &cosispec.DriverGrantBucketAccessResponse{
		AccountId: req.GetName(),
		Credentials: &cosispec.CredentialDetails{
			Secrets: credentials,
		},
	}, nil
}

// Grant access to a bucket. This example simply logs the action.
// In practice, this could involve setting permissions or policies at the Filer or IAM level.
func (s *provisionerServer) grantBucketAccess(ctx context.Context, bucketId, userId string) error {
	// Log the grant access action. Implement actual access control as required.
	klog.InfoS("Granted access to bucket", "bucketId", bucketId, "userId", userId)
	return nil
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


// Revoke access to a bucket. This example simply logs the action.
// In practice, this would involve removing permissions or policies.
func (s *provisionerServer) revokeBucketAccess(ctx context.Context, accountId string) error {
	// Log the revoke access action. Implement actual access control removal as required.
	klog.InfoS("Revoked access for account", "accountId", accountId)
	return nil
}
