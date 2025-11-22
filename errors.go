package webdavfs

import (
	"fmt"
	"os"
)

// ConfigError represents an error in the configuration
type ConfigError struct {
	Field  string
	Reason string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: %s: %s", e.Field, e.Reason)
}

// WebDAVError represents an error from the WebDAV server
type WebDAVError struct {
	StatusCode int
	Method     string
	Path       string
	Message    string
}

func (e *WebDAVError) Error() string {
	return fmt.Sprintf("webdav %s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
}

// httpStatusToOSError converts HTTP status codes to appropriate os package errors
func httpStatusToOSError(statusCode int, path string) error {
	switch statusCode {
	case 404:
		return &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	case 403:
		return &os.PathError{Op: "access", Path: path, Err: os.ErrPermission}
	case 409:
		// Conflict - typically parent directory doesn't exist
		return &os.PathError{Op: "create", Path: path, Err: os.ErrNotExist}
	case 412:
		// Precondition Failed - typically used for exists checks
		return &os.PathError{Op: "create", Path: path, Err: os.ErrExist}
	case 423:
		// Locked
		return &os.PathError{Op: "access", Path: path, Err: os.ErrPermission}
	case 507:
		// Insufficient Storage
		return &os.PathError{Op: "write", Path: path, Err: fmt.Errorf("insufficient storage")}
	default:
		return &os.PathError{Op: "webdav", Path: path, Err: fmt.Errorf("http status %d", statusCode)}
	}
}

// FileClosedError is returned when an operation is attempted on a closed file
type FileClosedError struct {
	Path string
}

func (e *FileClosedError) Error() string {
	return fmt.Sprintf("file already closed: %s", e.Path)
}

// InvalidSeekError is returned when an invalid seek operation is attempted
type InvalidSeekError struct {
	Offset int64
	Whence int
}

func (e *InvalidSeekError) Error() string {
	return fmt.Sprintf("invalid seek: offset=%d whence=%d", e.Offset, e.Whence)
}
