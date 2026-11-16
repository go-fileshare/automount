// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build windows

package automount

import (
	"context"
	"errors"
	"testing"

	"github.com/go-mswin/win32"
)

func TestWindowsBackend(t *testing.T) {
	oldM, oldC, oldD := mapSMB, cancelConn, logicalDrives
	defer func() { mapSMB, cancelConn, logicalDrives = oldM, oldC, oldD }()

	var got win32.SMBMapping
	var cancelled string
	var force bool
	used := uint32(1<<2 | 1<<25) // C: and Z:
	logicalDrives = func() (uint32, error) { return used, nil }
	mapSMB = func(m win32.SMBMapping) error { got = m; return nil }
	cancelConn = func(name string, f bool) error { cancelled, force = name, f; return nil }

	share := Share{URL: mustURL(t, "smb://127.0.0.1:4455/theca"), User: "alice", Password: "pw"}
	m, err := Mount(context.Background(), share, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := win32.SMBMapping{Remote: `\\127.0.0.1\theca`, Local: "Y:", User: `.\alice`, Password: "pw", TCPPort: 4455}
	if got != want || m.Path() != `Y:\` {
		t.Errorf("mapping %+v, path %q", got, m.Path())
	}
	if err := m.Unmount(context.Background()); err != nil || cancelled != "Y:" || force {
		t.Errorf("unmount: %v, %q, %v", err, cancelled, force)
	}

	if m, err = Mount(context.Background(), share, Options{Drive: "k:"}); err != nil || m.Path() != `K:\` {
		t.Errorf("explicit drive: %v", err)
	}
	if _, err := Mount(context.Background(), share, Options{Drive: "Z:"}); !errors.Is(err, ErrDriveInUse) {
		t.Errorf("err = %v", err)
	}
	if _, err := Mount(context.Background(), share, Options{ReadOnly: true}); !errors.Is(err, ErrReadOnlyUnsupported) {
		t.Errorf("read-only: %v", err)
	}

	port := &win32.SMBPortError{HRESULT: 0x80041001, Code: 1326}
	mapSMB = func(win32.SMBMapping) error { return port }
	_, err = Mount(context.Background(), share, Options{})
	var pe *win32.SMBPortError
	if !errors.As(err, &pe) || pe != port {
		t.Errorf("err = %v", err)
	}

	boom := errors.New("boom")
	logicalDrives = func() (uint32, error) { return 0, boom }
	if _, err := Mount(context.Background(), share, Options{}); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
