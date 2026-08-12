package main

import (
	"fmt"
	"os"

	"github.com/ivano/gitreleaser/internal/cli"
)

func main() {
	if err := cli.New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(cli.ExitCode(err))
	}
}
