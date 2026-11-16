// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build darwin

package automount

import (
	"context"
	"net/url"

	"github.com/go-macos/netfs"
)

const nativeReadOnly = true

// The NetFS entry points, replaceable by tests.
var (
	netfsMount   = netfs.Mount
	netfsUnmount = netfs.Unmount
)

func nativeMount(ctx context.Context, r request) (string, func(context.Context) error, error) {
	u := &url.URL{Scheme: "smb", Host: r.hostport, Path: "/" + r.share}
	mp, err := netfsMount(ctx, u, r.user, r.password, netfs.Options{
		AllowLoopback: r.loopback,
		ReadOnly:      r.opts.ReadOnly,
	})
	if err != nil {
		return "", nil, err
	}
	return mp, func(context.Context) error { return netfsUnmount(mp) }, nil
}
