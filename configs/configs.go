package configs

import (
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/pelletier/go-toml"
)

var keyChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890&~@#$%")

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
	SignKey       string `toml:"sign_key"`
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

// WriteConfig writes configuration to a file.
func WriteConfig(filename string) error {
	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	enc := toml.NewEncoder(fd).
		ArraysWithOneElementPerLine(true).
		Indentation("  ").
		Order(toml.OrderPreserve)

	if err = enc.Encode(Config); err != nil {
		defer fd.Close()
		return err
	}

	return fd.Close()
}

// MakeKey returns a random key
func MakeKey(length int) string {
	rand.Seed(time.Now().UnixNano())

	b := make([]rune, length)
	for i := range b {
		b[i] = keyChars[rand.Intn(len(keyChars))]
	}
	return string(b)
}
