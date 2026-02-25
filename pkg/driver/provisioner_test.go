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
	"net"
	"reflect"
	"sort"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
	"github.com/seaweedfs/seaweedfs/weed/s3api/s3_constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	cosispec "sigs.k8s.io/container-object-storage-interface-spec"
)

/* -------------------------------- fake Filer --------------------------------- */

type fakeFiler struct {
	filer_pb.UnimplementedSeaweedFilerServer
	files   map[string][]byte
	buckets map[string]*filer_pb.Entry
}

func (f *fakeFiler) fileKey(dir, name string) string { return dir + "/" + name }

func (f *fakeFiler) CreateEntry(_ context.Context, in *filer_pb.CreateEntryRequest) (*filer_pb.CreateEntryResponse, error) {
	if in.Entry.Content != nil {
		f.files[f.fileKey(in.Directory, in.Entry.Name)] = in.Entry.Content
	}
	if in.Directory == "/buckets" {
		if f.buckets == nil {
			f.buckets = make(map[string]*filer_pb.Entry)
		}
		f.buckets[in.Entry.Name] = in.Entry
	}
	return &filer_pb.CreateEntryResponse{}, nil
}

func (f *fakeFiler) UpdateEntry(_ context.Context, in *filer_pb.UpdateEntryRequest) (*filer_pb.UpdateEntryResponse, error) {
	key := f.fileKey(in.Directory, in.Entry.Name)
	if _, ok := f.files[key]; !ok {
		return nil, fmt.Errorf("no entry is found in filer store")
	}
	f.files[key] = in.Entry.Content
	return &filer_pb.UpdateEntryResponse{}, nil
}

func (f *fakeFiler) LookupDirectoryEntry(_ context.Context, in *filer_pb.LookupDirectoryEntryRequest) (*filer_pb.LookupDirectoryEntryResponse, error) {
	key := f.fileKey(in.Directory, in.Name)
	data, ok := f.files[key]
	if !ok {
		return nil, fmt.Errorf("no entry is found in filer store")
	}
	return &filer_pb.LookupDirectoryEntryResponse{Entry: &filer_pb.Entry{
		Content:    data,
		Attributes: &filer_pb.FuseAttributes{},
	}}, nil
}

func (f *fakeFiler) DeleteEntry(_ context.Context, in *filer_pb.DeleteEntryRequest) (*filer_pb.DeleteEntryResponse, error) {
	delete(f.files, f.fileKey(in.Directory, in.Name))
	return &filer_pb.DeleteEntryResponse{}, nil
}

/* ------------------------- helper: real TCP gRPC server ----------------------- */

func newProv(t *testing.T) (*provisionerServer, *fakeFiler) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ff := &fakeFiler{files: make(map[string][]byte)}
	srv := grpc.NewServer()
	filer_pb.RegisterSeaweedFilerServer(srv, ff)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	p, err := NewProvisionerServer("prov", lis.Addr().String(), "", "", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("init prov: %v", err)
	}
	return p.(*provisionerServer), ff
}

/* ----------------------------------- tests ----------------------------------- */

