package admin

import (
	"codeberg.org/readeck/readeck/internal/server"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	// API routes
	api := newAdminAPI(s)

	// API routes
	s.AddRoute("/api/admin", api)
}
