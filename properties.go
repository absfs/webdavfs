package webdavfs

import (
	"encoding/xml"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// XML namespace constants
const (
	nsDAV = "DAV:"
)

// multistatus represents a WebDAV multistatus response
type multistatus struct {
	XMLName   xml.Name   `xml:"multistatus"`
	Responses []response `xml:"response"`
}

// response represents a single response within a multistatus
type response struct {
	Href     string   `xml:"href"`
	Propstat propstat `xml:"propstat"`
}

// propstat represents property status
type propstat struct {
	Prop   prop   `xml:"prop"`
	Status string `xml:"status"`
}

// prop represents WebDAV properties
type prop struct {
	DisplayName      string       `xml:"displayname"`
	GetContentLength string       `xml:"getcontentlength"`
	GetLastModified  string       `xml:"getlastmodified"`
	ResourceType     resourceType `xml:"resourcetype"`
	GetETag          string       `xml:"getetag"`
	GetContentType   string       `xml:"getcontenttype"`
	CreationDate     string       `xml:"creationdate"`
}

// resourceType indicates if a resource is a collection (directory)
type resourceType struct {
	Collection *struct{} `xml:"collection"`
}

// fileInfo implements os.FileInfo for WebDAV resources
type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() interface{}   { return nil }

// parseMultistatus parses a WebDAV multistatus XML response
func parseMultistatus(r io.Reader) (*multistatus, error) {
	var ms multistatus
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&ms); err != nil {
		return nil, err
	}
	return &ms, nil
}

// parseFileInfo converts a WebDAV response to os.FileInfo
func parseFileInfo(resp response, basePath string) (os.FileInfo, error) {
	// Extract the name from href
	href := strings.TrimPrefix(resp.Href, "/")
	name := path.Base(href)
	if name == "" || name == "/" {
		name = path.Base(basePath)
	}

	// Parse size
	var size int64
	if resp.Propstat.Prop.GetContentLength != "" {
		var err error
		size, err = strconv.ParseInt(resp.Propstat.Prop.GetContentLength, 10, 64)
		if err != nil {
			size = 0
		}
	}

	// Parse modification time
	modTime := time.Now()
	if resp.Propstat.Prop.GetLastModified != "" {
		if t, err := parseWebDAVTime(resp.Propstat.Prop.GetLastModified); err == nil {
			modTime = t
		}
	}

	// Determine if it's a directory
	isDir := resp.Propstat.Prop.ResourceType.Collection != nil

	// Set mode
	mode := os.FileMode(0644)
	if isDir {
		mode = os.FileMode(0755) | os.ModeDir
	}

	return &fileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		isDir:   isDir,
	}, nil
}

// parseWebDAVTime parses various WebDAV time formats
func parseWebDAVTime(s string) (time.Time, error) {
	// Try RFC1123 format (HTTP-date)
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return t, nil
	}

	// Try RFC3339 format (ISO 8601)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try common WebDAV format
	formats := []string{
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"2006-01-02T15:04:05Z",
		time.RFC1123Z,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, &os.PathError{
		Op:   "parse",
		Path: s,
		Err:  os.ErrInvalid,
	}
}

// buildPropfindBody creates a PROPFIND request body
func buildPropfindBody() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:getcontentlength/>
    <D:getlastmodified/>
    <D:resourcetype/>
    <D:getetag/>
    <D:getcontenttype/>
    <D:creationdate/>
  </D:prop>
</D:propfind>`
}

// buildProppatchBody creates a PROPPATCH request body for setting modification time
func buildProppatchBody(modTime time.Time) string {
	timeStr := modTime.UTC().Format(time.RFC1123)
	return `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:getlastmodified>` + timeStr + `</D:getlastmodified>
    </D:prop>
  </D:set>
</D:propertyupdate>`
}
