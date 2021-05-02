package cookbook

import (
	"github.com/readeck/readeck/internal/server"
)

// SetupRoutes mounts the routes for the cookbook domain.
func SetupRoutes(s *server.Server) {
	// API routes
	api := newCookbookAPI(s)
	s.AddRoute("/api/cookbook", api)

	// Views
	s.AddRoute("/cookbook", newCookbookViews(api))
}
