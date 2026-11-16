// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build !darwin && !linux && !windows

package automount

import "context"

const nativeReadOnly = false

func nativeMount(context.Context, request) (string, func(context.Context) error, error) {
	return "", nil, ErrUnsupported
}
