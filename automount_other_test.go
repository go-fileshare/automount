// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

//go:build !darwin && !linux && !windows

package automount

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupported(t *testing.T) {
	_, err := Mount(context.Background(), Share{URL: mustURL(t, "smb://h/s")}, Options{})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v", err)
	}
}
