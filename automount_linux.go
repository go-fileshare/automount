// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build linux

package automount

import (
	"context"

	"github.com/go-freedesktop/gvfs"
)

const nativeReadOnly = false

// The GVfs entry points, replaceable by tests.
var (
	gvfsMount   = gvfs.Mount
	gvfsUnmount = gvfs.Unmount
)

func nativeMount(ctx context.Context, r request) (string, func(context.Context) error, error) {
	p, err := gvfsMount(ctx, r.uri, r.user, r.password)
	if err != nil {
		return "", nil, err
	}
	return p, func(ctx context.Context) error { return gvfsUnmount(ctx, r.uri) }, nil
}
