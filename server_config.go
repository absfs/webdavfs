package webdavfs

import (
	"net/http"

	"golang.org/x/net/webdav"
)

// ServerConfig holds configuration for the WebDAV server.
type ServerConfig struct {
	// Prefix is the URL path prefix to strip from WebDAV resource paths.
	// Example: "/webdav" means requests to "/webdav/file.txt" access "/file.txt"
	Prefix string

	// Auth is an optional authentication handler.
	// If nil, no authentication is required.
	Auth AuthProvider

	// Logger is an optional request logger function.
	// If nil, requests are not logged.
	Logger func(r *http.Request, err error)

	// LockSystem configures WebDAV locking behavior.
	// If nil, a MemLS (in-memory lock system) is used.
	LockSystem webdav.LockSystem
}

// AuthProvider defines the interface for authentication.
type AuthProvider interface {
	// Authenticate checks the request and returns true if authenticated.
	// It may modify the response (e.g., send 401) if authentication fails.
	Authenticate(w http.ResponseWriter, r *http.Request) bool
}

// BasicAuth implements HTTP Basic authentication.
type BasicAuth struct {
	// Realm is the authentication realm shown to the user.
	Realm string

	// Validator validates username/password combinations.
	// Returns true if valid, false otherwise.
	Validator func(username, password string) bool
}

// Authenticate implements AuthProvider for HTTP Basic authentication.
func (b *BasicAuth) Authenticate(w http.ResponseWriter, r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok || b.Validator == nil || !b.Validator(username, password) {
		realm := b.Realm
		if realm == "" {
			realm = "WebDAV"
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// BearerAuth implements HTTP Bearer token authentication.
type BearerAuth struct {
	// Realm is the authentication realm shown to the user.
	Realm string

	// Validator validates the bearer token.
	// Returns true if valid, false otherwise.
	Validator func(token string) bool
}

// Authenticate implements AuthProvider for HTTP Bearer authentication.
func (b *BearerAuth) Authenticate(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) < len(prefix) || auth[:len(prefix)] != prefix {
		b.sendChallenge(w)
		return false
	}

	token := auth[len(prefix):]
	if b.Validator == nil || !b.Validator(token) {
		b.sendChallenge(w)
		return false
	}
	return true
}

func (b *BearerAuth) sendChallenge(w http.ResponseWriter) {
	realm := b.Realm
	if realm == "" {
		realm = "WebDAV"
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="`+realm+`"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// MultiAuth combines multiple authentication providers.
// Authentication succeeds if any provider succeeds.
type MultiAuth struct {
	Providers []AuthProvider
}

// Authenticate implements AuthProvider by trying each provider in order.
// Returns true if any provider succeeds.
func (m *MultiAuth) Authenticate(w http.ResponseWriter, r *http.Request) bool {
	// Create a response recorder to capture failed auth attempts
	// so we don't send multiple 401 responses
	for _, p := range m.Providers {
		recorder := &authRecorder{ResponseWriter: w}
		if p.Authenticate(recorder, r) {
			return true
		}
	}

	// All providers failed, send the last challenge
	if len(m.Providers) > 0 {
		m.Providers[len(m.Providers)-1].Authenticate(w, r)
	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
	return false
}

// authRecorder captures authentication failures without writing to the response
type authRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (r *authRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.written = true
	// Don't actually write - we're just recording
}

func (r *authRecorder) Write(b []byte) (int, error) {
	r.written = true
	// Don't actually write - we're just recording
	return len(b), nil
}
