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
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
	"google.golang.org/grpc"
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
	endpoint         string
	region           string
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
func createFilerClient(filerEndpoint string, grpcDialOption grpc.DialOption) (filer_pb.SeaweedFilerClient, error) {
	conn, err := grpc.Dial(filerEndpoint, grpcDialOption)
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
func NewProvisionerServer(provisioner, filerEndpoint, endpoint, region string, grpcDialOption grpc.DialOption) (cosispec.ProvisionerServer, error) {
	// Create filer client here
	filerClient, err := createFilerClient(filerEndpoint, grpcDialOption)
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
		endpoint:         endpoint,
		region:           region,
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

// Revoke access to a bucket.
func (s *provisionerServer) revokeBucketAccess(ctx context.Context, userId string) error {
	err := s.configureS3Access(ctx, userId, "", "", nil, true)
	if err != nil {
		// Check if the error is because the entry was not found
		if strings.HasSuffix(err.Error(), "no entry is found in filer store") {
			klog.InfoS("no entry found in filer store, treating as success", "user", userId)
			return nil
		}
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
		// Handle the case where the file is not found
		if strings.HasSuffix(err.Error(), "no entry is found in filer store") {
			return nil
		}
		return err
	}

	if entry.Entry != nil && entry.Entry.Content != nil {
		buf.Write(entry.Entry.Content)
	}

	return nil
}

// Save the S3 configuration to the SeaweedFS Filer.
func (s *provisionerServer) saveS3Configuration(ctx context.Context, data []byte) error {
	// Check if the S3 configuration file exists
	_, err := s.filerClient.LookupDirectoryEntry(ctx, &filer_pb.LookupDirectoryEntryRequest{
		Directory: filer.IamConfigDirectory,
		Name:      filer.IamIdentityFile,
	})
	if err != nil {
		// Handle the case where the file is not found
		if strings.HasSuffix(err.Error(), "no entry is found in filer store") {
			// Create the S3 configuration file
			_, createErr := s.filerClient.CreateEntry(ctx, &filer_pb.CreateEntryRequest{
				Directory: filer.IamConfigDirectory,
				Entry: &filer_pb.Entry{
					Name:        filer.IamIdentityFile,
					Content:     data,
					IsDirectory: false,
				},
			})
			if createErr != nil {
				return fmt.Errorf("failed to create S3 configuration file: %w", createErr)
			}
			return nil
		} else {
			return fmt.Errorf("failed to check S3 configuration file: %w", err)
		}
	}

	// Update the existing S3 configuration file
	_, err = s.filerClient.UpdateEntry(ctx, &filer_pb.UpdateEntryRequest{
		Directory: filer.IamConfigDirectory,
		Entry: &filer_pb.Entry{
			Name:        filer.IamIdentityFile,
			Content:     data,
			IsDirectory: false,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update S3 configuration: %w", err)
	}

	return nil
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
	if userName == "" || bucketName == "" {
		return nil, fmt.Errorf("user name or bucket name cannot be empty")
	}
	klog.V(5).Infof("req %v", req)
	klog.Info("Granting user accessPolicy to bucket ", "userName ", userName, " bucketName", bucketName)

	// Generate Access Key ID and Secret Access Key
	accessKey, err := GenerateAccessKeyID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate access key ID: %w", err)
	}
	secretKey, err := GenerateSecretAccessKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret access key: %w", err)
	}

	// Read current S3 configuration
	var buf bytes.Buffer
	if err := s.readS3Configuration(ctx, &buf); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to read S3 configuration: %s", err))
	}

	s3cfg := &iam_pb.S3ApiConfiguration{}
	if buf.Len() > 0 {
		if err := filer.ParseS3ConfigurationFromBytes(buf.Bytes(), s3cfg); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to parse S3 configuration: %s", err))
		}
	}

	// Find or create the identity for the user
	var identity *iam_pb.Identity
	for _, id := range s3cfg.Identities {
		if id.Name == userName {
			identity = id
			break
		}
	}
	if identity == nil {
		identity = &iam_pb.Identity{
			Name:        userName,
			Actions:     []string{},
			Credentials: []*iam_pb.Credential{},
		}
		s3cfg.Identities = append(s3cfg.Identities, identity)
	}

	// Check if credentials already exist for the user
	for _, cred := range identity.Credentials {
		if cred.AccessKey == accessKey && cred.SecretKey == secretKey {
			klog.InfoS("Credentials already exist for user", "userName", userName)
			return &cosispec.DriverGrantBucketAccessResponse{
				AccountId: userName,
				Credentials: map[string]*cosispec.CredentialDetails{
					"s3": {
						Secrets: map[string]string{
							"accessKeyID":     accessKey,
							"accessSecretKey": secretKey,
							"endpoint":        "",
							"region":          "",
						},
					},
				},
			}, nil
		}
	}

	// Add the new credentials to the identity
	identity.Credentials = append(identity.Credentials, &iam_pb.Credential{
		AccessKey: accessKey,
		SecretKey: secretKey,
	})

	// Update actions for the identity
	actions := []string{"Read", "Write", "List", "Tagging"}
	for _, action := range actions {
		fullAction := fmt.Sprintf("%s:%s", action, bucketName)
		if !contains(identity.Actions, fullAction) {
			identity.Actions = append(identity.Actions, fullAction)
		}
	}

	// Save updated S3 configuration
	buf.Reset()
	filer.ProtoToText(&buf, s3cfg)
	if err := s.saveS3Configuration(ctx, buf.Bytes()); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to save S3 configuration: %s", err))
	}

	klog.InfoS("Successfully granted bucket access", "bucketName", bucketName, "userName", userName)

	// Prepare the response with generated credentials
	credentials := map[string]string{
		"accessKeyID":     accessKey,
		"accessSecretKey": secretKey,
		"endpoint":        s.endpoint,
		"region":          s.region,
	}

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
	userName := req.GetAccountId()
	if userName == "" {
		return nil, fmt.Errorf("user name cannot be empty")
	}
	klog.InfoS("revoking bucket access", "user", userName)

	// Implement access revoke logic using SeaweedFS filer client
	err := s.revokeBucketAccess(ctx, userName)
	if err != nil {
		klog.ErrorS(err, "failed to revoke access", "user", userName)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to revoke bucket access: %s", err))
	}

	return &cosispec.DriverRevokeBucketAccessResponse{}, nil
}

// GenerateAccessKeyID generates an Access Key ID of 20 characters long, consisting of uppercase letters and numbers.
func GenerateAccessKeyID() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return generateRandomString(20, charset)
}

// GenerateSecretAccessKey generates a Secret Access Key of 40 characters long.
func GenerateSecretAccessKey() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789/+"
	return generateRandomString(40, charset)
}

// generateRandomString generates a random string of the specified length from the given set of characters.
func generateRandomString(length int, charset string) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := 0; i < length; i++ {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}
