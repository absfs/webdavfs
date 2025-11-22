package webdavfs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// webdavClient handles HTTP communication with the WebDAV server
type webdavClient struct {
	httpClient *http.Client
	baseURL    *url.URL
	username   string
	password   string
	bearerToken string
}

// newWebDAVClient creates a new WebDAV client
func newWebDAVClient(config *Config) (*webdavClient, error) {
	baseURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, &ConfigError{Field: "URL", Reason: fmt.Sprintf("invalid URL: %v", err)}
	}

	// Ensure base URL ends with /
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}

	return &webdavClient{
		httpClient:  config.HTTPClient,
		baseURL:     baseURL,
		username:    config.Username,
		password:    config.Password,
		bearerToken: config.BearerToken,
	}, nil
}

// buildURL constructs the full URL for a path
func (c *webdavClient) buildURL(pathStr string) (*url.URL, error) {
	// Clean and normalize the path
	pathStr = path.Clean("/" + strings.TrimPrefix(pathStr, "/"))

	// Parse as URL to properly handle encoding
	u, err := url.Parse(c.baseURL.String())
	if err != nil {
		return nil, err
	}

	// Join paths
	u.Path = path.Join(u.Path, pathStr)

	return u, nil
}

// doRequest performs an HTTP request with authentication
func (c *webdavClient) doRequest(method, pathStr string, body io.Reader, headers map[string]string) (*http.Response, error) {
	reqURL, err := c.buildURL(pathStr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Add authentication
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	} else if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	// Add custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &os.PathError{Op: method, Path: pathStr, Err: err}
	}

	return resp, nil
}

// propfind performs a PROPFIND request
func (c *webdavClient) propfind(pathStr string, depth int) (*multistatus, error) {
	headers := map[string]string{
		"Content-Type": "application/xml",
		"Depth":        fmt.Sprintf("%d", depth),
	}

	body := buildPropfindBody()
	resp, err := c.doRequest("PROPFIND", pathStr, strings.NewReader(body), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, &os.PathError{Op: "stat", Path: pathStr, Err: os.ErrNotExist}
	}

	if resp.StatusCode != 207 { // 207 Multi-Status
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &WebDAVError{
			StatusCode: resp.StatusCode,
			Method:     "PROPFIND",
			Path:       pathStr,
			Message:    string(bodyBytes),
		}
	}

	ms, err := parseMultistatus(resp.Body)
	if err != nil {
		return nil, &os.PathError{Op: "propfind", Path: pathStr, Err: err}
	}

	return ms, nil
}

// stat retrieves file information
func (c *webdavClient) stat(pathStr string) (os.FileInfo, error) {
	ms, err := c.propfind(pathStr, 0)
	if err != nil {
		return nil, err
	}

	if len(ms.Responses) == 0 {
		return nil, &os.PathError{Op: "stat", Path: pathStr, Err: os.ErrNotExist}
	}

	return parseFileInfo(ms.Responses[0], pathStr)
}

// readDir lists directory contents
func (c *webdavClient) readDir(pathStr string) ([]os.FileInfo, error) {
	// Ensure path ends with / for directory listing
	if !strings.HasSuffix(pathStr, "/") {
		pathStr += "/"
	}

	ms, err := c.propfind(pathStr, 1)
	if err != nil {
		return nil, err
	}

	if len(ms.Responses) == 0 {
		return []os.FileInfo{}, nil
	}

	// First response is the directory itself, skip it
	var infos []os.FileInfo
	for i := 1; i < len(ms.Responses); i++ {
		info, err := parseFileInfo(ms.Responses[i], pathStr)
		if err != nil {
			continue // Skip entries we can't parse
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// get downloads file content
func (c *webdavClient) get(pathStr string, offset int64) (io.ReadCloser, error) {
	headers := make(map[string]string)
	if offset > 0 {
		headers["Range"] = fmt.Sprintf("bytes=%d-", offset)
	}

	resp, err := c.doRequest("GET", pathStr, nil, headers)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 206 { // 200 OK or 206 Partial Content
		resp.Body.Close()
		return nil, httpStatusToOSError(resp.StatusCode, pathStr)
	}

	return resp.Body, nil
}

// put uploads file content
func (c *webdavClient) put(pathStr string, data io.Reader) error {
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
	}

	resp, err := c.doRequest("PUT", pathStr, data, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 204 { // 201 Created or 204 No Content
		return httpStatusToOSError(resp.StatusCode, pathStr)
	}

	return nil
}

// putRange uploads partial file content
func (c *webdavClient) putRange(pathStr string, data []byte, offset int64) error {
	headers := map[string]string{
		"Content-Type":  "application/octet-stream",
		"Content-Range": fmt.Sprintf("bytes %d-%d/*", offset, offset+int64(len(data))-1),
	}

	resp, err := c.doRequest("PUT", pathStr, bytes.NewReader(data), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 204 {
		return httpStatusToOSError(resp.StatusCode, pathStr)
	}

	return nil
}

// mkcol creates a directory
func (c *webdavClient) mkcol(pathStr string) error {
	resp, err := c.doRequest("MKCOL", pathStr, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 { // 201 Created
		return httpStatusToOSError(resp.StatusCode, pathStr)
	}

	return nil
}

// delete removes a file or directory
func (c *webdavClient) delete(pathStr string) error {
	resp, err := c.doRequest("DELETE", pathStr, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 { // 204 No Content or 200 OK
		return httpStatusToOSError(resp.StatusCode, pathStr)
	}

	return nil
}

// move renames/moves a file or directory
func (c *webdavClient) move(oldPath, newPath string) error {
	destURL, err := c.buildURL(newPath)
	if err != nil {
		return err
	}

	headers := map[string]string{
		"Destination": destURL.String(),
		"Overwrite":   "F", // Don't overwrite existing files
	}

	resp, err := c.doRequest("MOVE", oldPath, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 204 { // 201 Created or 204 No Content
		return httpStatusToOSError(resp.StatusCode, oldPath)
	}

	return nil
}

// proppatch modifies properties
func (c *webdavClient) proppatch(pathStr string, modTime time.Time) error {
	headers := map[string]string{
		"Content-Type": "application/xml",
	}

	body := buildProppatchBody(modTime)
	resp, err := c.doRequest("PROPPATCH", pathStr, strings.NewReader(body), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 207 { // 207 Multi-Status
		// Some servers don't support PROPPATCH, treat as non-fatal
		return nil
	}

	return nil
}
