// Package cli wires the cobra command tree.
package cli

import (
	"github.com/guysoft/guyide-cli/internal/output"
	"github.com/spf13/cobra"
)

// Globals are shared across all commands. Populated by persistent flags.
type Globals struct {
	JSON    bool
	NoColor bool
	Verbose bool
	Socket  string
	Session string
	Timeout string
}

// NewRoot builds the root command and attaches all subcommands.
func NewRoot() *cobra.Command {
	g := &Globals{}
	root := &cobra.Command{
		Use:   "guyide",
		Short: "AI-friendly bridge to nvim, tmux, and dap",
		Long: "guyide is a CLI bridge that lets AI agents (and humans) drive\n" +
			"nvim, tmux, and the Debug Adapter Protocol from a single binary.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVar(&g.JSON, "json", false, "machine-readable JSON output")
	root.PersistentFlags().BoolVar(&g.NoColor, "no-color", false, "disable ANSI colors and emoji")
	root.PersistentFlags().BoolVarP(&g.Verbose, "verbose", "v", false, "verbose logging to stderr")
	root.PersistentFlags().StringVar(&g.Socket, "socket", "", "explicit nvim msgpack socket path")
	root.PersistentFlags().StringVar(&g.Session, "session", "", "tmux session name (default: current)")
	root.PersistentFlags().StringVar(&g.Timeout, "timeout", "30s", "default timeout for blocking operations")

	root.AddCommand(newVersionCmd(g))
	root.AddCommand(newEnvCmd(g))
	root.AddCommand(newDoctorCmd(g))
	root.AddCommand(newNvimCmd(g))
	root.AddCommand(newTmuxCmd(g))
	root.AddCommand(newDebugCmd(g))
	root.AddCommand(newLayoutCmd(g))
	root.AddCommand(newInstallCmd(g))
	root.AddCommand(newUninstallCmd(g))
	root.AddCommand(newListBackupsCmd(g))

	return root
}

// writerFor builds an output.Writer from globals; commands must use this
// instead of constructing writers themselves.
func writerFor(g *Globals) output.Writer {
	return output.New(output.Options{JSON: g.JSON, NoColor: g.NoColor})
}
