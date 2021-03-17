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

	keyChars = [][2]rune{
		{33, 124},                                      // latin set (symbols, numbers, alphabet)
		{161, 187}, {191, 214}, {216, 246}, {248, 255}, // latin supplement
		{128512, 128584}, // emojis
	}

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
	Host    string
	Port    int
	Session configSession `json:"session"`
}

type configDB struct {
	Driver string `json:"driver"`
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

	loadKeys(Config.Main.SecretKey)

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

// GenerateKey returns a random key
func GenerateKey(minLen, maxLen int) string {
	if minLen >= maxLen {
		panic("maxLen must be greater then minLen")
	}
	rand.Seed(time.Now().UnixNano())

	runes := []rune{}
	for _, table := range keyChars {
		for i := table[0]; i <= table[1]; i++ {
			if i == 34 || i == 92 { // exclude " and \
				continue
			}
			runes = append(runes, i)
		}
	}

	l := rand.Intn(maxLen-minLen) + minLen
	b := make([]rune, l)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
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
