package cli

import (
	"github.com/spf13/cobra"
)

func newTmuxCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "tmux",
		Short: "Tmux pane operations",
	}
	c.AddCommand(&cobra.Command{
		Use:   "panes",
		Short: "List panes in the current window",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			out.Info("tmux panes: not yet implemented (Phase 1 stub)")
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "send [pane] [keys...]",
		Short: "Send keys to a pane",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			out.Info("tmux send: not yet implemented (Phase 1 stub)")
			return nil
		},
	})

	watch := &cobra.Command{
		Use:   "watch",
		Short: "Watch a pane until a regex matches or timeout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			out.Info("tmux watch: not yet implemented (Phase 1 stub)")
			return nil
		},
	}
	watch.Flags().String("pane", "", "pane id to watch (default: active)")
	watch.Flags().String("until", "", "regex to match (required)")
	watch.Flags().String("timeout", "30s", "timeout duration")
	c.AddCommand(watch)

	return c
}
