package migrations

import "embed"

// Files contains all the static files needed by the app
//
//go:embed *
var Files embed.FS
