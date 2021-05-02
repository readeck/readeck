package admin

import (
	"github.com/readeck/readeck/internal/server"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	// API routes
	api := newAdminAPI(s)

	// API routes
	s.AddRoute("/api/admin", api)

	// Website views
	s.AddRoute("/admin", newAdminViews(api))
}
