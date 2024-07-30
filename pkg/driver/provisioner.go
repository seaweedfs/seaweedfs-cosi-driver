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

func createFilerClient(filerEndpoint, accessKey, secretKey string) (filer_pb.SeaweedFilerClient, error) {
	// Logic to connect to SeaweedFS filer
	return nil, fmt.Errorf("not implemented")
}

func getFilerBucketsPath(filerClient filer_pb.SeaweedFilerClient) (string, error) {
	// Logic to get the path where buckets are stored
	return "", fmt.Errorf("not implemented")
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

func (s *provisionerServer) createBucket(ctx context.Context, bucketName string) error {
	// Placeholder for bucket creation logic
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

func (s *provisionerServer) deleteBucket(ctx context.Context, bucketId string) error {
	// Placeholder for bucket deletion logic
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

func (s *provisionerServer) grantBucketAccess(ctx context.Context, bucketId, userId string) error {
	// Placeholder for access grant logic
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

func (s *provisionerServer) revokeBucketAccess(ctx context.Context, accountId string) error {
	// Placeholder for access revoke logic
	return nil
}
