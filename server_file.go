package webdavfs

import (
	"io"
	"os"

	"github.com/absfs/absfs"
	"golang.org/x/net/webdav"
)

// ServerFile adapts absfs.File to webdav.File.
// It wraps an absfs.File to provide the interface required by
// golang.org/x/net/webdav for serving files via WebDAV protocol.
type ServerFile struct {
	file absfs.File
}

// Close closes the file.
func (f *ServerFile) Close() error {
	return f.file.Close()
}

// Read reads up to len(p) bytes into p.
func (f *ServerFile) Read(p []byte) (int, error) {
	return f.file.Read(p)
}

// Seek sets the offset for the next Read or Write.
func (f *ServerFile) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

// Readdir reads the contents of the directory and returns a slice of
// os.FileInfo values, as would be returned by Stat, in directory order.
func (f *ServerFile) Readdir(count int) ([]os.FileInfo, error) {
	return f.file.Readdir(count)
}

// Stat returns the FileInfo structure describing the file.
func (f *ServerFile) Stat() (os.FileInfo, error) {
	return f.file.Stat()
}

// Write writes len(p) bytes from p to the file.
func (f *ServerFile) Write(p []byte) (int, error) {
	return f.file.Write(p)
}

// Interface compliance checks
var _ webdav.File = (*ServerFile)(nil)
var _ io.ReadSeeker = (*ServerFile)(nil)
var _ io.Writer = (*ServerFile)(nil)
