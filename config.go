package webdavfs

import (
	"net/http"
	"time"
)

// Config holds the configuration for connecting to a WebDAV server.
type Config struct {
	// URL is the base URL of the WebDAV server (e.g., "https://webdav.example.com/remote.php/dav/files/user/")
	URL string

	// Username for HTTP authentication (optional)
	Username string

	// Password for HTTP authentication (optional)
	Password string

	// BearerToken for Bearer token authentication (optional, mutually exclusive with Username/Password)
	BearerToken string

	// HTTPClient allows customization of the HTTP client (optional)
	// If nil, a default client with reasonable timeouts will be used
	HTTPClient *http.Client

	// Timeout for HTTP requests (default: 30 seconds)
	Timeout time.Duration

	// TempDir specifies the temporary directory path on the WebDAV server (optional)
	// If empty, defaults to "/tmp"
	TempDir string
}

// setDefaults sets default values for the configuration
func (c *Config) setDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{
			Timeout: c.Timeout,
		}
	}

	if c.TempDir == "" {
		c.TempDir = "/tmp"
	}
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if c.URL == "" {
		return &ConfigError{Field: "URL", Reason: "URL is required"}
	}

	// Check for mutually exclusive auth methods
	if c.BearerToken != "" && (c.Username != "" || c.Password != "") {
		return &ConfigError{
			Field:  "Authentication",
			Reason: "BearerToken and Username/Password are mutually exclusive",
		}
	}

	return nil
}
