package main

import (
	"fmt"
	"os"

	app "processing/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
