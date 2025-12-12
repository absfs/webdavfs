package webdavfs

import (
	"context"
	"net/url"
	"os"
	"strings"

	"github.com/absfs/absfs"
	"golang.org/x/net/webdav"
)

// normalizePath cleans a path for filesystem operations.
// It handles URL-formatted paths (from WebDAV Destination header) and
// strips trailing slashes that some clients add for directories.
func normalizePath(p string) string {
	// If it's a URL, extract just the path
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		if u, err := url.Parse(p); err == nil {
			p = u.Path
		}
	}
	// Strip trailing slash (except for root)
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

// ServerFileSystem adapts absfs.FileSystem to webdav.FileSystem,
// allowing any absfs filesystem to be served via WebDAV.
type ServerFileSystem struct {
	fs absfs.FileSystem
}

// NewServerFileSystem creates a new WebDAV filesystem adapter that wraps
// the given absfs.FileSystem. The returned webdav.FileSystem can be used
// with golang.org/x/net/webdav.Handler to serve files via WebDAV protocol.
func NewServerFileSystem(fs absfs.FileSystem) webdav.FileSystem {
	return &ServerFileSystem{fs: fs}
}

// Mkdir creates a directory.
// The context parameter is accepted for interface compliance but not used.
func (s *ServerFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return s.fs.Mkdir(name, perm)
}

// OpenFile opens a file with the specified flags and permissions.
// The context parameter is accepted for interface compliance but not used.
func (s *ServerFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	f, err := s.fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &ServerFile{file: f}, nil
}

// RemoveAll removes a file or directory tree.
// The context parameter is accepted for interface compliance but not used.
func (s *ServerFileSystem) RemoveAll(ctx context.Context, name string) error {
	return s.fs.RemoveAll(name)
}

// Rename moves/renames a file or directory.
// The context parameter is accepted for interface compliance but not used.
// Paths are normalized to handle URL-formatted destinations and trailing slashes.
func (s *ServerFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	return s.fs.Rename(normalizePath(oldName), normalizePath(newName))
}

// Stat returns file information.
// The context parameter is accepted for interface compliance but not used.
func (s *ServerFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	return s.fs.Stat(name)
}

// Interface compliance check
var _ webdav.FileSystem = (*ServerFileSystem)(nil)
