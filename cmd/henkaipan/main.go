// Command henkaipan is the standalone CLI for the HenKaiPan REST API.
//
// It is the client-agnostic companion to the henkaipan-action GitHub Action:
// same X-API-Key auth, same endpoints, but invocable from any shell, script, or
// CI step that can run a static binary.
package main

import (
	"fmt"
	"os"

	"github.com/dyallab/henkaipan-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}