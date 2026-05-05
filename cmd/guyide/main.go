// Command guyide is the AI-friendly bridge to nvim, tmux, and DAP.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/guysoft/guyide-cli/internal/cli"
)

// coder is implemented by errors that carry a desired process exit code.
type coder interface {
	Error() string
	Code() int
}

func main() {
	root := cli.NewRoot()
	err := root.Execute()
	if err == nil {
		return
	}
	var ce coder
	if errors.As(err, &ce) {
		os.Exit(ce.Code())
	}
	fmt.Fprintln(os.Stderr, "guyide:", err)
	os.Exit(1)
}
