// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package automount_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-macos/netfs"
)

func liveAuthErr(err error) bool { return errors.Is(err, netfs.ErrAuth) }

func liveCheckPath(t *testing.T, p string) {
	if !strings.HasPrefix(p, "/Volumes/") {
		t.Errorf("mountpoint %q is not under /Volumes", p)
	}
}
