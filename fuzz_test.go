package webdavfs

import (
	"bufio"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// FuzzXMLParsing tests XML multistatus parsing with malformed input
// This tests for: billion laughs, XXE, invalid UTF-8, unclosed tags,
// deep nesting, and huge documents
func FuzzXMLParsing(f *testing.F) {
	// Seed with valid XML structures
	f.Add(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`)
	f.Add(`<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/path/to/file</D:href>
    <D:propstat>
      <D:prop><D:getcontentlength>1024</D:getcontentlength></D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`)
	f.Add(`<?xml version="1.0" encoding="utf-8"?>
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
</D:multistatus>`)
	// Seed with collection/directory response
	f.Add(`<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/dir/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`)

	f.Fuzz(func(t *testing.T, xmlData string) {
		var ms multistatus
		// Should not panic on any XML input
		err := xml.Unmarshal([]byte(xmlData), &ms)

		// If unmarshal succeeded, test property extraction
		if err == nil {
			for _, resp := range ms.Responses {
				// Access all fields without panicking
				_ = resp.Href
				_ = resp.Propstat.Prop.GetContentLength
				_ = resp.Propstat.Prop.GetLastModified
				_ = resp.Propstat.Prop.DisplayName
				_ = resp.Propstat.Prop.ResourceType.Collection
				_ = resp.Propstat.Prop.GetETag
				_ = resp.Propstat.Prop.GetContentType
				_ = resp.Propstat.Prop.CreationDate
				_ = resp.Propstat.Status

				// Test parseFileInfo with the response
				_, _ = parseFileInfo(resp, "/test")
			}
		}

		// Test parseMultistatus function
		_, _ = parseMultistatus(strings.NewReader(xmlData))
	})
}

// FuzzPathEncoding tests path encoding/decoding with edge cases
// This tests for: null bytes, unicode normalization, very long paths,
// special characters, and URL encoding tricks
func FuzzPathEncoding(f *testing.F) {
	f.Add("/normal/path.txt")
	f.Add("/path with spaces/file.txt")
	f.Add("/unicode/文件.txt")
	f.Add("/special/!@#$%^&*().txt")
	f.Add("/emoji/🎉.txt")
	f.Add("/dots/../parent.txt")
	f.Add("/multiple///slashes.txt")
	f.Add("/trailing/slash/")
	f.Add("relative/path.txt")
	f.Add("./current/dir.txt")

	f.Fuzz(func(t *testing.T, path string) {
		// Test URL path encoding
		encoded := url.PathEscape(path)

		// Should be able to decode back
		decoded, err := url.PathUnescape(encoded)
		if err != nil {
			return // Invalid encoding is ok
		}

		// Roundtrip should be stable
		if decoded != path {
			t.Errorf("roundtrip failed: %q → %q → %q", path, encoded, decoded)
		}

		// Test with url.URL
		u := &url.URL{Path: path}
		_ = u.String()

		// Test buildURL doesn't panic (using a mock client)
		testURL, _ := url.Parse("http://example.com/webdav/")
		client := &webdavClient{baseURL: testURL}
		_, _ = client.buildURL(path)
	})
}

// FuzzHTTPResponse tests HTTP response parsing with malformed data
// This tests for: invalid Content-Length, truncated responses,
// invalid status codes, and header injection
func FuzzHTTPResponse(f *testing.F) {
	f.Add("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n")
	f.Add("HTTP/1.1 404 Not Found\r\n\r\n")
	f.Add("HTTP/1.1 207 Multi-Status\r\nContent-Type: application/xml\r\n\r\n<?xml version=\"1.0\"?>")
	f.Add("HTTP/1.1 201 Created\r\n\r\n")
	f.Add("HTTP/1.1 204 No Content\r\n\r\n")
	f.Add("HTTP/1.1 401 Unauthorized\r\nWWW-Authenticate: Basic realm=\"WebDAV\"\r\n\r\n")
	f.Add("HTTP/1.1 500 Internal Server Error\r\n\r\nError message")

	f.Fuzz(func(t *testing.T, responseData string) {
		// Parse as HTTP response
		resp, err := http.ReadResponse(
			bufio.NewReader(strings.NewReader(responseData)),
			nil,
		)
		if err != nil {
			return // Malformed HTTP is expected
		}
		defer resp.Body.Close()

		// Should not panic reading body
		body, _ := io.ReadAll(resp.Body)

		// Test parsing response body as multistatus for 207 responses
		if resp.StatusCode == 207 && len(body) > 0 {
			_, _ = parseMultistatus(strings.NewReader(string(body)))
		}

		// Test httpStatusToOSError doesn't panic
		_ = httpStatusToOSError(resp.StatusCode, "/test")
	})
}

// FuzzAuthHeaders tests authentication header parsing
// This tests for: malformed Basic auth, invalid Bearer tokens,
// and header injection attempts
func FuzzAuthHeaders(f *testing.F) {
	f.Add("Basic dXNlcjpwYXNz")
	f.Add("Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	f.Add("Basic ")
	f.Add("Bearer ")
	f.Add("Invalid")
	f.Add("")
	f.Add("Basic dXNlcjpwYXNz extra")
	f.Add("Bearer token123")

	f.Fuzz(func(t *testing.T, authValue string) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			return
		}

		// Should handle any auth header without panic
		req.Header.Set("Authorization", authValue)
		_ = req.Header.Get("Authorization")

		// Test extracting auth type
		parts := strings.SplitN(authValue, " ", 2)
		if len(parts) > 0 {
			authType := parts[0]
			_ = strings.ToLower(authType)
		}

		// Test with webdavClient authentication setup
		if strings.HasPrefix(authValue, "Basic ") {
			// Basic auth is handled by SetBasicAuth
			req.SetBasicAuth("user", "pass")
		} else if strings.HasPrefix(authValue, "Bearer ") && len(parts) == 2 {
			// Bearer token
			token := parts[1]
			req.Header.Set("Authorization", "Bearer "+token)
		}
	})
}

// FuzzPropertyValues tests parsing of WebDAV property values
// This tests for: invalid integers, malformed timestamps,
// unusual MIME types, and other property value edge cases
func FuzzPropertyValues(f *testing.F) {
	// Content lengths
	f.Add("1024")
	f.Add("0")
	f.Add("9223372036854775807") // max int64

	// Timestamps
	f.Add("Mon, 02 Jan 2006 15:04:05 GMT")
	f.Add("Mon, 02 Jan 2006 15:04:05 MST")
	f.Add("2006-01-02T15:04:05Z")

	// Content types
	f.Add("text/plain")
	f.Add("application/octet-stream")
	f.Add("application/xml")

	// Invalid values
	f.Add("")
	f.Add("-1")
	f.Add("invalid")
	f.Add("NaN")

	f.Fuzz(func(t *testing.T, value string) {
		// Parse as content length (int64)
		size, err := strconv.ParseInt(value, 10, 64)
		if err == nil && size < 0 {
			// Negative sizes should be treated as 0
			size = 0
		}
		_ = size

		// Parse as timestamp
		_, _ = parseWebDAVTime(value)

		// Test in a prop struct
		p := prop{
			GetContentLength: value,
			GetLastModified:  value,
			GetContentType:   value,
			DisplayName:      value,
			GetETag:          value,
			CreationDate:     value,
		}

		// Test parseFileInfo with this prop
		resp := response{
			Href: "/test.txt",
			Propstat: propstat{
				Prop:   p,
				Status: "HTTP/1.1 200 OK",
			},
		}
		_, _ = parseFileInfo(resp, "/test")
	})
}
