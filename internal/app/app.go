package app

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/mattn/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/extract/fftr"
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
		&configs.Config.Main.LogLevel, "level", "l",
		configs.Config.Main.LogLevel, "Log level",
	)
}

func appPersistentPreRun(_ *cobra.Command, _ []string) error {
	if configPath == "" {
		configPath = "config.toml"
	}
	if err := createConfigFile(configPath); err != nil {
		return err
	}

	if err := configs.LoadConfiguration(configPath); err != nil {
		return fmt.Errorf("error loading configuration (%s)", err)
	}

	if updateConfig() {
		if err := configs.WriteConfig(configPath); err != nil {
			return err
		}
	}

	// Enforce debug in dev mode
	if configs.Config.Main.DevMode {
		configs.Config.Main.LogLevel = "debug"
	}

	// Setup logger
	lvl, err := log.ParseLevel(configs.Config.Main.LogLevel)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.WithField("log_level", lvl).Debug()
	if configs.Config.Main.DevMode {
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
		log.SetOutput(colorable.NewColorableStdout())
		log.SetLevel(log.TraceLevel)
	}

	// Load site-config user folders
	for _, x := range configs.Config.Extractor.SiteConfig {
		addSiteConfig(x.Name, x.Src)
	}

	dsn, err := url.Parse(configs.Config.Database.Source)
	if err != nil {
		log.WithError(err).Fatal("Can't read database source value")
	}

	// Create required folders
	if err := createFolder(configs.Config.Main.DataDirectory); err != nil {
		log.WithError(err).Fatal("Can't create data directory")
	}

	// SQLite data path
	if dsn.Scheme == "sqlite3" {
		if err := createFolder(path.Dir(dsn.Opaque)); err != nil {
			log.WithError(err).Fatal("Can't create database directory")
		}
	}

	// Connect to database
	if err := db.Open(configs.Config.Database.Source); err != nil {
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
		if err := users.Users.Create(&users.User{
			Username: "admin",
			Email:    "admin@localhost",
			Password: "admin",
			Group:    "admin",
		}); err != nil {
			log.WithError(err).Fatal()
		}
	}

	return nil
}

func appPersistentPostRunE(_ *cobra.Command, _ []string) error {
	return cleanup()
}

func cleanup() error {
	return db.Close()
}

func createConfigFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		if err = fd.Close(); err != nil {
			return err
		}
	}
	return nil
}

func updateConfig() bool {
	if configs.Config.Main.SecretKey == "" {
		configs.Config.Main.SecretKey = configs.GenerateKey(48, 64)
		return true
	}

	return false
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
		FS:   os.DirFS(src),
		Name: name,
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

	return nil
}
