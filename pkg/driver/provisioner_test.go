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
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/s3api/s3_constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

/* -------------------------------- фейковый Filer ------------------------------ */

type fakeFiler struct {
	filer_pb.UnimplementedSeaweedFilerServer
	iam     bytes.Buffer
	buckets map[string]*filer_pb.Entry
}

func (f *fakeFiler) CreateEntry(ctx context.Context, in *filer_pb.CreateEntryRequest) (*filer_pb.CreateEntryResponse, error) {
	if in.Directory == filer.IamConfigDirectory {
		f.iam.Reset()
		f.iam.Write(in.Entry.Content)
	}
	if in.Directory == "/buckets" {
		if f.buckets == nil {
			f.buckets = make(map[string]*filer_pb.Entry)
		}
		f.buckets[in.Entry.Name] = in.Entry
	}
	return &filer_pb.CreateEntryResponse{}, nil
}
func (f *fakeFiler) UpdateEntry(ctx context.Context, in *filer_pb.UpdateEntryRequest) (*filer_pb.UpdateEntryResponse, error) {
	f.iam.Reset()
	f.iam.Write(in.Entry.Content)
	return &filer_pb.UpdateEntryResponse{}, nil
}
func (f *fakeFiler) LookupDirectoryEntry(ctx context.Context, in *filer_pb.LookupDirectoryEntryRequest) (*filer_pb.LookupDirectoryEntryResponse, error) {
	if f.iam.Len() == 0 {
		return nil, fmt.Errorf("no entry is found in filer store")
	}
	return &filer_pb.LookupDirectoryEntryResponse{Entry: &filer_pb.Entry{Content: f.iam.Bytes()}}, nil
}
func (*fakeFiler) DeleteEntry(context.Context, *filer_pb.DeleteEntryRequest) (*filer_pb.DeleteEntryResponse, error) {
	return &filer_pb.DeleteEntryResponse{}, nil
}

/* ------------------------- helper: real TCP gRPC server ----------------------- */

func newProv(t *testing.T) (*provisionerServer, *fakeFiler) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ff := &fakeFiler{}
	srv := grpc.NewServer()
	filer_pb.RegisterSeaweedFilerServer(srv, ff)
	go srv.Serve(lis)

	p, err := NewProvisionerServer("prov", lis.Addr().String(), "", "", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("init prov: %v", err)
	}
	return p.(*provisionerServer), ff
}

/* ----------------------------------- tests ----------------------------------- */

func TestDriverGrantBucketAccess(t *testing.T) {
	p, _ := newProv(t)

	cases := []struct {
		name    string
		req     *cosispec.DriverGrantBucketAccessRequest
		wantErr bool
	}{
		{"empty bucket", &cosispec.DriverGrantBucketAccessRequest{Name: "u"}, true},
		{"empty user", &cosispec.DriverGrantBucketAccessRequest{BucketId: "b"}, true},
		{"ok", &cosispec.DriverGrantBucketAccessRequest{BucketId: "b", Name: "u"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := p.DriverGrantBucketAccess(context.Background(), c.req)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if resp.AccountId != "u" {
				t.Errorf("AccountId=%s want u", resp.AccountId)
			}
			if resp.Credentials["s3"].Secrets["accessKeyID"] == "" ||
				resp.Credentials["s3"].Secrets["accessSecretKey"] == "" {
				t.Errorf("credentials missing")
			}
		})
	}
}

func TestDriverRevokeBucketAccess(t *testing.T) {
	p, _ := newProv(t)
	_, _ = p.DriverGrantBucketAccess(context.Background(),
		&cosispec.DriverGrantBucketAccessRequest{BucketId: "b", Name: "u"})

	cases := []struct {
		name    string
		req     *cosispec.DriverRevokeBucketAccessRequest
		wantErr bool
	}{
		{"empty user", &cosispec.DriverRevokeBucketAccessRequest{}, true},
		{"ok", &cosispec.DriverRevokeBucketAccessRequest{AccountId: "u"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := p.DriverRevokeBucketAccess(context.Background(), c.req)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if !c.wantErr && !reflect.DeepEqual(got, &cosispec.DriverRevokeBucketAccessResponse{}) {
				t.Errorf("unexpected resp=%+v", got)
			}
		})
	}
}

// bucketExtended returns the Extended map from a stored bucket entry.
func bucketExtended(t *testing.T, ff *fakeFiler, name string) map[string][]byte {
	t.Helper()
	if ff.buckets == nil {
		return nil
	}
	e, ok := ff.buckets[name]
	if !ok {
		t.Fatalf("bucket %q not found in fakeFiler", name)
	}
	return e.Extended
}

