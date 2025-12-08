package webdavfs

import (
	"context"
	"os"

	"github.com/absfs/absfs"
	"golang.org/x/net/webdav"
)

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
func (s *ServerFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	return s.fs.Rename(oldName, newName)
}

// Stat returns file information.
// The context parameter is accepted for interface compliance but not used.
func (s *ServerFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	return s.fs.Stat(name)
}

// Interface compliance check
var _ webdav.FileSystem = (*ServerFileSystem)(nil)
