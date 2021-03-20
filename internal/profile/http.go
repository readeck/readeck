package profile

import (
	"codeberg.org/readeck/readeck/internal/server"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	// API routes
	api := newProfileAPI(s)
	s.AddRoute("/api/profile", api)

	// Website views
	s.AddRoute("/profile", newProfileViews(api))
}
