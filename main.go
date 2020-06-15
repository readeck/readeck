package main

import (
	"fmt"
	"os"

	"github.com/readeck/readeck/internal/app"
)

//go:generate go run -tags=!build tools/generate_assets.go

func main() {
	if err := app.Run(); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}
