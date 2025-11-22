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
