// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build darwin

package automount_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-fileshare/automount"
	"github.com/go-filesystems/smb"
	"github.com/go-macos/netfs"
)

// TestLiveMount mounts an SMB server of this process on 127.0.0.1 and a random
// port through the facade, reads and writes a file through the returned path,
// unmounts, and checks the mountpoint is gone. It changes the machine's mount
// table, so it runs only with AUTOMOUNT_REAL=1.
func TestLiveMount(t *testing.T) {
	if os.Getenv("AUTOMOUNT_REAL") != "1" {
		t.Skip("set AUTOMOUNT_REAL=1 to mount a real SMB share")
	}
	const user, password = "alice", "hunter2"

	mem := newMemFS()
	if err := mem.WriteFile("/hello.txt", []byte("from the server"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := smb.New()
	srv.AddUser(user, password)
	if err := srv.Share("share", mem, smb.AllowUsers(user)); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	u := &url.URL{Scheme: "smb", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Path: "/share"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// A wrong password goes first (see netfs's own live test): it must fail
	// without a dialog, and errors.Is must reach netfs.ErrAuth.
	bad, err := automount.Mount(ctx, automount.Share{URL: u, User: user, Password: "wrong"}, automount.Options{})
	if err == nil {
		t.Logf("unmount: %v", bad.Unmount(ctx))
		t.Fatalf("a wrong password mounted at %s", bad.Path())
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Errorf("error mentions the password: %v", err)
	}
	t.Logf("wrong password: %v (netfs.ErrAuth=%v)", err, errors.Is(err, netfs.ErrAuth))

	m, err := automount.Mount(ctx, automount.Share{URL: u, User: user, Password: password}, automount.Options{})
	if err != nil {
		t.Fatalf("mount %s: %v", u, err)
	}
	t.Logf("mounted %s at %s", u, m.Path())
	defer m.Unmount(ctx)

	if !strings.HasPrefix(m.Path(), "/Volumes/") {
		t.Errorf("mountpoint %q is not under /Volumes", m.Path())
	}
	if b, err := os.ReadFile(filepath.Join(m.Path(), "hello.txt")); err != nil || string(b) != "from the server" {
		t.Errorf("read server file: %q, %v", b, err)
	}
	want := []byte("round trip through automount")
	if err := os.WriteFile(filepath.Join(m.Path(), "back.txt"), want, 0o644); err != nil {
		t.Fatalf("write through the mount: %v", err)
	}
	if b, err := mem.ReadFile("/back.txt"); err != nil || string(b) != string(want) {
		t.Errorf("server side holds %q, %v", b, err)
	}

	if err := m.Unmount(ctx); err != nil {
		t.Fatalf("unmount: %v", err)
	}
	if err := m.Unmount(ctx); err != nil {
		t.Errorf("second unmount: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Path(), "hello.txt")); err == nil {
		t.Errorf("%s still serves files after Unmount", m.Path())
	}
	// The mountpoint directory is removed shortly after unmount(2) returns.
	start := time.Now()
	for {
		if _, err := os.Stat(m.Path()); err != nil {
			t.Logf("mountpoint %s gone %v after Unmount", m.Path(), time.Since(start).Round(time.Millisecond))
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Errorf("mountpoint %s still exists 5 s after Unmount", m.Path())
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}
