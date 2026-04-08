package main

import (
	"context"
	"fmt"
	"os"

	"neo-code/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "neocode: %v\n", err)
		os.Exit(1)
	}
}
