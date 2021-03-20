package main

import (
	"fmt"
	"os"

	"codeberg.org/readeck/readeck/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}
