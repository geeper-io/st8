package main

import (
	"context"
	"fmt"
	"os"

	"github.com/geeper-io/st8/internal/cli"
)

func main() {
	code, err := cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(code)
}
