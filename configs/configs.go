package configs

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"os"
	"runtime"
	"time"

	"github.com/komkom/toml"
)

var (
	version      string = "dev"
	buildTimeStr string
	buildTime    time.Time
	startTime    time.Time = time.Now().UTC()

	cookieHk []byte
	cookieBk []byte
	csrfKey  []byte
	jwtSk    ed25519.PrivateKey
	jwtPk    ed25519.PublicKey
)

func init() {
	buildTime, _ = time.Parse("2006-01-02T15:04:05", buildTimeStr)
}

// Because we don't need viper's mess for just storing configuration from
// a source.
type config struct {
	Main      configMain      `json:"main"`
	Server    configServer    `json:"server"`
	Database  configDB        `json:"database"`
	Extractor configExtractor `json:"extractor"`
}

type configMain struct {
	LogLevel      string `json:"log_level"`
	DevMode       bool   `json:"dev_mode"`
	SecretKey     string `json:"secret_key"`
	DataDirectory string `json:"data_directory"`
}

type configServer struct {
	Host    string        `json:"host"`
	Port    int           `json:"port"`
	Prefix  string        `json:"prefix"`
	Session configSession `json:"session"`
}

type configDB struct {
	Source string `json:"source"`
}

type configSession struct {
	CookieName string `json:"cookie_name"`
	MaxAge     int    `json:"max_age"` // in minutes
}

type configExtractor struct {
	NumWorkers int                `json:"workers"`
	SiteConfig []configSiteConfig `json:"site_config"`
}

type configSiteConfig struct {
	Name string `json:"name"`
	Src  string `json:"src"`
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
		Session: configSession{
			CookieName: "sxid",
			MaxAge:     86400 * 30,
		},
	},
	Database: configDB{
		Source: "sqlite3:data/db.sqlite3",
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

	dec := json.NewDecoder(toml.New(fd))
	if err := dec.Decode(&Config); err != nil {
		return err
	}

	loadKeys(Config.Main.SecretKey)

	return nil
}

// loadKeys prepares all the keys derivated from the configuration's
// secret key.
func loadKeys(sk string) {
	// Pad the secret key with its own checksum to have a
	// long enough byte list.
	h := sha512.Sum512([]byte(sk))
	seed := append([]byte(sk), h[:]...)

	hashMsg := func(k []byte, m string) []byte {
		mac := hmac.New(sha256.New, k[:])
		mac.Write([]byte(m))
		return mac.Sum(nil)
	}

	cookieHk = hashMsg(seed, "cookie-hash-key")
	cookieBk = hashMsg(seed, "cookie-block-key")
	csrfKey = hashMsg(seed, "csrf-key")

	jwtSk = ed25519.NewKeyFromSeed(seed[32:64])
	jwtPk = jwtSk.Public().(ed25519.PublicKey)
}

// CookieHashKey returns the key used by session cookies
func CookieHashKey() []byte {
	return cookieHk
}

// CookieBlockKey returns the key used by session cookies
func CookieBlockKey() []byte {
	return cookieBk
}

// CsrfKey returns the key used by CSRF protection
func CsrfKey() []byte {
	return csrfKey
}

// JwtSk returns the private key for JWT handlers
func JwtSk() ed25519.PrivateKey {
	return jwtSk
}

// JwtPk returns the public key for JWT handlers
func JwtPk() ed25519.PublicKey {
	return jwtPk
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
