package config

import (
	"os"
	"runtime"

	"github.com/pelletier/go-toml"
)

// Because we don't need viper's mess for just storing configuration from
// a source.
type config struct {
	Main      configMain      `toml:"main"`
	Server    configServer    `toml:"server"`
	Database  configDB        `toml:"database"`
	Extractor configExtractor `toml:"extractor"`
}

type configMain struct {
	LogLevel      string `toml:"log_level"`
	DevMode       bool   `toml:"dev_mode"`
	SecretKey     string `toml:"secret_key"`
	DataDirectory string `toml:"data_directory"`
}

type configServer struct {
	Host string
	Port int
}

type configDB struct {
	Driver string `toml:"driver"`
	Source string `toml:"source"`
}

type configExtractor struct {
	NumWorkers int                `toml:"workers"`
	SiteConfig []configSiteConfig `toml:"site_config"`
}

type configSiteConfig struct {
	Name string `toml:"name"`
	Src  string `toml:"src"`
}

// Config holds the configuration data from configuration files
// or flags.
//
// This variable sets some default values that might be overwritten
// by a configuration file.
var Config = config{
	Main: configMain{
		LogLevel:      "info",
		DevMode:       false,
		DataDirectory: "data",
	},
	Server: configServer{
		Host: "127.0.0.1",
		Port: 5000,
	},
	Database: configDB{
		Driver: "sqlite3",
		Source: "data/db.sqlite3",
	},
	Extractor: configExtractor{
		NumWorkers: runtime.NumCPU(),
	},
}

// LoadConfiguration loads the configuration file.
func LoadConfiguration(configPath string) error {
	if configPath == "" {
		return nil
	}

	fd, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	dec := toml.NewDecoder(fd)
	if err := dec.Decode(&Config); err != nil {
		return err
	}

	return nil
}
