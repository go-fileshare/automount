// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package automount_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/go-mswin/win32"
)

func liveAuthErr(err error) bool {
	var pe *win32.SMBPortError
	return errors.As(err, &pe)
}

func liveCheckPath(t *testing.T, p string) {
	if !regexp.MustCompile(`^[A-Z]:\\$`).MatchString(p) {
		t.Errorf("path %q is not a drive root", p)
	}
}
