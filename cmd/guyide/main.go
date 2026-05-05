// Command guyide is the AI-friendly bridge to nvim, tmux, and DAP.
package main

import (
	"fmt"
	"os"

	"github.com/guysoft/guyide-cli/internal/cli"
)

func main() {
	root := cli.NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "guyide:", err)
		os.Exit(1)
	}
}
