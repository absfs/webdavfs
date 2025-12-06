package webdavfs

import (
	"bytes"
	"io"
	"os"

	"github.com/absfs/absfs"
)

// File represents an open file in the WebDAV filesystem
type File struct {
	fs       *FileSystem
	path     string
	flag     int
	offset   int64
	info     os.FileInfo
	buffer   *bytes.Buffer // For write buffering
	modified bool
	closed   bool
	reader   io.ReadCloser // For reading
	dirIndex int           // For directory iteration
	dirInfos []os.FileInfo // Cached directory contents
}

// Read reads data from the file
func (f *File) Read(b []byte) (int, error) {
	if f.closed {
		return 0, &FileClosedError{Path: f.path}
	}

	// Check if file is opened for reading
	if f.flag&os.O_WRONLY != 0 {
		return 0, &os.PathError{Op: "read", Path: f.path, Err: os.ErrInvalid}
	}

	// If this is a directory, cannot read
	if f.info.IsDir() {
		return 0, &os.PathError{Op: "read", Path: f.path, Err: os.ErrInvalid}
	}

	// Initialize reader if needed
	if f.reader == nil {
		reader, err := f.fs.client.get(f.path, f.offset)
		if err != nil {
			return 0, err
		}
		f.reader = reader
	}

	n, err := f.reader.Read(b)
	f.offset += int64(n)
	return n, err
}

// Write writes data to the file
func (f *File) Write(b []byte) (int, error) {
	if f.closed {
		return 0, &FileClosedError{Path: f.path}
	}

	// Check if file is opened for writing
	if f.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, &os.PathError{Op: "write", Path: f.path, Err: os.ErrInvalid}
	}

	// If this is a directory, cannot write
	if f.info.IsDir() {
		return 0, &os.PathError{Op: "write", Path: f.path, Err: os.ErrInvalid}
	}

	// Initialize buffer if needed
	if f.buffer == nil {
		f.buffer = &bytes.Buffer{}
	}

	n, err := f.buffer.Write(b)
	if err != nil {
		return n, err
	}

	f.offset += int64(n)
	f.modified = true
	return n, nil
}

// Close closes the file
func (f *File) Close() error {
	if f.closed {
		return nil
	}

	f.closed = true

	// Close reader if open
	if f.reader != nil {
		f.reader.Close()
	}

	// Flush writes if modified
	if f.modified && f.buffer != nil {
		if err := f.fs.client.put(f.path, f.buffer); err != nil {
			return err
		}
	}

	return nil
}

// Seek sets the offset for the next Read or Write
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, &FileClosedError{Path: f.path}
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		if f.info == nil {
			var err error
			f.info, err = f.fs.client.stat(f.path)
			if err != nil {
				return 0, err
			}
		}
		newOffset = f.info.Size() + offset
	default:
		return 0, &InvalidSeekError{Offset: offset, Whence: whence}
	}

	if newOffset < 0 {
		return 0, &InvalidSeekError{Offset: offset, Whence: whence}
	}

	// If we have an active reader and offset changed, close it
	if f.reader != nil && newOffset != f.offset {
		f.reader.Close()
		f.reader = nil
	}

	f.offset = newOffset
	return f.offset, nil
}

// Stat returns file information
func (f *File) Stat() (os.FileInfo, error) {
	if f.closed {
		return nil, &FileClosedError{Path: f.path}
	}

	if f.info != nil {
		return f.info, nil
	}

	info, err := f.fs.client.stat(f.path)
	if err != nil {
		return nil, err
	}

	f.info = info
	return f.info, nil
}

// ReadAt reads from the file at a specific offset
func (f *File) ReadAt(b []byte, off int64) (int, error) {
	if f.closed {
		return 0, &FileClosedError{Path: f.path}
	}

	// Check if file is opened for reading
	if f.flag&os.O_WRONLY != 0 {
		return 0, &os.PathError{Op: "read", Path: f.path, Err: os.ErrInvalid}
	}

	reader, err := f.fs.client.get(f.path, off)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	return io.ReadFull(reader, b)
}

// WriteAt writes to the file at a specific offset
func (f *File) WriteAt(b []byte, off int64) (int, error) {
	if f.closed {
		return 0, &FileClosedError{Path: f.path}
	}

	// Check if file is opened for writing
	if f.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, &os.PathError{Op: "write", Path: f.path, Err: os.ErrInvalid}
	}

	// Use putRange for partial updates
	if err := f.fs.client.putRange(f.path, b, off); err != nil {
		return 0, err
	}

	return len(b), nil
}

// Readdir reads directory contents
func (f *File) Readdir(n int) ([]os.FileInfo, error) {
	if f.closed {
		return nil, &FileClosedError{Path: f.path}
	}

	if !f.info.IsDir() {
		return nil, &os.PathError{Op: "readdir", Path: f.path, Err: os.ErrInvalid}
	}

	// Load directory contents if not cached
	if f.dirInfos == nil {
		infos, err := f.fs.client.readDir(f.path)
		if err != nil {
			return nil, err
		}
		f.dirInfos = infos
		f.dirIndex = 0
	}

	// Return all remaining entries if n <= 0
	if n <= 0 {
		result := f.dirInfos[f.dirIndex:]
		f.dirIndex = len(f.dirInfos)
		if len(result) == 0 {
			return nil, io.EOF
		}
		return result, nil
	}

	// Return n entries
	if f.dirIndex >= len(f.dirInfos) {
		return nil, io.EOF
	}

	end := f.dirIndex + n
	if end > len(f.dirInfos) {
		end = len(f.dirInfos)
	}

	result := f.dirInfos[f.dirIndex:end]
	f.dirIndex = end

	return result, nil
}

// Readdirnames reads directory entry names
func (f *File) Readdirnames(n int) ([]string, error) {
	infos, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name()
	}

	return names, nil
}

// Truncate changes the size of the file
func (f *File) Truncate(size int64) error {
	if f.closed {
		return &FileClosedError{Path: f.path}
	}

	// Check if file is opened for writing
	if f.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return &os.PathError{Op: "truncate", Path: f.path, Err: os.ErrInvalid}
	}

	// If truncating to 0, just clear the buffer
	if size == 0 {
		if f.buffer != nil {
			f.buffer.Reset()
		} else {
			f.buffer = &bytes.Buffer{}
		}
		f.modified = true
		return nil
	}

	// For other sizes, we need to read current content and resize
	// This is a simplified implementation - servers may not support this
	return &os.PathError{Op: "truncate", Path: f.path, Err: os.ErrInvalid}
}

// Sync flushes buffered writes
func (f *File) Sync() error {
	if f.closed {
		return &FileClosedError{Path: f.path}
	}

	if f.modified && f.buffer != nil {
		if err := f.fs.client.put(f.path, f.buffer); err != nil {
			return err
		}
		f.modified = false
	}

	return nil
}

// Name returns the file name
func (f *File) Name() string {
	return f.path
}

// WriteString writes a string to the file
func (f *File) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Interface compliance check
var _ absfs.File = (*File)(nil)
