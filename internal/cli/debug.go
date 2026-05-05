package cli

import (
	"github.com/spf13/cobra"
)

func newDebugCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "debug",
		Short: "Drive nvim-dap debug sessions",
	}
	for _, sub := range []struct {
		use, short string
	}{
		{"start", "Start a debug session by config name"},
		{"stop", "Stop the active debug session"},
		{"state", "Print current session state (frames, vars, location)"},
		{"step", "Step over the next statement"},
		{"continue", "Continue execution"},
	} {
		s := sub
		c.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			RunE: func(cmd *cobra.Command, _ []string) error {
				out := writerFor(g)
				out.Info("debug %s: not yet implemented (Phase 1 stub)", s.use)
				return nil
			},
		})
	}

	br := &cobra.Command{Use: "break", Short: "Manage breakpoints"}
	for _, sub := range []struct {
		use, short string
	}{
		{"set", "Set a breakpoint"},
		{"list", "List breakpoints"},
		{"clear", "Clear breakpoints"},
	} {
		s := sub
		br.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			RunE: func(cmd *cobra.Command, _ []string) error {
				out := writerFor(g)
				out.Info("debug break %s: not yet implemented (Phase 1 stub)", s.use)
				return nil
			},
		})
	}
	c.AddCommand(br)
	return c
}
