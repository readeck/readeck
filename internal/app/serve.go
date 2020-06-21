package app

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/bookmarks"
	"github.com/readeck/readeck/internal/cookbook"
	"github.com/readeck/readeck/internal/server"
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
	Use: "serve",
	Run: runServe,
}

func runServe(c *cobra.Command, args []string) {
	s := server.New("/")

	// Base routes (assets, auth, system info...)
	s.SetupRoutes()

	// Bookmark routes
	// - /api/bookmarks/*
	// - /bm/* (for bookmark media files)
	bookmarks.SetupRoutes(s)

	// Only in dev mode
	if configs.Config.Main.DevMode {
		// Cookbook routes
		cookbook.SetupRoutes(s)
	}

	log.WithField("url", fmt.Sprintf("http://%s:%d/",
		configs.Config.Server.Host, configs.Config.Server.Port),
	).Info("Starting server")
	s.ListenAndServe()
}
