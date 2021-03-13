package configs

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/pelletier/go-toml"
)

var (
	version      string = "dev"
	buildTimeStr string
	buildTime    time.Time
	startTime    time.Time = time.Now().UTC()

	keyChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890&~@#$%")
)

func init() {
	buildTime, _ = time.Parse("2006-01-02T15:04:05", buildTimeStr)
}

// Because we don't need viper's mess for just storing configuration from
// a source.
type config struct {
	Main      configMain      `toml:"main"`
	Server    configServer    `toml:"server"`
	Database  configDB        `toml:"database"`
	Extractor configExtractor `toml:"extractor"`
	Keys      configKeys      `toml:"-"`
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

type configKeys struct {
	CookieSk [32]byte
	CookieEk [32]byte
	CsrfKey  []byte
	JwtSk    ed25519.PrivateKey
	JwtPk    ed25519.PublicKey
}

func newConfigKeys(sk string) configKeys {
	k1 := []byte(sk)
	k2 := sha512.Sum512(k1)

	res := configKeys{
		CookieSk: sha512.Sum512_256(k1),
		CookieEk: sha256.Sum256(k1),
		CsrfKey:  k2[40:64],
		JwtSk:    ed25519.NewKeyFromSeed(k2[8:40]),
	}
	res.JwtPk = res.JwtSk.Public().(ed25519.PublicKey)

	return res
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

	Config.Keys = newConfigKeys(Config.Main.SecretKey)

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

// Version returns the current readeck version
func Version() string {
	return version
}

// BuildTime returns the build time or, if empty, the time
// when the application started
func BuildTime() time.Time {
	if buildTime.IsZero() {
		return startTime
	}
	return buildTime
}
