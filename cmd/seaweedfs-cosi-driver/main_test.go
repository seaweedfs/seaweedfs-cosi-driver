/*
Copyright 2026 SeaweedFS contributors.

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
	"net"
	"os"
	"path/filepath"
	"testing"
)

// A stale socket (file present, no listener) must be removed so the next bind
// succeeds — this is the CrashLoopBackOff self-heal.
func TestRemoveStaleSocket_RemovesDeadSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "cosi.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// Keep the socket file on close so it looks like a non-graceful exit.
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("setup: stale socket should exist: %v", err)
	}

	if err := removeStaleSocket("unix://" + sock); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("stale socket should be removed, stat err = %v", err)
	}
}

// A socket with a live listener must NOT be removed — deleting it would orphan
// an actively-served socket.
func TestRemoveStaleSocket_KeepsLiveSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "cosi.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = l.Close() }()

	if err := removeStaleSocket("unix://" + sock); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("live socket must not be removed: %v", err)
	}
}

// No socket present is a no-op (clean first start).
func TestRemoveStaleSocket_NoSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "absent.sock")
	if err := removeStaleSocket("unix://" + sock); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
}

// A misconfigured endpoint pointing at a regular file must NOT delete that file.
func TestRemoveStaleSocket_LeavesRegularFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(f, []byte("keepme"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := removeStaleSocket("unix://" + f); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("regular file must not be removed: %v", err)
	}
}

// Non-unix endpoints are ignored.
func TestRemoveStaleSocket_NonUnixEndpoint(t *testing.T) {
	if err := removeStaleSocket("tcp://127.0.0.1:9000"); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
}

// A unix endpoint with no path is ignored without touching the filesystem.
func TestRemoveStaleSocket_EmptyPath(t *testing.T) {
	if err := removeStaleSocket("unix://"); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
}

// The path removed is parsed the same way the provisioner sidecar binds it
// (net/url), so the cleanup targets the exact path that will be bound.
func TestRemoveStaleSocket_ParsesLikeURL(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sock := filepath.Join(sub, "cosi.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	endpoint := "unix://" + sock // absolute path -> three-slash form
	if err := removeStaleSocket(endpoint); err != nil {
		t.Fatalf("removeStaleSocket: %v", err)
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket at url.Path should be removed, stat err = %v", err)
	}
}
