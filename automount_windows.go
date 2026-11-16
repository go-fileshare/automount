// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build windows

package automount

import (
	"context"

	"github.com/go-mswin/win32"
	"golang.org/x/sys/windows"
)

const nativeReadOnly = false

// The Windows entry points, replaceable by tests.
var (
	mapSMB        = win32.MapSMB
	cancelConn    = win32.CancelConnection
	logicalDrives = windows.GetLogicalDrives
)

func nativeMount(ctx context.Context, r request) (string, func(context.Context) error, error) {
	used, err := logicalDrives()
	if err != nil {
		return "", nil, err
	}
	drive, err := pickDrive(used, r.opts.Drive)
	if err != nil {
		return "", nil, err
	}
	err = mapSMB(win32.SMBMapping{
		Remote:   `\\` + r.host + `\` + r.share,
		Local:    drive,
		User:     winUser(r.user),
		Password: r.password,
		TCPPort:  r.port,
	})
	if err != nil {
		return "", nil, err
	}
	return drive + `\`, func(context.Context) error { return cancelConn(drive, false) }, nil
}