func TestDriverCreateBucket(t *testing.T) {
	confKey := filer.DirectoryEtcSeaweedFS + "/" + filer.FilerConfName

	cases := []struct {
		name     string
		bucket   string
		params   map[string]string
		wantDisk string
		wantRepl string
		wantConf bool
		wantErr  bool
	}{
		{
			name:   "no params",
			bucket: "plain",
		},
		{
			name:     "disk only",
			bucket:   "ssd-bucket",
			params:   map[string]string{"disk": "ssd"},
			wantDisk: "ssd",
			wantConf: true,
		},
		{
			name:     "replication only",
			bucket:   "repl-bucket",
			params:   map[string]string{"replication": "001"},
			wantRepl: "001",
			wantConf: true,
		},
		{
			name:     "both params",
			bucket:   "both-bucket",
			params:   map[string]string{"disk": "hdd", "replication": "010"},
			wantDisk: "hdd",
			wantRepl: "010",
			wantConf: true,
		},
		{
			name:    "invalid disk",
			bucket:  "bad-disk",
			params:  map[string]string{"disk": "floppy"},
			wantErr: true,
		},
		{
			name:    "invalid replication",
			bucket:  "bad-repl",
			params:  map[string]string{"replication": "xyz"},
			wantErr: true,
		},
		{
			name:    "replication undocumented value",
			bucket:  "bad-repl2",
			params:  map[string]string{"replication": "999"},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ff := newProv(t)

			resp, err := p.DriverCreateBucket(context.Background(), &cosispec.DriverCreateBucketRequest{
				Name:       c.bucket,
				Parameters: c.params,
			})
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.BucketId != c.bucket {
				t.Errorf("BucketId=%s want %s", resp.BucketId, c.bucket)
			}

			data, exists := ff.files[confKey]
			if !c.wantConf {
				if exists {
					t.Fatal("FilerConf should not exist when no params are set")
				}
				return
			}
			if !exists {
				t.Fatal("FilerConf should exist")
			}

			fc := filer.NewFilerConf()
			if err := fc.LoadFromBytes(data); err != nil {
				t.Fatalf("parse filer conf: %v", err)
			}
			prefix := "/buckets/" + c.bucket + "/"
			var found *filer_pb.FilerConf_PathConf
			for _, loc := range fc.ToProto().Locations {
				if loc.LocationPrefix == prefix {
					found = loc
					break
				}
			}
			if found == nil {
				t.Fatalf("PathConf not found for %s", prefix)
			}
			if found.DiskType != c.wantDisk {
				t.Errorf("DiskType=%s want %s", found.DiskType, c.wantDisk)
			}
			if found.Replication != c.wantRepl {
				t.Errorf("Replication=%s want %s", found.Replication, c.wantRepl)
			}
		})
	}
}

func TestDriverCreateBucketSequentialPreservesAll(t *testing.T) {
	p, ff := newProv(t)
	confKey := filer.DirectoryEtcSeaweedFS + "/" + filer.FilerConfName

	// Create two buckets with different FilerConf params.
	_, err := p.DriverCreateBucket(context.Background(), &cosispec.DriverCreateBucketRequest{
		Name:       "bucket-a",
		Parameters: map[string]string{"disk": "ssd"},
	})
	if err != nil {
		t.Fatalf("create bucket-a: %v", err)
	}
	_, err = p.DriverCreateBucket(context.Background(), &cosispec.DriverCreateBucketRequest{
		Name:       "bucket-b",
		Parameters: map[string]string{"replication": "010"},
	})
	if err != nil {
		t.Fatalf("create bucket-b: %v", err)
	}

	// Both PathConf entries must be present.
	data := ff.files[confKey]
	fc := filer.NewFilerConf()
	if err := fc.LoadFromBytes(data); err != nil {
		t.Fatalf("parse filer conf: %v", err)
	}

	locs := fc.ToProto().Locations
	find := func(prefix string) *filer_pb.FilerConf_PathConf {
		for _, loc := range locs {
			if loc.LocationPrefix == prefix {
				return loc
			}
		}
		return nil
	}

	a := find("/buckets/bucket-a/")
	if a == nil {
		t.Fatal("PathConf for bucket-a missing after second create")
	}
	if a.DiskType != "ssd" {
		t.Errorf("bucket-a DiskType=%s want ssd", a.DiskType)
	}

	b := find("/buckets/bucket-b/")
	if b == nil {
		t.Fatal("PathConf for bucket-b missing")
	}
	if b.Replication != "010" {
		t.Errorf("bucket-b Replication=%s want 010", b.Replication)
	}
}

