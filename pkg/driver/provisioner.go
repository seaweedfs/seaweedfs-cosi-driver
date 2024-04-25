/*
Copyright 2023 SUSE, LLC.
Copyright 2024 s3gw maintainers.

Licensed under the Apache License, Version 2.0 (the "License");
You may not use this file except in compliance with the License.
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
	"errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	rgwadmin "github.com/ceph/go-ceph/rgw/admin"
	"github.com/s3gw-tech/s3gw-cosi-driver/pkg/util/s3client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

// provisionerServer implements cosi.ProvisionerServer interface.
// It contains two clients:
// - s3Client for RGWAdminOps: mainly for user related operations
// - rgwAdminClient for S3 operations: mainly for bucket related operations
type provisionerServer struct {
	provisioner    string
	s3Client       *s3client.S3Agent
	rgwAdminClient *rgwadmin.API
}

// Interface guards.
var _ cosispec.ProvisionerServer = &provisionerServer{}

// NewProvisionerServer returns provisioner.Server with initialized clients.
func NewProvisionerServer(provisioner, rgwEndpoint, accessKey, secretKey string) (cosispec.ProvisionerServer, error) {
	// TODO: use different user this operation
	s3Client, err := s3client.NewS3Agent(accessKey, secretKey, rgwEndpoint, true)
	if err != nil {
		return nil, err
	}

	//TODO: add support for TLS endpoint
	rgwAdminClient, err := rgwadmin.New(rgwEndpoint, accessKey, secretKey, nil)
	if err != nil {
		return nil, err
	}

	return &provisionerServer{
		provisioner:    provisioner,
		s3Client:       s3Client,
		rgwAdminClient: rgwAdminClient,
	}, nil
}

// DriverCreateBucket call is made to create the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
//  1. If a bucket that matches both name and parameters already exists, then OK (success) must be returned.
//  2. If a bucket by same name, but different parameters is provided, then the appropriate error code ALREADY_EXISTS must be returned.
func (s *provisionerServer) DriverCreateBucket(
	ctx context.Context,
	req *cosispec.DriverCreateBucketRequest,
) (*cosispec.DriverCreateBucketResponse, error) {
	klog.InfoS("using ceph rgw to create backend bucket")

	bucketName := req.GetName()
	klog.V(3).InfoS("creating bucket",
		"name", bucketName)

	err := s.s3Client.CreateBucket(bucketName)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			klog.V(8).InfoS("after s3 call",
				"ok", ok,
				"aerr", aerr)

			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				klog.InfoS("bucket already exists",
					"name", bucketName)

				return nil, status.Error(codes.AlreadyExists, "bucket already exists")

			case s3.ErrCodeBucketAlreadyOwnedByYou:
				// TODO: validate if parameters are as expected

				klog.InfoS("bucket already owned by you",
					"name", bucketName)

				return &cosispec.DriverCreateBucketResponse{
					BucketId: bucketName,
				}, nil
			}
		}

		klog.ErrorS(err, "failed to create bucket",
			"bucketName", bucketName)

		return nil, status.Error(codes.Internal, "failed to create bucket")
	}

	klog.InfoS("successfully created backend bucket",
		"bucketName", bucketName)

	return &cosispec.DriverCreateBucketResponse{
		BucketId: bucketName,
	}, nil
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *provisionerServer) DriverDeleteBucket(
	ctx context.Context,
	req *cosispec.DriverDeleteBucketRequest,
) (*cosispec.DriverDeleteBucketResponse, error) {
	klog.InfoS("deleting bucket",
		"id", req.GetBucketId())

	if _, err := s.s3Client.DeleteBucket(req.GetBucketId()); err != nil {
		klog.ErrorS(err, "failed to delete bucket",
			"id", req.GetBucketId())

		return nil, status.Error(codes.Internal, "failed to delete bucket")
	}

	klog.InfoS("successfully deleted bucket",
		"id", req.GetBucketId())

	return &cosispec.DriverDeleteBucketResponse{}, nil
}

// DriverGrantBucketAccess call grants access to an account.
// The account_name in the request shall be used as a unique identifier to create credentials.
//
// NOTE: this call needs to be idempotent.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeBucketAccess.
// The returned secret does not need to be the same each call to achieve idempotency.
func (s *provisionerServer) DriverGrantBucketAccess(
	ctx context.Context,
	req *cosispec.DriverGrantBucketAccessRequest,
) (*cosispec.DriverGrantBucketAccessResponse, error) {
	// TODO: validate below details, Authenticationtype, Parameters
	userName := req.GetName()
	bucketName := req.GetBucketId()

	klog.InfoS("granting user accessPolicy to bucket",
		"userName", userName,
		"bucketName", bucketName)

	user, err := s.rgwAdminClient.CreateUser(ctx, rgwadmin.User{
		ID:          userName,
		DisplayName: userName,
	})

	// TODO: Do we need fail for UserErrorExists, or same account can have multiple BAR
	if err != nil && !errors.Is(err, rgwadmin.ErrUserExists) {
		klog.ErrorS(err, "failed to create user")

		return nil, status.Error(codes.Internal, "user creation failed")
	}

	// TODO: Handle access policy in request, currently granting all perms to this user
	policy, err := s.s3Client.GetBucketPolicy(bucketName)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "NoSuchBucketPolicy" {
			return nil, status.Error(codes.Internal, "fetching policy failed")
		}
	}

	statement := s3client.NewPolicyStatement().
		WithSID(userName).
		ForPrincipals(userName).
		ForResources(bucketName).
		ForSubResources(bucketName).
		Allows().
		Actions(s3client.AllowedActions...)
	if policy == nil {
		policy = s3client.NewBucketPolicy(*statement)
	} else {
		policy = policy.ModifyBucketPolicy(*statement)
	}
	_, err = s.s3Client.PutBucketPolicy(bucketName, *policy)
	if err != nil {
		klog.ErrorS(err, "failed to set policy")

		return nil, status.Error(codes.Internal, "failed to set policy")
	}

	// TODO: limit the bucket count for this user to 0

	// Below response if not final, may change in future
	return &cosispec.DriverGrantBucketAccessResponse{
		AccountId:   userName,
		Credentials: fetchUserCredentials(user, s.rgwAdminClient.Endpoint, ""),
	}, nil
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket from a principal.
//
// NOTE: this call needs to be idempotent.
func (s *provisionerServer) DriverRevokeBucketAccess(
	ctx context.Context,
	req *cosispec.DriverRevokeBucketAccessRequest,
) (*cosispec.DriverRevokeBucketAccessResponse, error) {

	// TODO: instead of deleting user, revoke its permission and delete only if no more bucket attached to it
	klog.InfoS("deleting user",
		"id", req.GetAccountId())

	if err := s.rgwAdminClient.RemoveUser(context.Background(), rgwadmin.User{
		ID:          req.GetAccountId(),
		DisplayName: req.GetAccountId(),
	}); err != nil {
		klog.ErrorS(err, "failed to revoke bucket access")

		return nil, status.Error(codes.Internal, "failed to revoke bucket access")
	}

	return &cosispec.DriverRevokeBucketAccessResponse{}, nil
}

func fetchUserCredentials(user rgwadmin.User, endpoint string, region string) map[string]*cosispec.CredentialDetails {
	s3Keys := make(map[string]string)
	s3Keys["accessKeyID"] = user.Keys[0].AccessKey
	s3Keys["accessSecretKey"] = user.Keys[0].SecretKey
	s3Keys["endpoint"] = endpoint
	s3Keys["region"] = region

	creds := &cosispec.CredentialDetails{
		Secrets: s3Keys,
	}

	credDetails := make(map[string]*cosispec.CredentialDetails)
	credDetails["s3"] = creds

	return credDetails
}
