# automount

Mount an SMB share in the operating system's file manager **without any
password dialog**, and unmount it. A small pure-Go facade (no cgo) over three
existing backends:

| OS | Backend | What appears |
|---|---|---|
| macOS | [go-macos/netfs](https://github.com/go-macos/netfs) (NetFS, `NoUI`) | a volume under `/Volumes`, in the Finder sidebar |
| Linux (GNOME) | [go-freedesktop/gvfs](https://github.com/go-freedesktop/gvfs) (answers GVfs's own `AskPassword`) | a gvfs location, in Files |
| Windows | [go-mswin/win32](https://github.com/go-mswin/win32) (`MSFT_SmbMapping.Create`, any TCP port, standard user) | a drive letter, in "This PC" |

Any other GOOS compiles and answers `automount.ErrUnsupported`
(`errors.ErrUnsupported`).

```go
u, _ := url.Parse("smb://127.0.0.1:4455/theca")
m, err := automount.Mount(ctx, automount.Share{URL: u, User: "alice", Password: pw}, automount.Options{})
if err != nil { /* errors.Is(err, netfs.ErrAuth) etc. */ }
fmt.Println(m.Path())   // /Volumes/theca, Z:\, /run/user/1000/gvfs/...
defer m.Unmount(ctx)
```

## API

```go
type Share struct {
    URL            *url.URL // smb://host[:port]/share, no user information
    User, Password string
}
type Options struct {
    Drive    string // Windows: "X:"; "" = the highest free letter from Z: down
    ReadOnly bool   // macOS only; elsewhere ErrReadOnlyUnsupported
}
func Mount(ctx context.Context, s Share, o Options) (*Mounted, error)
func (m *Mounted) Path() string
func (m *Mounted) Unmount(ctx context.Context) error // idempotent; a failure may be retried
```

(The type is `Mounted` because Go has one namespace for `Mount` the function
and the type.) `Mount` refuses, before touching the OS: a nil or non-`smb`
URL, a missing host or share, a share with a sub-path, a bad port
(`ErrBadURL`); user information in the URL (`ErrCredentialsInURL`); a bad or
used Windows drive (`ErrBadDrive`, `ErrDriveInUse`, `ErrNoFreeDrive`);
`ReadOnly` where it cannot be honoured (`ErrReadOnlyUnsupported`). Loopback
hosts (`127.0.0.0/8`, `::1`, `localhost`) set NetFS's `AllowLoopback`. On
Windows a bare user name is prefixed with `.\` (a local account); a name with
`\` or `@` is left alone.

## The no-dialog guarantee and secrets

* macOS: NetFS is told `NoUI`; credentials are CFStrings, not part of the URL.
* Linux: the daemon's `AskPassword` call is answered in-process; a second
  request (wrong credentials) aborts with `gvfs.ErrAuthFailed`.
* Windows: the CIM method takes the credentials as arguments; no `net use`
  process is started.

The password never goes into an argument vector, the environment, a URL, an
error message or a log. Backend errors are wrapped with `%w`, so `errors.Is`
reaches `netfs.ErrAuth`, `gvfs.ErrAuthFailed` and `errors.As` reaches
`*win32.SMBPortError`.

## Measured, and where

* **macOS 27.2.1 (arm64): measured**, see
  [automount-live-darwin-2026-11-16.txt](automount-live-darwin-2026-11-16.txt).
  The SMB server of [go-filesystems/smb](https://github.com/go-filesystems/smb)
  runs in the test process on `127.0.0.1:<random port>`; the facade mounts it,
  a file is read and written through `Path()`, `Unmount` removes the volume.
  A wrong password fails with `netfs.ErrAuth`. Run it with
  `AUTOMOUNT_REAL=1 gotest -count=1 -run TestLiveMount .` (the same test runs
  on all three platforms).
* **Linux (Ubuntu 24.04, gvfs 1.54.4): measured** in a headless session
  (`dbus-run-session` with a private `XDG_RUNTIME_DIR`, gvfsd, gvfsd-smb and
  gvfsd-fuse D-Bus-activated), see
  [automount-live-linux-2026-11-16.txt](automount-live-linux-2026-11-16.txt):
  the same sequence as on macOS, wrong password reaching `gvfs.ErrAuthFailed`.
  Not measured: a real GNOME desktop session. The run found a defect in
  go-freedesktop/gvfs v0.1.0 (`/` returned as the path when gvfs-fuse is not
  serving the session), fixed in v0.1.1. When gvfs-fuse is not serving the
  session, `Path()` is the `smb://` URI, which GIO can open and `os` calls
  cannot.
* **Windows 11 25H2 (arm64, QEMU): measured as a standard user and as an
  administrator.** As the standard user `std` (Users only, Medium Mandatory
  Level, not elevated), see
  [automount-live-windows-std-2026-11-17.txt](automount-live-windows-std-2026-11-17.txt),
  and as an administrator,
  [automount-live-windows-2026-11-16.txt](automount-live-windows-2026-11-16.txt):
  wrong password reaching `*win32.SMBPortError`, map `Z:`, read and write
  through it, unmount, `Z:` gone. Not measured: the letter's appearance in
  Explorer's "This PC": an ssh logon has its own logon session, so the check
  is through `os` calls in the same session.

The package is held to 100 % statement coverage on macOS, Linux and Windows in
CI; every OS call sits behind a package-level variable.

## License

BSD 3-Clause.
