package app

import (
	"fmt"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/readeck/readeck/pkg/bookmarks"
	"github.com/readeck/readeck/pkg/config"
	"github.com/readeck/readeck/pkg/cookbook"
	"github.com/readeck/readeck/pkg/server"
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.PersistentFlags().StringVarP(
		&config.Config.Server.Host, "host", "H",
		config.Config.Server.Host, "server host")
	serveCmd.PersistentFlags().IntVarP(
		&config.Config.Server.Port, "port", "p",
		config.Config.Server.Port, "server host")
}

var serveCmd = &cobra.Command{
	Use: "serve",
	Run: runServe,
}

func runServe(c *cobra.Command, args []string) {
	s := server.New()

	s.Router.Route("/", func(r chi.Router) {
		r.Mount("/", s.BaseRoutes())

		r.Route("/api", func(r chi.Router) {
			r.Mount("/", s.AuthRoutes())
			r.Mount("/sys", s.SysRoutes())

			r.Mount("/bookmarks", bookmarks.Routes(s))

			if config.Config.Main.DevMode {
				r.Mount("/cookbook", cookbook.Routes(s))
			}
		})
	})

	log.WithField("url", fmt.Sprintf("http://%s:%d/",
		config.Config.Server.Host, config.Config.Server.Port),
	).Info("Starting server")
	s.ListenAndServe()
}
