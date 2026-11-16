// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package automount

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestNewRequestRefusals(t *testing.T) {
	cases := []struct {
		name string
		u    *url.URL
		want error
	}{
		{"nil", nil, ErrBadURL},
		{"scheme", mustURL(t, "http://h/s"), ErrBadURL},
		{"no host", mustURL(t, "smb:///s"), ErrBadURL},
		{"userinfo", mustURL(t, "smb://u:p@h/s"), ErrCredentialsInURL},
		{"no share", mustURL(t, "smb://h"), ErrBadURL},
		{"empty share", mustURL(t, "smb://h/"), ErrBadURL},
		{"subpath", mustURL(t, "smb://h/s/d"), ErrBadURL},
		{"query", mustURL(t, "smb://h/s?x=1"), ErrBadURL},
		{"fragment", mustURL(t, "smb://h/s#f"), ErrBadURL},
		{"opaque", &url.URL{Scheme: "smb", Opaque: "h/s"}, ErrBadURL},
		{"port 0", mustURL(t, "smb://h:0/s"), ErrBadURL},
		{"port big", mustURL(t, "smb://h:99999/s"), ErrBadURL},
	}
	for _, c := range cases {
		if _, err := newRequest(Share{URL: c.u}, Options{}); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestNewRequest(t *testing.T) {
	cases := []struct {
		raw      string
		host     string
		hostport string
		port     uint16
		uri      string
		loopback bool
	}{
		{"smb://nas/docs", "nas", "nas", 0, "smb://nas/docs", false},
		{"smb://127.0.0.1:4455/my%20share", "127.0.0.1", "127.0.0.1:4455", 4455, "smb://127.0.0.1:4455/my%20share", true},
		{"smb://[::1]:445/s", "::1", "[::1]:445", 445, "smb://[::1]:445/s", true},
		{"smb://localhost/s", "localhost", "localhost", 0, "smb://localhost/s", true},
		{"smb://10.0.0.2/s", "10.0.0.2", "10.0.0.2", 0, "smb://10.0.0.2/s", false},
	}
	for _, c := range cases {
		r, err := newRequest(Share{URL: mustURL(t, c.raw), User: "alice", Password: "pw"}, Options{ReadOnly: true})
		if err != nil {
			t.Fatalf("%s: %v", c.raw, err)
		}
		if r.host != c.host || r.hostport != c.hostport || r.port != c.port || r.uri != c.uri || r.loopback != c.loopback {
			t.Errorf("%s: got %+v", c.raw, r)
		}
		if r.user != "alice" || r.password != "pw" || !r.opts.ReadOnly {
			t.Errorf("%s: credentials or options lost: %+v", c.raw, r)
		}
	}
	r, _ := newRequest(Share{URL: mustURL(t, "smb://h/my%20share")}, Options{})
	if r.share != "my share" {
		t.Errorf("share = %q", r.share)
	}
}

func TestMountWith(t *testing.T) {
	const secret = "s3cret-pw"
	share := Share{URL: mustURL(t, "smb://h:4455/s"), User: "alice", Password: secret}
	var got request
	unmounts := 0
	good := func(_ context.Context, r request) (string, func(context.Context) error, error) {
		got = r
		return "/mnt/x", func(context.Context) error { unmounts++; return nil }, nil
	}
	m, err := mountWith(context.Background(), good, share, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Path() != "/mnt/x" || got.password != secret || got.uri != "smb://h:4455/s" {
		t.Errorf("path %q request %+v", m.Path(), got)
	}
	for range 2 {
		if err := m.Unmount(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if unmounts != 1 {
		t.Errorf("backend unmounted %d times, want 1", unmounts)
	}

	// Backend errors keep their identity and never carry the password.
	sentinel := errors.New("sentinel")
	bad := func(context.Context, request) (string, func(context.Context) error, error) {
		return "", nil, sentinel
	}
	_, err = mountWith(context.Background(), bad, share, Options{})
	if !errors.Is(err, sentinel) || strings.Contains(err.Error(), secret) {
		t.Errorf("err = %v", err)
	}

	// A failed unmount may be retried.
	fail := true
	flaky := func(context.Context, request) (string, func(context.Context) error, error) {
		return "/mnt/y", func(context.Context) error {
			if fail {
				return sentinel
			}
			return nil
		}, nil
	}
	m, _ = mountWith(context.Background(), flaky, share, Options{})
	if err := m.Unmount(context.Background()); !errors.Is(err, sentinel) {
		t.Errorf("unmount err = %v", err)
	}
	fail = false
	if err := m.Unmount(context.Background()); err != nil {
		t.Errorf("retry: %v", err)
	}

	// Validation runs before the backend; a canceled context too.
	if _, err := mountWith(context.Background(), good, Share{}, Options{}); !errors.Is(err, ErrBadURL) {
		t.Errorf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mountWith(ctx, good, share, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestReadOnlyGate(t *testing.T) {
	old := readOnlySupported
	defer func() { readOnlySupported = old }()
	share := Share{URL: mustURL(t, "smb://h/s")}
	b := func(context.Context, request) (string, func(context.Context) error, error) {
		return "/m", nil, nil
	}
	readOnlySupported = false
	if _, err := mountWith(context.Background(), b, share, Options{ReadOnly: true}); !errors.Is(err, ErrReadOnlyUnsupported) {
		t.Errorf("err = %v", err)
	}
	readOnlySupported = true
	if _, err := mountWith(context.Background(), b, share, Options{ReadOnly: true}); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestWinUser(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		"alice":         `.\alice`,
		`DOM\alice`:     `DOM\alice`,
		`.\alice`:       `.\alice`,
		"alice@corp.ex": "alice@corp.ex",
	} {
		if got := winUser(in); got != want {
			t.Errorf("winUser(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickDrive(t *testing.T) {
	bit := func(ls ...byte) uint32 {
		var m uint32
		for _, l := range ls {
			m |= 1 << (l - 'A')
		}
		return m
	}
	ok := []struct {
		used uint32
		want string
		got  string
	}{
		{bit('C'), "", "Z:"},
		{bit('C', 'Z', 'Y'), "", "X:"},
		{bit('C', 'D'), "q:", "Q:"},
		{0, "", "Z:"},
	}
	for _, c := range ok {
		got, err := pickDrive(c.used, c.want)
		if err != nil || got != c.got {
			t.Errorf("pickDrive(%b, %q) = %q, %v; want %q", c.used, c.want, got, err, c.got)
		}
	}
	if _, err := pickDrive(bit('Q'), "Q:"); !errors.Is(err, ErrDriveInUse) {
		t.Errorf("err = %v", err)
	}
	for _, bad := range []string{"Q", "Q:\\", "1:", "::", "QQ", "@:", "[:"} {
		if _, err := pickDrive(0, bad); !errors.Is(err, ErrBadDrive) {
			t.Errorf("pickDrive(%q) err = %v", bad, err)
		}
	}
	var all uint32
	for l := byte('D'); l <= 'Z'; l++ {
		all |= bit(l)
	}
	if _, err := pickDrive(all, ""); !errors.Is(err, ErrNoFreeDrive) {
		t.Errorf("err = %v", err)
	}
	if got, err := pickDrive(all&^bit('D'), ""); err != nil || got != "D:" {
		t.Errorf("lowest: %q, %v", got, err)
	}
}