func TestDriverDeleteBucket(t *testing.T) {
	confKey := filer.DirectoryEtcSeaweedFS + "/" + filer.FilerConfName

	t.Run("removes PathConf entry", func(t *testing.T) {
		p, ff := newProv(t)

		// Create a bucket with FilerConf params.
		_, err := p.DriverCreateBucket(context.Background(), &cosispec.DriverCreateBucketRequest{
			Name:       "to-delete",
			Parameters: map[string]string{"disk": "ssd"},
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, ok := ff.files[confKey]; !ok {
			t.Fatal("FilerConf should exist after create")
		}

		// Delete the bucket.
		_, err = p.DriverDeleteBucket(context.Background(), &cosispec.DriverDeleteBucketRequest{
			BucketId: "to-delete",
		})
		if err != nil {
			t.Fatalf("delete: %v", err)
		}

		// Verify PathConf entry was removed.
		data, ok := ff.files[confKey]
		if !ok {
			return // FilerConf file removed entirely is also acceptable
		}
		fc := filer.NewFilerConf()
		if err := fc.LoadFromBytes(data); err != nil {
			t.Fatalf("parse filer conf: %v", err)
		}
		for _, loc := range fc.ToProto().Locations {
			if loc.LocationPrefix == "/buckets/to-delete/" {
				t.Error("PathConf for deleted bucket should be removed")
			}
		}
	})

	t.Run("no FilerConf created for plain bucket", func(t *testing.T) {
		p, ff := newProv(t)

		// Create bucket without FilerConf params.
		_, err := p.DriverCreateBucket(context.Background(), &cosispec.DriverCreateBucketRequest{
			Name: "plain",
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Delete the bucket.
		_, err = p.DriverDeleteBucket(context.Background(), &cosispec.DriverDeleteBucketRequest{
			BucketId: "plain",
		})
		if err != nil {
			t.Fatalf("delete: %v", err)
		}

		// FilerConf file should not have been created.
		if _, ok := ff.files[confKey]; ok {
			t.Error("FilerConf should not be created when deleting a bucket without params")
		}
	})
}

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

// iamActions parses the fakeFiler IAM buffer and returns sorted actions for the given identity.
func iamActions(t *testing.T, ff *fakeFiler, identity string) []string {
	t.Helper()
	cfg := &iam_pb.S3ApiConfiguration{}
	if ff.iam.Len() > 0 {
		if err := filer.ParseS3ConfigurationFromBytes(ff.iam.Bytes(), cfg); err != nil {
			t.Fatalf("parse IAM config: %v", err)
		}
	}
	for _, id := range cfg.Identities {
		if id.Name == identity {
			actions := make([]string, len(id.Actions))
			copy(actions, id.Actions)
			sort.Strings(actions)
			return actions
		}
	}
	t.Fatalf("identity %q not found in IAM config", identity)
	return nil
}

func TestDriverGrantBucketAccessPolicy(t *testing.T) {
	cases := []struct {
		name        string
		params      map[string]string
		wantActions []string
		wantErr     bool
	}{
		{
			name:        "readonly access",
			params:      map[string]string{"accessPolicy": "readonly"},
			wantActions: []string{"List:b", "Read:b"},
		},
		{
			name:        "readwrite access",
			params:      map[string]string{"accessPolicy": "readwrite"},
			wantActions: []string{"List:b", "Read:b", "Tagging:b", "Write:b"},
		},
		{
			name:        "default access (no param)",
			params:      nil,
			wantActions: []string{"List:b", "Read:b", "Tagging:b", "Write:b"},
		},
		{
			name:    "invalid access policy",
			params:  map[string]string{"accessPolicy": "invalid"},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ff := newProv(t)
			req := &cosispec.DriverGrantBucketAccessRequest{
				BucketId:   "b",
				Name:       "u",
				Parameters: c.params,
			}
			_, err := p.DriverGrantBucketAccess(context.Background(), req)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			got := iamActions(t, ff, "u")
			if !reflect.DeepEqual(got, c.wantActions) {
				t.Errorf("actions=%v want %v", got, c.wantActions)
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
		{
			name: "retention params without objectLockEnabled",
			params: map[string]string{
				"objectLockRetentionMode": "COMPLIANCE",
				"objectLockRetentionDays": "30",
			},
			wantErr: true,
		},
		{
			name:    "invalid objectLockEnabled value",
			params:  map[string]string{"objectLockEnabled": "True"},
			wantErr: true,
		},
		{
			name: "years is zero",
			params: map[string]string{
				"objectLockEnabled":        "true",
				"objectLockRetentionMode":  "COMPLIANCE",
				"objectLockRetentionYears": "0",
			},
			wantErr: true,
		},
		{
			name: "years is not a number",
			params: map[string]string{
				"objectLockEnabled":        "true",
				"objectLockRetentionMode":  "COMPLIANCE",
				"objectLockRetentionYears": "abc",
			},
			wantErr: true,
		},
		{
			name: "years without mode",
			params: map[string]string{
				"objectLockEnabled":        "true",
				"objectLockRetentionYears": "2",
			},
			wantErr: true,
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
