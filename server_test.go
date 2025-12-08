package webdavfs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/absfs/memfs"
)

func TestServerFileSystemMkdir(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	err = sfs.Mkdir(context.Background(), "/testdir", 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	info, err := sfs.Stat(context.Background(), "/testdir")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory")
	}
}

func TestServerFileSystemOpenFile(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	// Create a file
	f, err := sfs.OpenFile(context.Background(), "/test.txt", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("OpenFile create failed: %v", err)
	}

	data := []byte("Hello, WebDAV Server!")
	n, err := f.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write returned %d, expected %d", n, len(data))
	}

	f.Close()

	// Read it back
	f, err = sfs.OpenFile(context.Background(), "/test.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile read failed: %v", err)
	}
	defer f.Close()

	buf := make([]byte, len(data))
	n, err = f.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read failed: %v", err)
	}

	if string(buf[:n]) != string(data) {
		t.Errorf("Data mismatch: got %q, want %q", buf[:n], data)
	}
}

func TestServerFileSystemRemoveAll(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	// Create a directory with a file
	err = sfs.Mkdir(context.Background(), "/testdir", 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	f, err := sfs.OpenFile(context.Background(), "/testdir/file.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	f.Write([]byte("test"))
	f.Close()

	// Remove all
	err = sfs.RemoveAll(context.Background(), "/testdir")
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	// Verify it's gone
	_, err = sfs.Stat(context.Background(), "/testdir")
	if err == nil {
		t.Error("Expected error after RemoveAll, got nil")
	}
}

func TestServerFileSystemRename(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	// Create a file
	f, err := sfs.OpenFile(context.Background(), "/old.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	f.Write([]byte("content"))
	f.Close()

	// Rename it
	err = sfs.Rename(context.Background(), "/old.txt", "/new.txt")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	// Verify old is gone
	_, err = sfs.Stat(context.Background(), "/old.txt")
	if err == nil {
		t.Error("Expected error for old path, got nil")
	}

	// Verify new exists
	info, err := sfs.Stat(context.Background(), "/new.txt")
	if err != nil {
		t.Fatalf("Stat new path failed: %v", err)
	}
	if info.Name() != "new.txt" {
		t.Errorf("Expected name 'new.txt', got %q", info.Name())
	}
}

func TestServerFileReadWrite(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	// Create and write
	f, err := sfs.OpenFile(context.Background(), "/test.txt", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	data := []byte("Hello, WebDAV Server!")
	n, err := f.Write(data)
	if err != nil || n != len(data) {
		t.Fatalf("Write failed: %v", err)
	}

	// Seek back and read
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	buf := make([]byte, len(data))
	n, err = f.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read failed: %v", err)
	}

	if string(buf[:n]) != string(data) {
		t.Errorf("Data mismatch: got %q, want %q", buf[:n], data)
	}

	f.Close()
}

func TestServerFileReaddir(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	// Create some files
	fs.Mkdir("/testdir", 0755)
	f1, _ := fs.Create("/testdir/file1.txt")
	f1.Close()
	f2, _ := fs.Create("/testdir/file2.txt")
	f2.Close()
	fs.Mkdir("/testdir/subdir", 0755)

	sfs := NewServerFileSystem(fs)

	// Open directory
	dir, err := sfs.OpenFile(context.Background(), "/testdir", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer dir.Close()

	// Read entries
	entries, err := dir.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	expected := []string{"file1.txt", "file2.txt", "subdir"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("Missing entry: %s", name)
		}
	}
}

func TestServerFileStat(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	sfs := NewServerFileSystem(fs)

	f, err := sfs.OpenFile(context.Background(), "/test.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	f.Write([]byte("hello world"))
	f.Close()

	f, err = sfs.OpenFile(context.Background(), "/test.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %q", info.Name())
	}
	if info.Size() != 11 {
		t.Errorf("Expected size 11, got %d", info.Size())
	}
	if info.IsDir() {
		t.Error("Expected file, got directory")
	}
}

func TestServerHTTPIntegration(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	// Pre-populate
	fs.MkdirAll("/docs", 0755)
	f, _ := fs.Create("/docs/test.txt")
	f.Write([]byte("content"))
	f.Close()

	server := NewServer(fs, nil)
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Test OPTIONS (WebDAV discovery)
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("OPTIONS: expected 200, got %d", resp.StatusCode)
	}

	// Test PROPFIND (directory listing)
	req, _ = http.NewRequest("PROPFIND", ts.URL+"/docs", nil)
	req.Header.Set("Depth", "1")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PROPFIND failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 207 {
		t.Errorf("PROPFIND: expected 207 Multi-Status, got %d\nBody: %s", resp.StatusCode, body)
	}

	// Test GET (file download)
	resp, err = http.Get(ts.URL + "/docs/test.txt")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET: expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "content" {
		t.Errorf("GET content mismatch: got %q", body)
	}

	// Test PUT (file upload)
	req, _ = http.NewRequest("PUT", ts.URL+"/docs/new.txt", strings.NewReader("new content"))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 204 {
		t.Errorf("PUT: expected 201 or 204, got %d", resp.StatusCode)
	}

	// Verify PUT worked
	resp, err = http.Get(ts.URL + "/docs/new.txt")
	if err != nil {
		t.Fatalf("GET new file failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "new content" {
		t.Errorf("PUT verification failed: got %q", body)
	}

	// Test DELETE
	req, _ = http.NewRequest("DELETE", ts.URL+"/docs/new.txt", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("DELETE: expected 204, got %d", resp.StatusCode)
	}

	// Verify DELETE worked
	resp, err = http.Get(ts.URL + "/docs/new.txt")
	if err != nil {
		t.Fatalf("GET after DELETE failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("Expected 404 after DELETE, got %d", resp.StatusCode)
	}

	// Test MKCOL (create directory)
	req, _ = http.NewRequest("MKCOL", ts.URL+"/newdir", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MKCOL failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("MKCOL: expected 201, got %d", resp.StatusCode)
	}
}

func TestBasicAuth(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(fs, &ServerConfig{
		Auth: &BasicAuth{
			Realm: "Test",
			Validator: func(u, p string) bool {
				return u == "user" && p == "pass"
			},
		},
	})
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Without auth - should fail
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 without auth, got %d", resp.StatusCode)
	}

	// Check WWW-Authenticate header
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "Basic") {
		t.Errorf("Expected Basic challenge, got %q", wwwAuth)
	}

	// With wrong credentials - should fail
	req, _ := http.NewRequest("PROPFIND", ts.URL+"/", nil)
	req.SetBasicAuth("wrong", "creds")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 with wrong creds, got %d", resp.StatusCode)
	}

	// With correct credentials - should succeed
	req, _ = http.NewRequest("PROPFIND", ts.URL+"/", nil)
	req.SetBasicAuth("user", "pass")
	req.Header.Set("Depth", "0")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 207 {
		t.Errorf("Expected 207 with correct creds, got %d", resp.StatusCode)
	}
}

func TestBearerAuth(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	validToken := "secret-token-12345"

	server := NewServer(fs, &ServerConfig{
		Auth: &BearerAuth{
			Realm: "API",
			Validator: func(token string) bool {
				return token == validToken
			},
		},
	})
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Without auth - should fail
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 without auth, got %d", resp.StatusCode)
	}

	// Check WWW-Authenticate header
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "Bearer") {
		t.Errorf("Expected Bearer challenge, got %q", wwwAuth)
	}

	// With wrong token - should fail
	req, _ := http.NewRequest("PROPFIND", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 with wrong token, got %d", resp.StatusCode)
	}

	// With correct token - should succeed
	req, _ = http.NewRequest("PROPFIND", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	req.Header.Set("Depth", "0")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 207 {
		t.Errorf("Expected 207 with correct token, got %d", resp.StatusCode)
	}
}

func TestServerPrefix(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	fs.Create("/file.txt")

	server := NewServer(fs, &ServerConfig{
		Prefix: "/webdav",
	})
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Request without prefix should fail
	resp, err := http.Get(ts.URL + "/file.txt")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("Expected 404 without prefix, got %d", resp.StatusCode)
	}

	// Request with prefix should succeed
	resp, err = http.Get(ts.URL + "/webdav/file.txt")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 with prefix, got %d", resp.StatusCode)
	}
}

func TestServerLogger(t *testing.T) {
	fs, err := memfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	var logged []string
	server := NewServer(fs, &ServerConfig{
		Logger: func(r *http.Request, err error) {
			logged = append(logged, r.Method+" "+r.URL.Path)
		},
	})
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Make some requests
	http.Get(ts.URL + "/")
	req, _ := http.NewRequest("PROPFIND", ts.URL+"/", nil)
	req.Header.Set("Depth", "0")
	http.DefaultClient.Do(req)

	if len(logged) < 2 {
		t.Errorf("Expected at least 2 logged requests, got %d", len(logged))
	}
}
