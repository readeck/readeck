package fftr

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed site-config site-config/**/*
var assets embed.FS

// siteConfigFS returns the given site-config subfolder as an fs.FS instance.
func siteConfigFS(name string) fs.FS {
	sub, err := fs.Sub(assets, fmt.Sprintf("site-config/%s", name))
	if err != nil {
		panic(err)
	}
	return sub
}