func TestDriverCreateBucketObjectLock(t *testing.T) {
	cases := []struct {
		name       string
		params     map[string]string
		wantErr    bool
		wantExtKey []string // expected Extended keys
	}{
		{
			name:       "plain bucket (no params)",
			params:     nil,
			wantExtKey: nil,
		},
		{
			name:   "object lock enabled",
			params: map[string]string{"objectLockEnabled": "true"},
			wantExtKey: []string{
				s3_constants.ExtVersioningKey,
				s3_constants.ExtObjectLockEnabledKey,
			},
		},
		{
			name: "object lock with COMPLIANCE retention 30 days",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionMode": "COMPLIANCE",
				"objectLockRetentionDays": "30",
			},
			wantExtKey: []string{
				s3_constants.ExtVersioningKey,
				s3_constants.ExtObjectLockEnabledKey,
				s3_constants.ExtObjectLockDefaultModeKey,
				s3_constants.ExtObjectLockDefaultDaysKey,
			},
		},
		{
			name: "object lock with GOVERNANCE retention 2 years",
			params: map[string]string{
				"objectLockEnabled":        "true",
				"objectLockRetentionMode":  "GOVERNANCE",
				"objectLockRetentionYears": "2",
			},
			wantExtKey: []string{
				s3_constants.ExtVersioningKey,
				s3_constants.ExtObjectLockEnabledKey,
				s3_constants.ExtObjectLockDefaultModeKey,
				s3_constants.ExtObjectLockDefaultYearsKey,
			},
		},
		{
			name: "invalid retention mode",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionMode": "INVALID",
				"objectLockRetentionDays": "10",
			},
			wantErr: true,
		},
		{
			name: "days and years both set",
			params: map[string]string{
				"objectLockEnabled":        "true",
				"objectLockRetentionMode":  "COMPLIANCE",
				"objectLockRetentionDays":  "30",
				"objectLockRetentionYears": "1",
			},
			wantErr: true,
		},
		{
			name: "mode without period",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionMode": "GOVERNANCE",
			},
			wantErr: true,
		},
		{
			name: "days without mode",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionDays": "30",
			},
			wantErr: true,
		},
		{
			name: "days is zero",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionMode": "COMPLIANCE",
				"objectLockRetentionDays": "0",
			},
			wantErr: true,
		},
		{
			name: "days is not a number",
			params: map[string]string{
				"objectLockEnabled":       "true",
				"objectLockRetentionMode": "COMPLIANCE",
				"objectLockRetentionDays": "abc",
			},
			wantErr: true,
		},
		{
			name:       "objectLockEnabled=false treated as plain bucket",
			params:     map[string]string{"objectLockEnabled": "false"},
			wantExtKey: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ff := newProv(t)
			req := &cosispec.DriverCreateBucketRequest{
				Name:       "test-bucket",
				Parameters: c.params,
			}
			_, err := p.DriverCreateBucket(context.Background(), req)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}

			ext := bucketExtended(t, ff, "test-bucket")
			if len(c.wantExtKey) == 0 {
				if len(ext) != 0 {
					t.Errorf("expected no Extended, got %v", ext)
				}
				return
			}

			for _, k := range c.wantExtKey {
				if _, ok := ext[k]; !ok {
					t.Errorf("missing Extended key %q", k)
				}
			}
			if len(ext) != len(c.wantExtKey) {
				t.Errorf("Extended has %d keys, want %d", len(ext), len(c.wantExtKey))
			}

			// verify specific values for object lock keys
			if v, ok := ext[s3_constants.ExtVersioningKey]; ok {
				if string(v) != s3_constants.VersioningEnabled {
					t.Errorf("versioning=%q want %q", v, s3_constants.VersioningEnabled)
				}
			}
			if v, ok := ext[s3_constants.ExtObjectLockEnabledKey]; ok {
				if string(v) != s3_constants.ObjectLockEnabled {
					t.Errorf("objectLockEnabled=%q want %q", v, s3_constants.ObjectLockEnabled)
				}
			}
			if mode := c.params["objectLockRetentionMode"]; mode != "" {
				if string(ext[s3_constants.ExtObjectLockDefaultModeKey]) != mode {
					t.Errorf("mode=%q want %q", ext[s3_constants.ExtObjectLockDefaultModeKey], mode)
				}
			}
			if days := c.params["objectLockRetentionDays"]; days != "" {
				if string(ext[s3_constants.ExtObjectLockDefaultDaysKey]) != days {
					t.Errorf("days=%q want %q", ext[s3_constants.ExtObjectLockDefaultDaysKey], days)
				}
			}
			if years := c.params["objectLockRetentionYears"]; years != "" {
				if string(ext[s3_constants.ExtObjectLockDefaultYearsKey]) != years {
					t.Errorf("years=%q want %q", ext[s3_constants.ExtObjectLockDefaultYearsKey], years)
				}
			}
		})
	}
}
