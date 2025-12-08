package webdavfs

import (
	"net/http"

	"github.com/absfs/absfs"
	"golang.org/x/net/webdav"
)

// Server provides a WebDAV server backed by any absfs.FileSystem.
// It implements http.Handler and can be used directly with http.ListenAndServe
// or integrated into existing HTTP routers.
type Server struct {
	handler *webdav.Handler
	auth    AuthProvider
}

// NewServer creates a new WebDAV server for the given filesystem.
//
// Example usage:
//
//	fs, _ := memfs.NewFS()
//	server := webdavfs.NewServer(fs, &webdavfs.ServerConfig{
//	    Prefix: "/webdav",
//	    Auth: &webdavfs.BasicAuth{
//	        Realm: "My Server",
//	        Validator: func(user, pass string) bool {
//	            return user == "admin" && pass == "secret"
//	        },
//	    },
//	})
//	http.ListenAndServe(":8080", server)
func NewServer(fs absfs.FileSystem, config *ServerConfig) *Server {
	if config == nil {
		config = &ServerConfig{}
	}

	lockSystem := config.LockSystem
	if lockSystem == nil {
		lockSystem = webdav.NewMemLS()
	}

	handler := &webdav.Handler{
		Prefix:     config.Prefix,
		FileSystem: NewServerFileSystem(fs),
		LockSystem: lockSystem,
		Logger:     config.Logger,
	}

	return &Server{
		handler: handler,
		auth:    config.Auth,
	}
}

// ServeHTTP implements http.Handler.
// It handles WebDAV protocol methods (PROPFIND, PROPPATCH, MKCOL, COPY, MOVE, LOCK, UNLOCK)
// as well as standard HTTP methods (GET, PUT, DELETE, OPTIONS).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check authentication if configured
	if s.auth != nil {
		if !s.auth.Authenticate(w, r) {
			return
		}
	}

	s.handler.ServeHTTP(w, r)
}

// Handler returns the underlying http.Handler.
// Useful for wrapping with middleware.
func (s *Server) Handler() http.Handler {
	return s
}

// SetPrefix updates the URL path prefix.
func (s *Server) SetPrefix(prefix string) {
	s.handler.Prefix = prefix
}

// SetLogger updates the request logger.
func (s *Server) SetLogger(logger func(r *http.Request, err error)) {
	s.handler.Logger = logger
}

// SetAuth updates the authentication provider.
func (s *Server) SetAuth(auth AuthProvider) {
	s.auth = auth
}
