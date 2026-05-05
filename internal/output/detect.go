package output

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Options controls writer construction.
type Options struct {
	JSON     bool   // --json flag
	NoColor  bool   // --no-color flag
	Out      io.Writer
	IsTTY    *bool  // override for tests; nil → autodetect
	Env      func(string) string // override for tests; nil → os.Getenv
}

// New constructs a Writer based on options, env, and TTY state.
//
// Mode precedence (first match wins):
//  1. --json flag → ModeMachine
//  2. --no-color flag → ModeHumanPlain
//  3. NO_COLOR=1 → ModeHumanPlain
//  4. CI=true → ModeHumanPlain
//  5. !isatty(stdout) → ModeMachine (auto-JSON when piped)
//  6. else → ModeHumanStyled
func New(opts Options) Writer {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	getenv := opts.Env
	if getenv == nil {
		getenv = os.Getenv
	}
	isTTY := false
	if opts.IsTTY != nil {
		isTTY = *opts.IsTTY
	} else if f, ok := opts.Out.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	switch {
	case opts.JSON:
		return newMachineWriter(opts.Out)
	case opts.NoColor:
		return newHumanWriter(opts.Out, newPlainTheme(), ModeHumanPlain)
	case getenv("NO_COLOR") != "":
		return newHumanWriter(opts.Out, newPlainTheme(), ModeHumanPlain)
	case getenv("CI") == "true":
		return newHumanWriter(opts.Out, newPlainTheme(), ModeHumanPlain)
	case !isTTY:
		return newMachineWriter(opts.Out)
	default:
		return newHumanWriter(opts.Out, newTheme(), ModeHumanStyled)
	}
}
