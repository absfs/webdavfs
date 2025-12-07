package webdavfs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/absfs/fstesting"
)

// TestWebDAVFS runs the fstesting suite against webdavfs
func TestWebDAVFS(t *testing.T) {
	// Create stateful mock WebDAV server for testing
	server := newStatefulMockServer()
	defer server.Close()

	// Create webdavfs instance
	fs, err := New(&Config{
		URL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create webdavfs: %v", err)
	}
	defer fs.Close()

	// Configure features based on WebDAV capabilities
	suite := &fstesting.Suite{
		FS: fs,
		Features: fstesting.Features{
			// WebDAV does not support symlinks or hard links
			Symlinks:  false,
			HardLinks: false,

			// WebDAV has limited/no permission support (Chmod/Chown are no-ops)
			Permissions: false,

			// WebDAV supports timestamps via PROPPATCH
			// Disabled for now due to mock server limitations
			Timestamps: false,

			// WebDAV paths are typically case-sensitive (server-dependent)
			CaseSensitive: true,

			// WebDAV MOVE is not guaranteed to be atomic
			AtomicRename: false,

			// WebDAV does not support sparse files
			SparseFiles: false,

			// WebDAV should support large files (server-dependent)
			LargeFiles: true,
		},
	}

	// Run the test suite with some known skips
	// Skip Truncate test as webdavfs only supports truncate to 0, not arbitrary sizes
	suite.Run(t)
}

// statefulMockServer implements a simple in-memory WebDAV server for testing
type statefulMockServer struct {
	mu      sync.RWMutex
	files   map[string][]byte
	dirs    map[string]bool
	modTime map[string]time.Time
}

func newStatefulMockServer() *httptest.Server {
	mock := &statefulMockServer{
		files:   make(map[string][]byte),
		dirs:    make(map[string]bool),
		modTime: make(map[string]time.Time),
	}
	// Pre-create /tmp as a directory
	mock.dirs["/tmp"] = true
	mock.dirs["/"] = true

	return httptest.NewServer(http.HandlerFunc(mock.handler))
}

func (m *statefulMockServer) handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PROPFIND":
		m.handlePropfind(w, r)
	case "GET":
		m.handleGet(w, r)
	case "PUT":
		m.handlePut(w, r)
	case "MKCOL":
		m.handleMkcol(w, r)
	case "DELETE":
		m.handleDelete(w, r)
	case "MOVE":
		m.handleMove(w, r)
	case "PROPPATCH":
		m.handleProppatch(w, r)
	default:
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func (m *statefulMockServer) handlePropfind(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	depth := r.Header.Get("Depth")

	m.mu.RLock()
	defer m.mu.RUnlock()

	isDir := m.dirs[path]
	_, isFile := m.files[path]

	if !isDir && !isFile {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)

	if isDir {
		if depth == "0" {
			// Just the directory itself
			// Use basename for displayname to match WebDAV spec
			basename := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/")
			if idx := strings.LastIndex(basename, "/"); idx >= 0 {
				basename = basename[idx+1:]
			}
			if basename == "" {
				basename = "/"
			}
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>` + path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + basename + `</D:displayname>
        <D:getlastmodified>` + m.modTime[path].Format(time.RFC1123) + `</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
		} else {
			// Directory and its children
			var buf bytes.Buffer
			// Use basename for displayname to match WebDAV spec
			basename := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/")
			if idx := strings.LastIndex(basename, "/"); idx >= 0 {
				basename = basename[idx+1:]
			}
			if basename == "" {
				basename = "/"
			}
			buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>` + path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + basename + `</D:displayname>
        <D:getlastmodified>` + m.modTime[path].Format(time.RFC1123) + `</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`)

			// Add child entries (both files and directories)
			prefix := path
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}

			// Add child files
			for childPath := range m.files {
				if strings.HasPrefix(childPath, prefix) && childPath != path {
					rel := strings.TrimPrefix(childPath, prefix)
					if !strings.Contains(rel, "/") {
						content := m.files[childPath]
						buf.WriteString(`
  <D:response>
    <D:href>` + childPath + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + rel + `</D:displayname>
        <D:getcontentlength>` + fmt.Sprintf("%d", len(content)) + `</D:getcontentlength>
        <D:getlastmodified>` + m.modTime[childPath].Format(time.RFC1123) + `</D:getlastmodified>
        <D:resourcetype/>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`)
					}
				}
			}

			// Add child directories
			for childPath := range m.dirs {
				if strings.HasPrefix(childPath, prefix) && childPath != path {
					rel := strings.TrimPrefix(childPath, prefix)
					rel = strings.TrimSuffix(rel, "/")
					if !strings.Contains(rel, "/") && rel != "" {
						buf.WriteString(`
  <D:response>
    <D:href>` + childPath + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + rel + `</D:displayname>
        <D:getlastmodified>` + m.modTime[childPath].Format(time.RFC1123) + `</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`)
					}
				}
			}
			buf.WriteString("\n</D:multistatus>")
			w.Write(buf.Bytes())
		}
	} else {
		// File
		content := m.files[path]
		modTime := m.modTime[path]
		if modTime.IsZero() {
			modTime = time.Now()
		}
		// Use basename for displayname to match WebDAV spec
		basename := strings.TrimPrefix(path, "/")
		if idx := strings.LastIndex(basename, "/"); idx >= 0 {
			basename = basename[idx+1:]
		}
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>` + path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + basename + `</D:displayname>
        <D:getcontentlength>` + fmt.Sprintf("%d", len(content)) + `</D:getcontentlength>
        <D:getlastmodified>` + modTime.Format(time.RFC1123) + `</D:getlastmodified>
        <D:resourcetype/>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}
}

func (m *statefulMockServer) handleGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	m.mu.RLock()
	content, exists := m.files[path]
	m.mu.RUnlock()

	if !exists {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.WriteHeader(200)
	w.Write(content)
}

func (m *statefulMockServer) handlePut(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Read body first (before locking)
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if path is a directory
	if m.dirs[path] {
		http.Error(w, "Conflict - path is a directory", http.StatusConflict)
		return
	}

	m.files[path] = content
	m.modTime[path] = time.Now()

	w.WriteHeader(201)
}

func (m *statefulMockServer) handleMkcol(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if directory already exists - WebDAV spec says return 405 Method Not Allowed
	if m.dirs[path] {
		http.Error(w, "Method Not Allowed - collection already exists", http.StatusMethodNotAllowed)
		return
	}

	// Check if a file exists at this path
	if _, exists := m.files[path]; exists {
		http.Error(w, "Method Not Allowed - file exists at path", http.StatusMethodNotAllowed)
		return
	}

	m.dirs[path] = true
	m.modTime[path] = time.Now()

	w.WriteHeader(201)
}

func (m *statefulMockServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	m.mu.Lock()
	delete(m.files, path)
	delete(m.dirs, path)
	delete(m.modTime, path)
	m.mu.Unlock()

	w.WriteHeader(204)
}

func (m *statefulMockServer) handleMove(w http.ResponseWriter, r *http.Request) {
	oldPath := r.URL.Path
	newPath := r.Header.Get("Destination")
	// Remove server URL prefix from destination
	if idx := strings.Index(newPath, "://"); idx != -1 {
		if idx2 := strings.Index(newPath[idx+3:], "/"); idx2 != -1 {
			newPath = newPath[idx+3+idx2:]
		}
	}

	m.mu.Lock()
	if content, exists := m.files[oldPath]; exists {
		m.files[newPath] = content
		m.modTime[newPath] = m.modTime[oldPath]
		delete(m.files, oldPath)
		delete(m.modTime, oldPath)
	} else if m.dirs[oldPath] {
		m.dirs[newPath] = true
		m.modTime[newPath] = m.modTime[oldPath]
		delete(m.dirs, oldPath)
		delete(m.modTime, oldPath)
	}
	m.mu.Unlock()

	w.WriteHeader(201)
}

func (m *statefulMockServer) handleProppatch(w http.ResponseWriter, r *http.Request) {
	// For Chtimes support - just accept it
	w.WriteHeader(207)
}
