// Package webdavfs provides a WebDAV filesystem implementation for the absfs ecosystem.
package webdavfs

import (
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/absfs/absfs"
)

// FileSystem implements the absfs.FileSystem interface for WebDAV servers
type FileSystem struct {
	client  *webdavClient
	root    string
	cwd     string
	tempDir string
}

// New creates a new WebDAV filesystem
func New(config *Config) (*FileSystem, error) {
	if config == nil {
		return nil, &ConfigError{Field: "config", Reason: "config cannot be nil"}
	}

	// Set defaults and validate
	config.setDefaults()
	if err := config.validate(); err != nil {
		return nil, err
	}

	// Create WebDAV client
	client, err := newWebDAVClient(config)
	if err != nil {
		return nil, err
	}

	return &FileSystem{
		client:  client,
		root:    "/",
		cwd:     "/",
		tempDir: config.TempDir,
	}, nil
}

// cleanPath normalizes a path
func (fs *FileSystem) cleanPath(name string) string {
	// Handle absolute paths
	if path.IsAbs(name) {
		return path.Clean(name)
	}
	// Join with current working directory
	return path.Clean(path.Join(fs.cwd, name))
}

// OpenFile opens a file with the specified flags and permissions
func (fs *FileSystem) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	name = fs.cleanPath(name)

	// Check if file exists
	info, err := fs.client.stat(name)
	if err != nil {
		// File doesn't exist
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Creating new file
		if flag&os.O_CREATE == 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
		}

		// Create empty file
		if err := fs.client.put(name, strings.NewReader("")); err != nil {
			return nil, err
		}

		// Get info for the new file
		info, err = fs.client.stat(name)
		if err != nil {
			return nil, err
		}
	} else {
		// File exists
		if flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrExist}
		}

		// Truncate if requested
		if flag&os.O_TRUNC != 0 && !info.IsDir() {
			if err := fs.client.put(name, strings.NewReader("")); err != nil {
				return nil, err
			}
		}
	}

	f := &File{
		fs:   fs,
		path: name,
		flag: flag,
		info: info,
	}

	// Set initial offset for append mode
	if flag&os.O_APPEND != 0 {
		f.offset = info.Size()
	}

	return f, nil
}

// Open opens a file for reading
func (fs *FileSystem) Open(name string) (absfs.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// Create creates a new file for writing
func (fs *FileSystem) Create(name string) (absfs.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Mkdir creates a directory
func (fs *FileSystem) Mkdir(name string, perm os.FileMode) error {
	name = fs.cleanPath(name)
	return fs.client.mkcol(name)
}

// MkdirAll creates a directory and all parent directories
func (fs *FileSystem) MkdirAll(name string, perm os.FileMode) error {
	name = fs.cleanPath(name)

	// Check if it already exists
	if info, err := fs.client.stat(name); err == nil {
		if info.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: name, Err: os.ErrExist}
	}

	// Create parent directories recursively
	parent := path.Dir(name)
	if parent != "/" && parent != "." {
		if err := fs.MkdirAll(parent, perm); err != nil {
			return err
		}
	}

	// Create the directory
	return fs.client.mkcol(name)
}

// Remove removes a file or empty directory
func (fs *FileSystem) Remove(name string) error {
	name = fs.cleanPath(name)
	return fs.client.delete(name)
}

// RemoveAll removes a path and all children
func (fs *FileSystem) RemoveAll(name string) error {
	name = fs.cleanPath(name)
	return fs.client.delete(name)
}

// Rename renames (moves) a file or directory
func (fs *FileSystem) Rename(oldpath, newpath string) error {
	oldpath = fs.cleanPath(oldpath)
	newpath = fs.cleanPath(newpath)
	return fs.client.move(oldpath, newpath)
}

// Stat returns file information
func (fs *FileSystem) Stat(name string) (os.FileInfo, error) {
	name = fs.cleanPath(name)
	return fs.client.stat(name)
}

// Chmod changes file permissions (limited WebDAV support)
func (fs *FileSystem) Chmod(name string, mode os.FileMode) error {
	name = fs.cleanPath(name)
	// Most WebDAV servers don't support chmod
	// Check if file exists
	_, err := fs.client.stat(name)
	return err
}

// Chown changes file ownership (not supported by WebDAV)
func (fs *FileSystem) Chown(name string, uid, gid int) error {
	name = fs.cleanPath(name)
	// WebDAV doesn't support chown
	// Check if file exists
	_, err := fs.client.stat(name)
	return err
}

// Chtimes changes file modification time
func (fs *FileSystem) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = fs.cleanPath(name)
	return fs.client.proppatch(name, mtime)
}

// Separator returns the path separator
func (fs *FileSystem) Separator() uint8 {
	return '/'
}

// ListSeparator returns the list separator
func (fs *FileSystem) ListSeparator() uint8 {
	return ':'
}

// Chdir changes the current working directory
func (fs *FileSystem) Chdir(dir string) error {
	dir = fs.cleanPath(dir)

	// Check if directory exists
	info, err := fs.client.stat(dir)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return &os.PathError{Op: "chdir", Path: dir, Err: os.ErrInvalid}
	}

	fs.cwd = dir
	return nil
}

// Getwd returns the current working directory
func (fs *FileSystem) Getwd() (string, error) {
	return fs.cwd, nil
}

// TempDir returns the temporary directory path
func (fs *FileSystem) TempDir() string {
	return fs.tempDir
}

// Truncate truncates a file to a specified size
func (fs *FileSystem) Truncate(name string, size int64) error {
	name = fs.cleanPath(name)

	if size == 0 {
		// Truncate to zero by uploading empty content
		return fs.client.put(name, strings.NewReader(""))
	}

	// For non-zero sizes, this is complex with WebDAV
	// Would need to download, truncate, and re-upload
	return &os.PathError{Op: "truncate", Path: name, Err: os.ErrInvalid}
}

// ReadFile reads the entire file
func (fs *FileSystem) ReadFile(name string) ([]byte, error) {
	f, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, &os.PathError{Op: "read", Path: name, Err: os.ErrInvalid}
	}

	data := make([]byte, info.Size())
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return data[:n], nil
}

// WriteFile writes data to a file
func (fs *FileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := fs.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return f.Close()
}

// Close closes the filesystem connection
func (fs *FileSystem) Close() error {
	// Nothing to clean up for WebDAV client
	return nil
}

// Interface compliance check
var _ absfs.FileSystem = (*FileSystem)(nil)
