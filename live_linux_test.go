// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package automount_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-freedesktop/gvfs"
)

func liveAuthErr(err error) bool { return errors.Is(err, gvfs.ErrAuthFailed) }

func liveCheckPath(t *testing.T, p string) {
	if !strings.HasPrefix(p, "/") || !strings.Contains(p, "/gvfs/") {
		t.Errorf("path %q is not a gvfs-fuse path", p)
	}
}
