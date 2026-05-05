package cli

import (
	"github.com/spf13/cobra"
)

func newLayoutCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "layout",
		Short: "Inspect the tmux-ide layout",
	}
	c.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Describe panes and roles in the current layout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			out.Info("layout info: not yet implemented (Phase 1 stub)")
			return nil
		},
	})
	return c
}
