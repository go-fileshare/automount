// Copyright (c) 2026, the go-fileshare/automount authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

// Package automount mounts an SMB share in the operating system's file
// manager without any password dialog, and unmounts it.
//
// The credentials are handed to the platform's own mount service as values:
// NetFS with NoUI on macOS (github.com/go-macos/netfs), GVfs answering its own
// AskPassword call on Linux/GNOME (github.com/go-freedesktop/gvfs), and
// MSFT_SmbMapping.Create on Windows (github.com/go-mswin/win32). The password
// is never placed in an argument vector, the environment, a URL, an error
// message or a log. Every other GOOS answers ErrUnsupported.
package automount

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// ErrUnsupported is returned by Mount on a GOOS without a backend.
var ErrUnsupported = errors.ErrUnsupported

var (
	// ErrBadURL: the URL is not smb://host[:port]/share.
	ErrBadURL = errors.New("automount: URL must be smb://host[:port]/share")
	// ErrCredentialsInURL: user information in the URL; pass Share.User and
	// Share.Password instead, so that a secret is never part of a URL.
	ErrCredentialsInURL = errors.New("automount: credentials must be passed in Share, not in the URL")
	// ErrBadDrive: Options.Drive is not a drive letter and a colon ("X:").
	ErrBadDrive = errors.New("automount: Drive must be a letter and a colon, like \"X:\"")
	// ErrDriveInUse: Options.Drive is already a logical drive.
	ErrDriveInUse = errors.New("automount: drive letter already in use")
	// ErrNoFreeDrive: every drive letter from D: to Z: is taken.
	ErrNoFreeDrive = errors.New("automount: no free drive letter")
	// ErrReadOnlyUnsupported: Options.ReadOnly on a platform whose backend has
	// no read-only mount.
	ErrReadOnlyUnsupported = errors.New("automount: read-only mounts are not supported on this platform")
)

// Share names an SMB share and the account that opens it.
type Share struct {
	// URL is smb://host[:port]/share. It carries no user information.
	URL *url.URL
	// User and Password are the account. They are passed to the platform's
	// mount service as values.
	User, Password string
}

// Options tunes a mount.
type Options struct {
	// Drive is the Windows drive letter ("X:"). Empty means the highest free
	// letter from Z: down. It is ignored off Windows.
	Drive string
	// ReadOnly mounts read-only. Only macOS supports it; elsewhere Mount fails
	// with ErrReadOnlyUnsupported rather than mounting read-write silently.
	ReadOnly bool
}

// request is a validated Share.
type request struct {
	host     string // no brackets
	hostport string // as the URL wrote it: host[:port], IPv6 bracketed
	port     uint16 // 0 = default
	share    string
	uri      string // smb://host[:port]/share, no user information
	user     string
	password string
	opts     Options
	loopback bool
}

// backend mounts req and returns where it is reachable and how to undo it.
type backend func(ctx context.Context, r request) (path string, unmount func(context.Context) error, err error)

// readOnlySupported says whether the native backend can mount read-only.
var readOnlySupported = nativeReadOnly

// Mounted is one mounted share. (The type cannot be called Mount: Go has one
// namespace for the function and the type.)
type Mounted struct {
	path string

	mu      sync.Mutex
	unmount func(context.Context) error
	done    bool
}

// Mount mounts the share in the user's file manager: a volume under /Volumes
// on macOS, a gvfs location on Linux, a drive letter on Windows. No dialog
// appears; when the credentials are wrong the error says so (errors.Is
// reaches the backend's sentinel: netfs.ErrAuth, gvfs.ErrAuthFailed, or a
// *win32.SMBPortError).
func Mount(ctx context.Context, s Share, o Options) (*Mounted, error) {
	return mountWith(ctx, nativeMount, s, o)
}

func mountWith(ctx context.Context, b backend, s Share, o Options) (*Mounted, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := newRequest(s, o)
	if err != nil {
		return nil, err
	}
	if o.ReadOnly && !readOnlySupported {
		return nil, ErrReadOnlyUnsupported
	}
	p, un, err := b(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("automount: mount %s: %w", r.uri, err)
	}
	return &Mounted{path: p, unmount: un}, nil
}

// Path is where the share is reachable: a directory on macOS and Linux, a
// drive root such as `Z:\` on Windows. On Linux, when gvfs-fuse is not serving
// the session, it is the smb:// URI, which GIO opens and os calls do not.
func (m *Mounted) Path() string { return m.path }

// Unmount unmounts the share. It is safe to call twice; the second call does
// nothing. A failure (a busy volume, say) leaves the mount in place and may
// be retried.
func (m *Mounted) Unmount(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done {
		return nil
	}
	if err := m.unmount(ctx); err != nil {
		return fmt.Errorf("automount: unmount %s: %w", m.path, err)
	}
	m.done = true
	return nil
}

func newRequest(s Share, o Options) (request, error) {
	u := s.URL
	if u == nil || u.Scheme != "smb" || u.Hostname() == "" {
		return request{}, ErrBadURL
	}
	if u.User != nil {
		return request{}, ErrCredentialsInURL
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" || strings.Contains(name, "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return request{}, ErrBadURL
	}
	r := request{
		host:     u.Hostname(),
		hostport: u.Host,
		share:    name,
		user:     s.User,
		password: s.Password,
		opts:     o,
	}
	if ps := u.Port(); ps != "" {
		n, err := strconv.ParseUint(ps, 10, 16)
		if err != nil || n == 0 {
			return request{}, ErrBadURL
		}
		r.port = uint16(n)
	}
	r.uri = "smb://" + r.hostport + "/" + url.PathEscape(name)
	ip := net.ParseIP(r.host)
	r.loopback = r.host == "localhost" || (ip != nil && ip.IsLoopback())
	return r, nil
}

// winUser prefixes a bare account name with `.\` so that Windows looks for a
// local account instead of a domain one.
func winUser(u string) string {
	if u == "" || strings.ContainsAny(u, `\@`) {
		return u
	}
	return `.\` + u
}

// validDrive reports whether d is "X:" with a letter.
func validDrive(d string) bool {
	return len(d) == 2 && d[1] == ':' && (d[0]|0x20 >= 'a' && d[0]|0x20 <= 'z')
}

// pickDrive chooses the drive letter. used is the GetLogicalDrives bitmask
// (bit 0 = A:). want is Options.Drive: validated, and refused if used.
// Without one, the highest free letter from Z: down to D: is returned.
func pickDrive(used uint32, want string) (string, error) {
	if want != "" {
		if !validDrive(want) {
			return "", ErrBadDrive
		}
		l := (want[0] | 0x20) - 'a'
		if used&(1<<l) != 0 {
			return "", ErrDriveInUse
		}
		return strings.ToUpper(want[:1]) + ":", nil
	}
	for l := byte(25); l >= 3; l-- {
		if used&(1<<l) == 0 {
			return string([]byte{'A' + l, ':'}), nil
		}
	}
	return "", ErrNoFreeDrive
}
