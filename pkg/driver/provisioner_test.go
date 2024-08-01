/*
Copyright 2023 SUSE, LLC.
Copyright 2024 s3gw contributors.
Copyright 2024 SeaweedFS contributors.

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
	"fmt"
	"reflect"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"google.golang.org/grpc"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

const (
	userCreateJSON = `{
	"user_id": "test-user",
	"display_name": "test-user",
	"email": "",
	"suspended": 0,
	"max_buckets": 1000,
	"subusers": [],
	"keys": [
		{
			"user": "test-user",
			"access_key": "EOE7FYCNOBZJ5VFV909G",
			"secret_key": "qmIqpWm8HxCzmynCrD6U6vKWi4hnDBndOnmxXNsV"
		}
	],
	"swift_keys": [],
	"caps": [
		{
			"type": "users",
			"perm": "*"
		}
	],
	"op_mask": "read, write, delete",
	"default_placement": "",
	"default_storage_class": "",
	"placement_tags": [],
	"bucket_quota": {
		"enabled": false,
		"check_on_raw": false,
		"max_size": -1,
		"max_size_kb": 0,
		"max_objects": -1
	},
	"user_quota": {
		"enabled": false,
		"check_on_raw": false,
		"max_size": -1,
		"max_size_kb": 0,
		"max_objects": -1
	},
	"temp_url_keys": [],
	"type": "rgw",
	"mfa_ids": []
}`
)

func Test_provisionerServer_DriverGrantBucketAccess(t *testing.T) {
	type fields struct {
		provisioner string
		filerClient filer_pb.SeaweedFilerClient
	}
	type args struct {
		ctx context.Context
		req *cosispec.DriverGrantBucketAccessRequest
	}
	// Mocking the filer client
	filerClient := &mockSeaweedFilerClient{
		// Add any necessary mock implementations here
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *cosispec.DriverGrantBucketAccessResponse
		wantErr bool
	}{
		{"Empty Bucket Name", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverGrantBucketAccessRequest{BucketId: "", Name: "test-user"}}, nil, true},
		{"Empty User Name", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverGrantBucketAccessRequest{BucketId: "test-bucket", Name: ""}}, nil, true},
		{"Grant Bucket Access success", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverGrantBucketAccessRequest{BucketId: "test-bucket", Name: "test-user"}}, &cosispec.DriverGrantBucketAccessResponse{
			AccountId: "test-user",
			Credentials: map[string]*cosispec.CredentialDetails{
				"s3": {
					Secrets: map[string]string{
						"accessKeyID":     "some-access-key-id",
						"accessSecretKey": "some-secret-key",
						"endpoint":        "",
						"region":          "",
					},
				},
			},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &provisionerServer{
				provisioner: tt.fields.provisioner,
				filerClient: tt.fields.filerClient,
			}
			got, err := s.DriverGrantBucketAccess(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("provisionerServer.DriverGrantBucketAccess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				// Avoid deep equality check for generated credentials, focus on structure and presence of keys
				if got.AccountId != tt.want.AccountId {
					t.Errorf("provisionerServer.DriverGrantBucketAccess() got AccountId = %v, want AccountId = %v", got.AccountId, tt.want.AccountId)
				}
				if got.Credentials["s3"].Secrets["endpoint"] != tt.want.Credentials["s3"].Secrets["endpoint"] {
					t.Errorf("provisionerServer.DriverGrantBucketAccess() got endpoint = %v, want endpoint = %v", got.Credentials["s3"].Secrets["endpoint"], tt.want.Credentials["s3"].Secrets["endpoint"])
				}
				if got.Credentials["s3"].Secrets["region"] != tt.want.Credentials["s3"].Secrets["region"] {
					t.Errorf("provisionerServer.DriverGrantBucketAccess() got region = %v, want region = %v", got.Credentials["s3"].Secrets["region"], tt.want.Credentials["s3"].Secrets["region"])
				}
				if got.Credentials["s3"].Secrets["accessKeyID"] == "" || got.Credentials["s3"].Secrets["accessSecretKey"] == "" {
					t.Errorf("provisionerServer.DriverGrantBucketAccess() got invalid credentials")
				}
			}
		})
	}
}

func Test_provisionerServer_DriverRevokeBucketAccess(t *testing.T) {
	type fields struct {
		provisioner string
		filerClient filer_pb.SeaweedFilerClient
	}
	type args struct {
		ctx context.Context
		req *cosispec.DriverRevokeBucketAccessRequest
	}
	// Mocking the filer client with appropriate responses
	shouldFail := false
	filerClient := &mockSeaweedFilerClient{
		lookupDirectoryEntryFunc: func(ctx context.Context, in *filer_pb.LookupDirectoryEntryRequest, opts ...grpc.CallOption) (*filer_pb.LookupDirectoryEntryResponse, error) {
			if shouldFail {
				return nil, fmt.Errorf("lookupDirectoryEntryFunc error")
			}
			return &filer_pb.LookupDirectoryEntryResponse{}, nil
		},
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *cosispec.DriverRevokeBucketAccessResponse
		wantErr bool
	}{
		{"Empty user name", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverRevokeBucketAccessRequest{AccountId: ""}}, nil, true},
		{"Revoke Bucket Access success", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverRevokeBucketAccessRequest{AccountId: "test-user"}}, &cosispec.DriverRevokeBucketAccessResponse{}, false},
		{"Revoke Bucket Access failure", fields{"provisioner", filerClient}, args{context.Background(), &cosispec.DriverRevokeBucketAccessRequest{AccountId: "failed-user"}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldFail = tt.wantErr
			s := &provisionerServer{
				provisioner: tt.fields.provisioner,
				filerClient: tt.fields.filerClient,
			}
			got, err := s.DriverRevokeBucketAccess(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("provisionerServer.DriverRevokeBucketAccess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("provisionerServer.DriverRevokeBucketAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}
