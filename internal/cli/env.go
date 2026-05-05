package cli

import (
	"github.com/guysoft/guyide-cli/internal/discover"
	"github.com/spf13/cobra"
)

func newEnvCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Resolve and print discovery environment (socket, tmux, etc.)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			info := discover.Resolve(discover.Options{
				ExplicitSocket:  g.Socket,
				ExplicitSession: g.Session,
			})
			if g.JSON {
				out.JSON(info)
				return nil
			}
			out.Header("guyide env")
			out.KeyValue("socket", info.Socket)
			out.KeyValue("source", info.SocketSource)
			out.KeyValue("tmux_session", info.TmuxSession)
			out.KeyValue("tmux_window", info.TmuxWindow)
			out.KeyValue("NVIM_IDE_SOCK", info.NvimIDESock)
			out.KeyValue("GUYIDE_PANE_ROLE", info.GuyidePaneRole)
			if info.NvimReachable {
				out.Success("nvim reachable at %s", info.Socket)
			} else {
				out.Warning("nvim not reachable")
			}
			return nil
		},
	}
}
