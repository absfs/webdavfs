package webdavfs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// mockWebDAVServer creates a mock WebDAV server for testing
func mockWebDAVServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			handlePropfind(w, r)
		case "GET":
			handleGet(w, r)
		case "PUT":
			handlePut(w, r)
		case "MKCOL":
			handleMkcol(w, r)
		case "DELETE":
			handleDelete(w, r)
		case "MOVE":
			handleMove(w, r)
		default:
			http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		}
	}))
}

func handlePropfind(w http.ResponseWriter, r *http.Request) {
	depth := r.Header.Get("Depth")
	path := r.URL.Path

	if path == "/nonexistent" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Mock response for a file
	if path == "/test.txt" {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/test.txt</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>test.txt</D:displayname>
        <D:getcontentlength>11</D:getcontentlength>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype/>
        <D:getetag>"abc123"</D:getetag>
        <D:getcontenttype>text/plain</D:getcontenttype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
		return
	}

	// Mock response for directory stat (depth 0)
	if path == "/dir" || path == "/dir/" {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		if depth == "0" {
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/dir/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>dir</D:displayname>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
		} else {
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/dir/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>dir</D:displayname>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/dir/file1.txt</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>file1.txt</D:displayname>
        <D:getcontentlength>100</D:getcontentlength>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype/>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
		}
		return
	}

	// Default response for any other path
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)
	// Return a generic file response
	filename := path
	if filename == "" || filename == "/" {
		filename = "file"
	}
	w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>` + path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>` + filename + `</D:displayname>
        <D:getcontentlength>0</D:getcontentlength>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype/>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/test.txt" {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
		return
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func handleMkcol(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(204)
}

func handleMove(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func TestNew(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty URL",
			config: &Config{
				URL: "",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &Config{
				URL:      server.URL,
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "bearer token auth",
			config: &Config{
				URL:         server.URL,
				BearerToken: "token123",
			},
			wantErr: false,
		},
		{
			name: "conflicting auth",
			config: &Config{
				URL:         server.URL,
				Username:    "user",
				Password:    "pass",
				BearerToken: "token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && fs == nil {
				t.Error("New() returned nil filesystem")
			}
		})
	}
}

func TestFileSystem_Stat(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	t.Run("existing file", func(t *testing.T) {
		info, err := fs.Stat("/test.txt")
		if err != nil {
			t.Errorf("Stat() error = %v", err)
			return
		}
		if info.Name() != "test.txt" {
			t.Errorf("Expected name test.txt, got %s", info.Name())
		}
		if info.Size() != 11 {
			t.Errorf("Expected size 11, got %d", info.Size())
		}
		if info.IsDir() {
			t.Error("Expected file, got directory")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := fs.Stat("/nonexistent")
		if !os.IsNotExist(err) {
			t.Errorf("Expected ErrNotExist, got %v", err)
		}
	})
}

func TestFileSystem_OpenAndRead(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	data := make([]byte, 20)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		t.Errorf("Read() error = %v", err)
	}

	expected := "Hello World"
	if string(data[:n]) != expected {
		t.Errorf("Expected %q, got %q", expected, string(data[:n]))
	}
}

func TestFileSystem_MkdirAndChdir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.Mkdir("/testdir", 0755)
	if err != nil {
		t.Errorf("Mkdir() error = %v", err)
	}

	err = fs.Chdir("/dir")
	if err != nil {
		t.Errorf("Chdir() error = %v", err)
	}

	cwd, err := fs.Getwd()
	if err != nil {
		t.Errorf("Getwd() error = %v", err)
	}
	if cwd != "/dir" {
		t.Errorf("Expected cwd /dir, got %s", cwd)
	}
}

func TestFileSystem_Separator(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	if fs.Separator() != '/' {
		t.Errorf("Expected separator '/', got '%c'", fs.Separator())
	}

	if fs.ListSeparator() != ':' {
		t.Errorf("Expected list separator ':', got '%c'", fs.ListSeparator())
	}
}

func TestFileSystem_TempDir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	t.Run("default temp dir", func(t *testing.T) {
		fs, err := New(&Config{URL: server.URL})
		if err != nil {
			t.Fatalf("Failed to create filesystem: %v", err)
		}

		if fs.TempDir() != "/tmp" {
			t.Errorf("Expected temp dir /tmp, got %s", fs.TempDir())
		}
	})

	t.Run("custom temp dir", func(t *testing.T) {
		fs, err := New(&Config{
			URL:     server.URL,
			TempDir: "/custom/tmp",
		})
		if err != nil {
			t.Fatalf("Failed to create filesystem: %v", err)
		}

		if fs.TempDir() != "/custom/tmp" {
			t.Errorf("Expected temp dir /custom/tmp, got %s", fs.TempDir())
		}
	})
}

func TestParseWebDAVTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"Mon, 01 Jan 2024 00:00:00 GMT", false},
		{"Mon, 01 Jan 2024 00:00:00 MST", false},
		{"2024-01-01T00:00:00Z", false},
		{"invalid time", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseWebDAVTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWebDAVTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{
		URL: "https://example.com",
	}

	config.setDefaults()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.HTTPClient == nil {
		t.Error("Expected HTTPClient to be set")
	}

	if config.TempDir != "/tmp" {
		t.Errorf("Expected temp dir /tmp, got %s", config.TempDir)
	}
}

func TestFile_WriteAndClose(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/newfile.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	data := []byte("test data")
	n, err := f.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	err = f.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestFile_Seek(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	// Seek to beginning
	offset, err := f.Seek(0, io.SeekStart)
	if err != nil {
		t.Errorf("Seek(0, SeekStart) error = %v", err)
	}
	if offset != 0 {
		t.Errorf("Expected offset 0, got %d", offset)
	}

	// Seek to position 5
	offset, err = f.Seek(5, io.SeekStart)
	if err != nil {
		t.Errorf("Seek(5, SeekStart) error = %v", err)
	}
	if offset != 5 {
		t.Errorf("Expected offset 5, got %d", offset)
	}
}

func TestFileSystem_Rename(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.Rename("/test.txt", "/renamed.txt")
	if err != nil {
		t.Errorf("Rename() error = %v", err)
	}
}

func TestFileSystem_Remove(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.Remove("/test.txt")
	if err != nil {
		t.Errorf("Remove() error = %v", err)
	}
}

func TestFileSystem_ReadFile(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	data, err := fs.ReadFile("/test.txt")
	if err != nil {
		t.Errorf("ReadFile() error = %v", err)
	}

	expected := "Hello World"
	if string(data) != expected {
		t.Errorf("Expected %q, got %q", expected, string(data))
	}
}

func TestFileSystem_WriteFile(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	data := []byte("test content")
	err = fs.WriteFile("/newfile.txt", data, 0644)
	if err != nil {
		t.Errorf("WriteFile() error = %v", err)
	}
}

func TestHTTPStatusToOSError(t *testing.T) {
	tests := []struct {
		statusCode int
		wantErr    bool
	}{
		{404, true},
		{403, true},
		{409, true},
		{412, true},
		{423, true},
		{507, true},
		{500, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.statusCode)), func(t *testing.T) {
			err := httpStatusToOSError(tt.statusCode, "/test")
			if (err != nil) != tt.wantErr {
				t.Errorf("httpStatusToOSError() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCleanPath(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"/test.txt", "/test.txt"},
		{"test.txt", "/test.txt"},
		{"/dir/../test.txt", "/test.txt"},
		{"./test.txt", "/test.txt"},
		{"/dir/./test.txt", "/dir/test.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := fs.cleanPath(tt.input)
			if result != tt.expected {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFile_WriteString(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/writestring.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer f.Close()

	testStr := "Hello World"
	n, err := f.WriteString(testStr)
	if err != nil {
		t.Errorf("WriteString() error = %v", err)
	}
	if n != len(testStr) {
		t.Errorf("WriteString() = %d, want %d", n, len(testStr))
	}
}

func TestFile_Stat(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Errorf("Stat() error = %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "test.txt")
	}
}

func TestFile_Name(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	if f.Name() != "/test.txt" {
		t.Errorf("Name() = %q, want %q", f.Name(), "/test.txt")
	}
}

func TestFile_Sync(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/synctest.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer f.Close()

	_, err = f.Write([]byte("test data"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	err = f.Sync()
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

func TestFile_Truncate(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/trunctest.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer f.Close()

	_, err = f.Write([]byte("test data"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Truncate to 0 should succeed
	err = f.Truncate(0)
	if err != nil {
		t.Errorf("Truncate(0) error = %v", err)
	}

	// Truncate to non-zero should fail
	err = f.Truncate(5)
	if err == nil {
		t.Error("Truncate(5) expected error, got nil")
	}
}

func TestFileSystem_Truncate(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Truncate to 0 should succeed
	err = fs.Truncate("/test.txt", 0)
	if err != nil {
		t.Errorf("Truncate() error = %v", err)
	}

	// Truncate to non-zero should fail
	err = fs.Truncate("/test.txt", 5)
	if err == nil {
		t.Error("Truncate(5) expected error, got nil")
	}
}

func TestFileSystem_MkdirAll(t *testing.T) {
	// Create a custom mock server that handles the MkdirAll scenario properly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			path := r.URL.Path
			// Only /dir exists, everything else returns 404
			if path == "/dir" || path == "/dir/" {
				w.Header().Set("Content-Type", "application/xml; charset=utf-8")
				w.WriteHeader(207)
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/dir/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>dir</D:displayname>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			} else {
				http.Error(w, "Not Found", http.StatusNotFound)
			}
		case "MKCOL":
			w.WriteHeader(201)
		default:
			http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// MkdirAll on existing directory should not error
	err = fs.MkdirAll("/dir", 0755)
	if err != nil {
		t.Errorf("MkdirAll() error = %v", err)
	}

	// MkdirAll on nested path should create all directories
	err = fs.MkdirAll("/new/nested/dir", 0755)
	if err != nil {
		t.Errorf("MkdirAll() nested error = %v", err)
	}
}

func TestFileSystem_RemoveAll(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.RemoveAll("/test.txt")
	if err != nil {
		t.Errorf("RemoveAll() error = %v", err)
	}
}

func TestFileSystem_Chmod(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Chmod on existing file should not error
	err = fs.Chmod("/test.txt", 0644)
	if err != nil {
		t.Errorf("Chmod() error = %v", err)
	}
}

func TestFileSystem_Chown(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Chown on existing file should not error
	err = fs.Chown("/test.txt", 1000, 1000)
	if err != nil {
		t.Errorf("Chown() error = %v", err)
	}
}

func TestFileSystem_Chtimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			handlePropfind(w, r)
		case "PROPPATCH":
			w.WriteHeader(207)
		default:
			http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	now := time.Now()
	err = fs.Chtimes("/test.txt", now, now)
	if err != nil {
		t.Errorf("Chtimes() error = %v", err)
	}
}

func TestFileSystem_Close(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestFile_OperationsOnClosedFile(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Close the file
	f.Close()

	// Test operations on closed file
	_, err = f.Read(make([]byte, 10))
	if err == nil {
		t.Error("Read on closed file expected error")
	}

	_, err = f.Seek(0, io.SeekStart)
	if err == nil {
		t.Error("Seek on closed file expected error")
	}

	_, err = f.Stat()
	if err == nil {
		t.Error("Stat on closed file expected error")
	}
}

func TestFile_ReadOnWriteOnly(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.OpenFile("/test.txt", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	_, err = f.Read(make([]byte, 10))
	if err == nil {
		t.Error("Read on write-only file expected error")
	}
}

func TestFile_WriteOnReadOnly(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	_, err = f.Write([]byte("test"))
	if err == nil {
		t.Error("Write on read-only file expected error")
	}
}

func TestFile_Readdir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/dir")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	infos, err := f.Readdir(-1)
	if err != nil && err != io.EOF {
		t.Errorf("Readdir() error = %v", err)
	}
	if len(infos) == 0 {
		t.Error("Readdir() returned no entries")
	}
}

func TestFile_Readdirnames(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/dir")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	names, err := f.Readdirnames(-1)
	if err != nil && err != io.EOF {
		t.Errorf("Readdirnames() error = %v", err)
	}
	if len(names) == 0 {
		t.Error("Readdirnames() returned no entries")
	}
}

func TestFile_ReaddirOnFile(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	_, err = f.Readdir(-1)
	if err == nil {
		t.Error("Readdir on file expected error")
	}
}

func TestFile_SeekInvalid(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	// Invalid whence
	_, err = f.Seek(0, 999)
	if err == nil {
		t.Error("Seek with invalid whence expected error")
	}

	// Negative offset from start
	_, err = f.Seek(-1, io.SeekStart)
	if err == nil {
		t.Error("Seek with negative offset expected error")
	}
}

func TestFile_SeekEnd(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		t.Errorf("Seek(0, SeekEnd) error = %v", err)
	}
	if offset != 11 { // "Hello World" is 11 bytes
		t.Errorf("Seek(0, SeekEnd) = %d, want %d", offset, 11)
	}
}

func TestFile_SeekCurrent(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	// Read some data
	buf := make([]byte, 5)
	f.Read(buf)

	// Seek from current position
	offset, err := f.Seek(2, io.SeekCurrent)
	if err != nil {
		t.Errorf("Seek(2, SeekCurrent) error = %v", err)
	}
	if offset != 7 {
		t.Errorf("Seek(2, SeekCurrent) = %d, want %d", offset, 7)
	}
}

func TestFileSystem_OpenFileCreate(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Create new file
	f, err := fs.OpenFile("/newfile.txt", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Errorf("OpenFile(O_CREATE) error = %v", err)
	}
	f.Close()
}

func TestFileSystem_OpenFileExcl(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// O_EXCL on existing file should fail
	_, err = fs.OpenFile("/test.txt", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	if err == nil {
		t.Error("OpenFile(O_EXCL) on existing file expected error")
	}
}

func TestFileSystem_OpenFileAppend(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.OpenFile("/test.txt", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile(O_APPEND) error = %v", err)
	}
	defer f.Close()
}

func TestFileSystem_OpenFileTrunc(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.OpenFile("/test.txt", os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile(O_TRUNC) error = %v", err)
	}
	defer f.Close()
}

func TestFileSystem_ChdirToFile(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	err = fs.Chdir("/test.txt")
	if err == nil {
		t.Error("Chdir to file expected error")
	}
}

func TestErrors(t *testing.T) {
	t.Run("ConfigError", func(t *testing.T) {
		err := &ConfigError{Field: "URL", Reason: "test"}
		expected := "config error: URL: test"
		if err.Error() != expected {
			t.Errorf("ConfigError.Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("WebDAVError", func(t *testing.T) {
		err := &WebDAVError{StatusCode: 500, Method: "GET", Path: "/test", Message: "error"}
		if err.Error() == "" {
			t.Error("WebDAVError.Error() returned empty string")
		}
	})

	t.Run("FileClosedError", func(t *testing.T) {
		err := &FileClosedError{Path: "/test"}
		expected := "file already closed: /test"
		if err.Error() != expected {
			t.Errorf("FileClosedError.Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("InvalidSeekError", func(t *testing.T) {
		err := &InvalidSeekError{Offset: 10, Whence: 3}
		if err.Error() == "" {
			t.Error("InvalidSeekError.Error() returned empty string")
		}
	})
}

func TestFileInfo(t *testing.T) {
	fi := &fileInfo{
		name:    "test.txt",
		size:    100,
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}

	if fi.Name() != "test.txt" {
		t.Errorf("Name() = %q, want %q", fi.Name(), "test.txt")
	}
	if fi.Size() != 100 {
		t.Errorf("Size() = %d, want %d", fi.Size(), 100)
	}
	if fi.Mode() != 0644 {
		t.Errorf("Mode() = %v, want %v", fi.Mode(), os.FileMode(0644))
	}
	if fi.IsDir() {
		t.Error("IsDir() = true, want false")
	}
	if fi.Sys() != nil {
		t.Error("Sys() != nil")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	// This is a compile-time check that FileSystem implements absfs.FileSystem
	// The actual check is done in the source files with:
	// var _ absfs.FileSystem = (*FileSystem)(nil)
	// var _ absfs.File = (*File)(nil)

	server := mockWebDAVServer()
	defer server.Close()

	_, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}
}

func TestFile_ReadAtWriteAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			handlePropfind(w, r)
		case "GET":
			// Handle range requests
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				w.WriteHeader(206)
				w.Write([]byte("World"))
			} else {
				w.WriteHeader(200)
				w.Write([]byte("Hello World"))
			}
		case "PUT":
			w.WriteHeader(201)
		default:
			http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	t.Run("ReadAt", func(t *testing.T) {
		f, err := fs.Open("/test.txt")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer f.Close()

		buf := make([]byte, 5)
		n, err := f.ReadAt(buf, 6)
		if err != nil && err != io.EOF {
			t.Errorf("ReadAt() error = %v", err)
		}
		if n != 5 {
			t.Errorf("ReadAt() = %d, want %d", n, 5)
		}
	})

	t.Run("WriteAt", func(t *testing.T) {
		f, err := fs.OpenFile("/test.txt", os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("OpenFile() error = %v", err)
		}
		defer f.Close()

		data := []byte("test")
		n, err := f.WriteAt(data, 5)
		if err != nil {
			t.Errorf("WriteAt() error = %v", err)
		}
		if n != len(data) {
			t.Errorf("WriteAt() = %d, want %d", n, len(data))
		}
	})

	t.Run("ReadAt on write-only", func(t *testing.T) {
		f, err := fs.OpenFile("/test.txt", os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("OpenFile() error = %v", err)
		}
		defer f.Close()

		buf := make([]byte, 5)
		_, err = f.ReadAt(buf, 0)
		if err == nil {
			t.Error("ReadAt on write-only file expected error")
		}
	})

	t.Run("WriteAt on read-only", func(t *testing.T) {
		f, err := fs.Open("/test.txt")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer f.Close()

		_, err = f.WriteAt([]byte("test"), 0)
		if err == nil {
			t.Error("WriteAt on read-only file expected error")
		}
	})
}

func TestFile_ReaddirPagination(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/dir")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	// Read with n = 1
	infos, err := f.Readdir(1)
	if err != nil && err != io.EOF {
		t.Errorf("Readdir(1) error = %v", err)
	}
	if len(infos) > 1 {
		t.Errorf("Readdir(1) returned %d entries, want <= 1", len(infos))
	}
}

func TestFile_DoubleClose(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// First close should succeed
	err = f.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should not error
	err = f.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestFileSystem_ReadFileOnDir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	_, err = fs.ReadFile("/dir")
	if err == nil {
		t.Error("ReadFile on directory expected error")
	}
}

func TestFile_WriteCloseFlush(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/flushtest.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = f.Write([]byte("test data"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Close should flush the data
	err = f.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestFile_TruncateOnClosed(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/truncclosed.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	f.Close()

	err = f.Truncate(0)
	if err == nil {
		t.Error("Truncate on closed file expected error")
	}
}

func TestFile_SyncOnClosed(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/syncclosed.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	f.Close()

	err = f.Sync()
	if err == nil {
		t.Error("Sync on closed file expected error")
	}
}

func TestFile_ReaddirOnClosed(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/dir")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	f.Close()

	_, err = f.Readdir(-1)
	if err == nil {
		t.Error("Readdir on closed file expected error")
	}
}

func TestFile_WriteAtOnClosed(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Create("/writetest.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	f.Close()

	_, err = f.WriteAt([]byte("test"), 0)
	if err == nil {
		t.Error("WriteAt on closed file expected error")
	}
}

func TestFile_ReadAtOnClosed(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	f.Close()

	buf := make([]byte, 5)
	_, err = f.ReadAt(buf, 0)
	if err == nil {
		t.Error("ReadAt on closed file expected error")
	}
}

func TestFile_TruncateOnReadOnly(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	err = f.Truncate(0)
	if err == nil {
		t.Error("Truncate on read-only file expected error")
	}
}

func TestFile_ReadOnDir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.Open("/dir")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	_, err = f.Read(make([]byte, 10))
	if err == nil {
		t.Error("Read on directory expected error")
	}
}

func TestFile_WriteOnDir(t *testing.T) {
	server := mockWebDAVServer()
	defer server.Close()

	fs, err := New(&Config{URL: server.URL})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	f, err := fs.OpenFile("/dir", os.O_RDWR, 0755)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	_, err = f.Write([]byte("test"))
	if err == nil {
		t.Error("Write on directory expected error")
	}
}
