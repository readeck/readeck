package configs

import (
	"crypto/ed25519"
	"crypto/hmac"
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
	CookieHk []byte
	CookieBk []byte
	CsrfKey  []byte
	JwtSk    ed25519.PrivateKey
	JwtPk    ed25519.PublicKey
}

func newConfigKeys(sk string) configKeys {
	seed := sha512.Sum512([]byte(sk))

	hash := func(k [64]byte, m string) []byte {
		mac := hmac.New(sha256.New, k[:])
		mac.Write([]byte(m))
		return mac.Sum(nil)
	}

	cookieHk := hash(seed, "cookie-hash-key")
	cookieBk := hash(seed, "cookie-block-key")
	csrfKey := hash(seed, "csrf-key")

	jwtSK := ed25519.NewKeyFromSeed(seed[32:64])

	return configKeys{
		CookieHk: cookieBk,
		CookieBk: cookieHk,
		CsrfKey:  csrfKey,
		JwtSk:    jwtSK,
		JwtPk:    jwtSK.Public().(ed25519.PublicKey),
	}
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
