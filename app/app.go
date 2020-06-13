package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/readeck/readeck/pkg/auth"
	"github.com/readeck/readeck/pkg/config"
	"github.com/readeck/readeck/pkg/db"
	"github.com/readeck/readeck/pkg/extract/fftr"
)

var rootCmd = &cobra.Command{
	Use:                "readeck",
	PersistentPreRunE:  appPersistentPreRun,
	PersistentPostRunE: appPersistentPostRunE,
}

var configPath string

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&configPath, "config", "c",
		"", "Configuration file",
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.Config.Main.LogLevel, "level", "l",
		config.Config.Main.LogLevel, "Log level",
	)
}

func appPersistentPreRun(c *cobra.Command, args []string) error {
	if err := config.LoadConfiguration(configPath); err != nil {
		return fmt.Errorf("Error loading configuration (%s)", err)
	}

	// Check if we have a configuration secret key
	if strings.TrimSpace(config.Config.Main.SecretKey) == "" {
		return fmt.Errorf("Secret key is not set (in [main] secret_key)")
	}

	// Enforce debug in dev mode
	if config.Config.Main.DevMode {
		config.Config.Main.LogLevel = "debug"
	}

	// Setup logger
	lvl, err := log.ParseLevel(config.Config.Main.LogLevel)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.WithField("log_level", lvl).Debug()
	if log.IsLevelEnabled(log.DebugLevel) {
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
		log.SetOutput(os.Stdout)
		log.SetLevel(log.TraceLevel)
	}

	// Load site-config user folders
	for _, x := range config.Config.Extractor.SiteConfig {
		addSiteConfig(x.Name, x.Src)
	}

	// Create required folders
	if err := createFolder(config.Config.Main.DataDirectory); err != nil {
		log.WithError(err).Fatal("Can't create data directory")
	}
	if config.Config.Database.Driver == "sqlite3" {
		if err := createFolder(path.Dir(config.Config.Database.Source)); err != nil {
			log.WithError(err).Fatal("Can't create database directory")
		}
	}

	// Connect to database
	if err := db.Open(
		config.Config.Database.Driver,
		config.Config.Database.Source,
	); err != nil {
		log.WithError(err).Fatal("Can't connect to database")
	}

	// Init db schema
	if err := db.Init(); err != nil {
		log.WithError(err).Fatal()
	}

	// Create the first user if needed
	count, err := db.Q().From("user").Count()
	if err != nil {
		log.WithError(err).Fatal()
	}
	if count == 0 {
		if err := auth.Users.CreateUser(&auth.User{
			Username: "admin",
			Email:    "admin@localhost",
			Password: "admin",
		}); err != nil {
			log.WithError(err).Fatal()
		}
	}

	return nil
}

func appPersistentPostRunE(cmd *cobra.Command, args []string) error {
	return cleanup()
}

func cleanup() error {
	return db.Close()
}

func createFolder(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(name, 0750); err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		return fmt.Errorf("'%s' is not a directory", name)
	}

	return nil
}

func addSiteConfig(name, src string) {
	stat, err := os.Stat(src)
	l := log.WithField("path", src)
	if err != nil {
		l.WithError(err).Warn("can't open site-config folder")
		return
	}
	if !stat.IsDir() {
		l.Warn("site-config is not a folder")
		return
	}

	f := &fftr.ConfigFolder{
		FileSystem: http.Dir(src),
		Name:       name,
		Dir:        "./",
	}

	fftr.DefaultConfigurationFolders = append(fftr.ConfigFolderList{f}, fftr.DefaultConfigurationFolders...)
}

// Run starts the application
func Run() error {
	go func() {
		sigchan := make(chan os.Signal, 10)
		signal.Notify(sigchan,
			os.Interrupt, os.Kill,
			syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT,
			syscall.SIGKILL, syscall.SIGHUP,
		)
		<-sigchan
		println("Bye!")

		cleanup()
		os.Exit(0)
	}()

	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatal()
		os.Exit(1)
	}

	return rootCmd.Execute()
}
