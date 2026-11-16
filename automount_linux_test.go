// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build linux

package automount

import (
	"context"
	"errors"
	"testing"

	"github.com/go-freedesktop/gvfs"
)

func TestLinuxBackend(t *testing.T) {
	oldM, oldU := gvfsMount, gvfsUnmount
	defer func() { gvfsMount, gvfsUnmount = oldM, oldU }()

	var gotURI, gotUser, gotPW, unmounted string
	gvfsMount = func(_ context.Context, uri, user, pw string) (string, error) {
		gotURI, gotUser, gotPW = uri, user, pw
		return "/run/user/1000/gvfs/smb-share:server=h,share=s", nil
	}
	gvfsUnmount = func(_ context.Context, uri string) error { unmounted = uri; return nil }

	share := Share{URL: mustURL(t, "smb://h:4455/s"), User: "alice", Password: "pw"}
	m, err := Mount(context.Background(), share, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Path() != "/run/user/1000/gvfs/smb-share:server=h,share=s" || gotURI != "smb://h:4455/s" || gotUser != "alice" || gotPW != "pw" {
		t.Errorf("path %q uri %q user %q", m.Path(), gotURI, gotUser)
	}
	if err := m.Unmount(context.Background()); err != nil || unmounted != "smb://h:4455/s" {
		t.Errorf("unmount: %v, %q", err, unmounted)
	}

	gvfsMount = func(context.Context, string, string, string) (string, error) { return "", gvfs.ErrAuthFailed }
	_, err = Mount(context.Background(), share, Options{})
	if !errors.Is(err, gvfs.ErrAuthFailed) {
		t.Errorf("err = %v", err)
	}
	if _, err := Mount(context.Background(), share, Options{ReadOnly: true}); !errors.Is(err, ErrReadOnlyUnsupported) {
		t.Errorf("read-only: %v", err)
	}
}
