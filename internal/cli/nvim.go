package cli

import (
	"github.com/spf13/cobra"
)

func newNvimCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "nvim",
		Short: "Interact with the running nvim instance via msgpack-rpc",
	}
	c.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Report nvim reachability and basic info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			out.Info("nvim status: not yet implemented (Phase 1 stub)")
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "exec [vimscript]",
		Short: "Execute a vimscript fragment via nvim_command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			out.Info("nvim exec: not yet implemented (Phase 1 stub)")
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "eval [expr]",
		Short: "Evaluate a vimscript expression via nvim_eval",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			out.Info("nvim eval: not yet implemented (Phase 1 stub)")
			return nil
		},
	})
	return c
}
