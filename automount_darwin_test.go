// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build darwin

package automount

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/go-macos/netfs"
)

func TestDarwinBackend(t *testing.T) {
	oldM, oldU := netfsMount, netfsUnmount
	defer func() { netfsMount, netfsUnmount = oldM, oldU }()

	var gotURL *url.URL
	var gotUser, gotPW string
	var gotOpts netfs.Options
	var unmounted string
	netfsMount = func(_ context.Context, u *url.URL, user, pw string, o netfs.Options) (string, error) {
		gotURL, gotUser, gotPW, gotOpts = u, user, pw, o
		return "/Volumes/share", nil
	}
	netfsUnmount = func(mp string) error { unmounted = mp; return nil }

	share := Share{URL: mustURL(t, "smb://127.0.0.1:4455/share"), User: "alice", Password: "pw"}
	m, err := Mount(context.Background(), share, Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if m.Path() != "/Volumes/share" || gotUser != "alice" || gotPW != "pw" ||
		gotURL.String() != "smb://127.0.0.1:4455/share" || gotURL.User != nil ||
		!gotOpts.AllowLoopback || !gotOpts.ReadOnly {
		t.Errorf("path %q, url %v, user %q, opts %+v", m.Path(), gotURL, gotUser, gotOpts)
	}
	if err := m.Unmount(context.Background()); err != nil || unmounted != "/Volumes/share" {
		t.Errorf("unmount: %v, %q", err, unmounted)
	}

	// A remote host is not marked loopback; backend errors keep their identity.
	netfsMount = func(_ context.Context, _ *url.URL, _, _ string, o netfs.Options) (string, error) {
		if o.AllowLoopback {
			t.Error("AllowLoopback set for a remote host")
		}
		return "", netfs.ErrAuth
	}
	_, err = Mount(context.Background(), Share{URL: mustURL(t, "smb://nas/docs")}, Options{})
	if !errors.Is(err, netfs.ErrAuth) {
		t.Errorf("err = %v", err)
	}
}
