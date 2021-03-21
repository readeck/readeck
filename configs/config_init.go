package configs

import (
	"math/rand"
	"os"
	"text/template"
	"time"
)

const initialConfiguration = `
[main]
log_level = "{{ .Main.LogLevel }}"
secret_key = "{{ .Main.SecretKey }}"
data_directory = "{{ .Main.DataDirectory }}"

[server]
host = "{{ .Server.Host }}"
port = {{ .Server.Port }}

[database]
source = "{{ .Database.Source }}"
`

var keyChars = [][2]rune{
	{33, 124},                                      // latin set (symbols, numbers, alphabet)
	{161, 187}, {191, 214}, {216, 246}, {248, 255}, // latin supplement
	{128512, 128584}, // emojis
}

// WriteConfig writes configuration to a file.
func WriteConfig(filename string) error {
	tmpl, err := template.New("cfg").Parse(initialConfiguration)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if err = tmpl.Execute(fd, Config); err != nil {
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
