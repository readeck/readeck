package cookbook

import (
	"codeberg.org/readeck/readeck/internal/server"
)

// SetupRoutes mounts the routes for the cookbook domain.
func SetupRoutes(s *server.Server) {
	// API routes
	api := newCookbookAPI(s)
	s.AddRoute("/api/cookbook", api)
}
