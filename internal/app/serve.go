package app

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/assets"
	"codeberg.org/readeck/readeck/internal/auth/signin"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/cookbook"
	"codeberg.org/readeck/readeck/internal/dashboard"
	"codeberg.org/readeck/readeck/internal/profile"
	"codeberg.org/readeck/readeck/internal/server"
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.PersistentFlags().StringVarP(
		&configs.Config.Server.Host, "host", "H",
		configs.Config.Server.Host, "server host")
	serveCmd.PersistentFlags().IntVarP(
		&configs.Config.Server.Port, "port", "p",
		configs.Config.Server.Port, "server host")
}

var serveCmd = &cobra.Command{
	Use:  "serve",
	RunE: runServe,
}

func runServe(_ *cobra.Command, _ []string) error {
	if !configs.Config.Main.DevMode && len(configs.Config.Server.AllowedHosts) == 0 {
		return fmt.Errorf("The server.allowed_hosts setting is not set")
	}

	s := server.New(configs.Config.Server.Prefix)

	// Static asserts
	assets.SetupRoutes(s)

	// Auth routes
	signin.SetupRoutes(s)

	// Dashboard routes
	dashboard.SetupRoutes(s)

	// Bookmark routes
	// - /bookmarks/*
	// - /bm/* (for bookmark media files)
	bookmarks.SetupRoutes(s)

	// User routes
	profile.SetupRoutes(s)

	// Only in dev mode
	if configs.Config.Main.DevMode {
		// Cookbook routes
		cookbook.SetupRoutes(s)
	}

	log.WithField("url", fmt.Sprintf("http://%s:%d%s",
		configs.Config.Server.Host, configs.Config.Server.Port, s.BasePath),
	).Info("Starting server")
	return s.ListenAndServe()
}
