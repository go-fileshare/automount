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
  `AUTOMOUNT_REAL=1 gotest -count=1 -run TestLiveMount .`.
* **Linux: not measured** here (the unit tests replace the GVfs calls). The
  GVfs backend has its own tests.
* **Windows: not measured** here (the unit tests replace the win32 calls). The
  `MapSMB` call as a standard user is measured in
  [go-mswin/win32](https://github.com/go-mswin/win32)
  (`win32-mapsmb-standard-user-proof-2026-11-16.txt`); a mapping made from an
  interactive desktop session was not.

The package is held to 100 % statement coverage on macOS, Linux and Windows in
CI; every OS call sits behind a package-level variable.

## License

BSD 3-Clause.
