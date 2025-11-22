# webdavfs

[![Go Reference](https://pkg.go.dev/badge/github.com/absfs/webdavfs.svg)](https://pkg.go.dev/github.com/absfs/webdavfs)
[![Go Report Card](https://goreportcard.com/badge/github.com/absfs/webdavfs)](https://goreportcard.com/report/github.com/absfs/webdavfs)

WebDAV filesystem implementation for the [absfs](https://github.com/absfs/absfs) ecosystem.

## Overview

`webdavfs` is an implementation of the `absfs.FileSystem` interface that provides access to remote WebDAV servers. It allows any Go application using the absfs abstraction to transparently work with WebDAV-hosted files and directories as if they were local filesystem operations.

This package acts as an **adapter** between the absfs filesystem interface and WebDAV protocol operations, enabling seamless integration with any WebDAV-compliant server (Nextcloud, ownCloud, Apache mod_dav, nginx, etc.).

## Architecture

### Core Pattern: Direct Backend Integration

`webdavfs` follows the **Direct Backend Integration** pattern used by other absfs network filesystem implementations like `sftpfs` and `s3fs`. The implementation consists of:

1. **FileSystem struct** - Holds WebDAV client connection and configuration
2. **File wrapper** - Adapts WebDAV file operations to the `absfs.File` interface
3. **Protocol mapping** - Translates absfs operations to WebDAV HTTP methods

```
┌─────────────────────┐
│  absfs Application  │
│  (uses absfs API)   │
└──────────┬──────────┘
           │ absfs.FileSystem interface
           ▼
┌─────────────────────┐
│    webdavfs         │
│  FileSystem struct  │
└──────────┬──────────┘
           │ WebDAV HTTP protocol
           ▼
┌─────────────────────┐
│  WebDAV Server      │
│  (remote storage)   │
└─────────────────────┘
```

### absfs Interface Implementation

The package implements the complete `absfs.FileSystem` interface hierarchy:

#### Core Filer Interface (8 methods)
- `OpenFile(name string, flag int, perm os.FileMode) (File, error)` → WebDAV GET/PUT/MKCOL
- `Mkdir(name string, perm os.FileMode) error` → WebDAV MKCOL
- `Remove(name string) error` → WebDAV DELETE
- `Rename(oldpath, newpath string) error` → WebDAV MOVE
- `Stat(name string) (os.FileInfo, error)` → WebDAV PROPFIND
- `Chmod(name string, mode os.FileMode) error` → WebDAV PROPPATCH (limited support)
- `Chtimes(name string, atime, mtime time.Time) error` → WebDAV PROPPATCH
- `Chown(name string, uid, gid int) error` → WebDAV PROPPATCH (limited support)

#### Extended FileSystem Methods
- `Separator()` → Returns '/'
- `ListSeparator()` → Returns ':'
- `Chdir(dir string) error` → Local state tracking
- `Getwd() (string, error)` → Local state tracking
- `TempDir() string` → Returns server-specific temp path
- `Open(name string) (File, error)` → Convenience wrapper for OpenFile
- `Create(name string) (File, error)` → Convenience wrapper for OpenFile
- `MkdirAll(name string, perm os.FileMode) error` → Recursive MKCOL
- `RemoveAll(path string) error` → Recursive DELETE
- `Truncate(name string, size int64) error` → WebDAV partial PUT

### File Interface Implementation

The `File` wrapper implements `absfs.File` (which extends `io.Reader`, `io.Writer`, `io.Seeker`, `io.Closer`):

#### Core Operations
- `Read(b []byte) (int, error)` → HTTP GET with range
- `Write(b []byte) (int, error)` → HTTP PUT with buffering
- `Close() error` → Flush writes, close connection
- `Seek(offset int64, whence int) (int64, error)` → Update position tracking
- `Stat() (os.FileInfo, error)` → WebDAV PROPFIND

#### Extended Operations
- `ReadAt(b []byte, off int64) (int, error)` → HTTP GET with Range header
- `WriteAt(b []byte, off int64) (int, error)` → HTTP PUT with Content-Range
- `Readdir(n int) ([]os.FileInfo, error)` → WebDAV PROPFIND with depth=1
- `Readdirnames(n int) ([]string, error)` → Wrapper around Readdir
- `Truncate(size int64) error` → WebDAV partial PUT
- `Sync() error` → Flush buffered writes

## WebDAV Protocol Mapping

### HTTP Methods Used

| absfs Operation | WebDAV Method | Description |
|----------------|---------------|-------------|
| Stat | PROPFIND | Get file/directory properties |
| Open (read) | GET | Download file content |
| Create/Write | PUT | Upload file content |
| Mkdir | MKCOL | Create directory |
| Remove | DELETE | Delete file/directory |
| Rename | MOVE | Rename/move resource |
| Chtimes | PROPPATCH | Modify modification time |
| Readdir | PROPFIND (Depth: 1) | List directory contents |

### WebDAV Properties Used

Standard DAV properties accessed via PROPFIND:

- `displayname` → File name
- `getcontentlength` → File size
- `getlastmodified` → Modification time
- `resourcetype` → File vs directory detection
- `getetag` → Cache validation
- `getcontenttype` → MIME type

## Limitations and Considerations

### WebDAV Server Variability

WebDAV is a loosely defined standard with varying server implementations:

1. **Permissions** - Not all servers support Unix permissions via PROPPATCH
   - `Chmod` may be a no-op on some servers
   - `Chown` typically unsupported in most WebDAV servers

2. **Atomic Operations** - Limited atomicity guarantees
   - No native locking in basic WebDAV (requires WebDAV Locking extension)
   - Concurrent writes may result in race conditions

3. **Performance** - Network latency considerations
   - Each operation is an HTTP request
   - Directory listings can be expensive (recursive PROPFIND)
   - Consider using caching wrappers like `corfs` for read-heavy workloads

4. **Partial Updates** - Server-dependent support
   - `WriteAt` and `Truncate` require Content-Range support
   - Some servers may require full file replacement

### Security Considerations

1. **Authentication** - Support for HTTP Basic, Digest, and Bearer tokens
2. **TLS/HTTPS** - Strongly recommended for production use
3. **Credentials** - Stored in memory, consider using credential helpers
4. **Path Traversal** - All paths sanitized before HTTP requests

## Usage Patterns

### Standalone WebDAV Access

```go
import "github.com/absfs/webdavfs"

// Connect to WebDAV server
fs, err := webdavfs.New(&webdavfs.Config{
    URL:      "https://webdav.example.com/remote.php/dav/files/user/",
    Username: "user",
    Password: "password",
})
if err != nil {
    log.Fatal(err)
}
defer fs.Close()

// Use standard absfs operations
file, err := fs.Create("documents/report.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

_, err = file.Write([]byte("Hello WebDAV!"))
```

### Composition with Other absfs Filesystems

#### Cache-on-Read Pattern (using corfs)

```go
import (
    "github.com/absfs/webdavfs"
    "github.com/absfs/corfs"
    "github.com/absfs/memfs"
)

// Remote WebDAV as primary
remote, _ := webdavfs.New(&webdavfs.Config{...})

// Local memory cache
cache := memfs.NewFS()

// Wrap with cache-on-read
fs := corfs.New(remote, cache)

// First read hits remote, subsequent reads use cache
data, _ := fs.ReadFile("large-file.bin")
```

#### Copy-on-Write Layering (using cowfs)

```go
import (
    "github.com/absfs/webdavfs"
    "github.com/absfs/cowfs"
)

// WebDAV base layer (read-only in practice)
base, _ := webdavfs.New(&webdavfs.Config{...})

// Local overlay for writes
overlay := os.DirFS("/tmp/overlay")

// Writes go to overlay, reads fall through to WebDAV
fs := cowfs.New(overlay, base)

// Modify without affecting remote until explicit sync
fs.WriteFile("config.yml", data, 0644)
```

#### Multi-Protocol Access

```go
import (
    "github.com/absfs/webdavfs"
    "github.com/absfs/sftpfs"
    "github.com/absfs/s3fs"
)

// Access same logical filesystem via different protocols
webdav, _ := webdavfs.New(&webdavfs.Config{URL: "https://..."})
sftp, _ := sftpfs.New(&sftpfs.Config{Host: "..."})
s3, _ := s3fs.New(&s3fs.Config{Bucket: "..."})

// Application code uses absfs.FileSystem interface
// Can switch between protocols transparently
var fs absfs.FileSystem = webdav
```

## Implementation Structure

Expected package structure:

```
webdavfs/
├── README.md           # This file
├── go.mod              # Module definition
├── webdavfs.go         # FileSystem implementation
├── file.go             # File wrapper implementation
├── client.go           # WebDAV HTTP client
├── properties.go       # WebDAV property parsing
├── config.go           # Configuration structs
├── errors.go           # Error handling
└── webdavfs_test.go    # Integration tests
```

### Key Components

#### `webdavfs.go` - FileSystem Implementation
```go
type FileSystem struct {
    client *webdavClient
    root   string
    cwd    string
}

func New(config *Config) (*FileSystem, error)
func (fs *FileSystem) OpenFile(...) (absfs.File, error)
func (fs *FileSystem) Mkdir(...) error
// ... other Filer methods
```

#### `file.go` - File Wrapper
```go
type File struct {
    fs       *FileSystem
    path     string
    mode     int
    offset   int64
    info     os.FileInfo
    buffer   *bytes.Buffer  // For write buffering
    modified bool
}

func (f *File) Read(b []byte) (int, error)
func (f *File) Write(b []byte) (int, error)
// ... other File methods
```

#### `client.go` - WebDAV HTTP Client
```go
type webdavClient struct {
    httpClient *http.Client
    baseURL    *url.URL
    auth       authProvider
}

func (c *webdavClient) propfind(...) (*multistatus, error)
func (c *webdavClient) get(...) (io.ReadCloser, error)
func (c *webdavClient) put(...) error
func (c *webdavClient) mkcol(...) error
func (c *webdavClient) delete(...) error
func (c *webdavClient) move(...) error
```

## Testing Strategy

### Unit Tests
- Mock HTTP server using `httptest`
- Test each absfs method with various WebDAV responses
- Error handling for malformed XML, network errors
- Path sanitization and encoding

### Integration Tests
- Real WebDAV server (docker container)
- Full absfs compliance test suite
- Concurrent access patterns
- Large file handling

### Compatibility Tests
Against common WebDAV servers:
- Apache mod_dav
- nginx with ngx_http_dav_module
- Nextcloud
- ownCloud
- SabreDAV

## Related Projects

### absfs Ecosystem
- [absfs](https://github.com/absfs/absfs) - Core filesystem abstraction
- [memfs](https://github.com/absfs/memfs) - In-memory filesystem
- [osfs](https://github.com/absfs/osfs) - OS filesystem wrapper
- [sftpfs](https://github.com/absfs/sftpfs) - SFTP protocol access
- [s3fs](https://github.com/absfs/s3fs) - S3-compatible object storage
- [corfs](https://github.com/absfs/corfs) - Cache-on-read wrapper
- [cowfs](https://github.com/absfs/cowfs) - Copy-on-write layers

### WebDAV Libraries
- [golang.org/x/net/webdav](https://pkg.go.dev/golang.org/x/net/webdav) - WebDAV server implementation
- [github.com/studio-b12/gowebdav](https://github.com/studio-b12/gowebdav) - WebDAV client library

## Design Philosophy

This implementation follows absfs ecosystem principles:

1. **Interface Compliance** - Full implementation of `absfs.FileSystem`
2. **Composition-Friendly** - Works seamlessly with other absfs wrappers
3. **Minimal Dependencies** - Only essential WebDAV protocol libraries
4. **Error Transparency** - WebDAV errors wrapped in absfs error types
5. **Zero Configuration** - Sensible defaults, minimal required config
6. **Test Coverage** - Comprehensive tests against real servers

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please ensure:
- All `absfs.FileSystem` methods implemented
- Tests pass against multiple WebDAV servers
- Documentation updated for new features
- Error handling follows absfs patterns
