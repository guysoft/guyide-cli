package cli

import (
	"errors"
	"fmt"

	gnvim "github.com/guysoft/guyide-cli/internal/nvim"
	"github.com/guysoft/guyide-cli/internal/discover"
	"github.com/spf13/cobra"
)

// resolveClient connects to nvim using the discovery cascade. Caller must
// Close() on success.
func resolveClient(g *Globals) (*gnvim.Client, string, error) {
	env := discover.Resolve(discover.Options{
		ExplicitSocket:  g.Socket,
		ExplicitSession: g.Session,
	})
	if env.Socket == "" {
		return nil, "", errors.New("no nvim socket discovered (set --socket or NVIM_IDE_SOCK)")
	}
	c, err := gnvim.Dial(env.Socket)
	if err != nil {
		return nil, env.Socket, err
	}
	return c, env.Socket, nil
}

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
			cli, socket, err := resolveClient(g)
			if err != nil {
				out.Error("%s", err)
				return err
			}
			defer cli.Close()
			ch, ver, err := cli.APIInfo()
			if err != nil {
				out.Error("%s", err)
				return err
			}
			if g.JSON {
				out.JSON(map[string]any{
					"schema":         "guyide/v1",
					"level":          "info",
					"socket":         socket,
					"channel_id":     ch,
					"api_version":    ver,
					"reachable":      true,
				})
				return nil
			}
			out.Header("guyide nvim status")
			out.KeyValue("socket", socket)
			out.KeyValue("channel_id", fmt.Sprintf("%d", ch))
			if v, ok := ver["major"]; ok {
				out.KeyValue("api_major", fmt.Sprintf("%v", v))
			}
			out.Success("nvim reachable")
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "exec [vimscript]",
		Short: "Execute a vimscript fragment via nvim_command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			cli, socket, err := resolveClient(g)
			if err != nil {
				out.Error("%s", err)
				return err
			}
			defer cli.Close()
			vimcmd := joinArgs(args)
			if err := cli.Command(vimcmd); err != nil {
				out.Error("%s", err)
				return err
			}
			// Fetch context so the caller can verify the right nvim was targeted.
			buf, cwd := nvimContext(cli)
			if g.JSON {
				out.JSON(map[string]any{
					"schema":  "guyide/v1",
					"level":   "success",
					"command": vimcmd,
					"ok":      true,
					"socket":  socket,
					"buffer":  buf,
					"cwd":     cwd,
				})
				return nil
			}
			out.Success("ran: %s", vimcmd)
			out.KeyValue("socket", socket)
			out.KeyValue("buffer", buf)
			out.KeyValue("cwd", cwd)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "eval [expr]",
		Short: "Evaluate a vimscript expression via nvim_eval",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			cli, socket, err := resolveClient(g)
			if err != nil {
				out.Error("%s", err)
				return err
			}
			defer cli.Close()
			expr := joinArgs(args)
			val, err := cli.Eval(expr)
			if err != nil {
				out.Error("%s", err)
				return err
			}
			buf, cwd := nvimContext(cli)
			if g.JSON {
				out.JSON(map[string]any{
					"schema": "guyide/v1",
					"level":  "info",
					"expr":   expr,
					"value":  val,
					"socket": socket,
					"buffer": buf,
					"cwd":    cwd,
				})
				return nil
			}
			out.Header("guyide nvim eval")
			out.KeyValue("expr", expr)
			out.KeyValue("value", fmt.Sprintf("%v", val))
			out.KeyValue("socket", socket)
			out.KeyValue("buffer", buf)
			out.KeyValue("cwd", cwd)
			return nil
		},
	})
	return c
}

// nvimContext fetches the current buffer path and cwd from nvim so the
// caller (typically an AI agent) can verify they're talking to the right
// instance. Errors are swallowed — this is best-effort context.
func nvimContext(cli *gnvim.Client) (buffer, cwd string) {
	if val, err := cli.Eval(`expand("%:p")`); err == nil {
		buffer, _ = val.(string)
	}
	if val, err := cli.Eval(`getcwd()`); err == nil {
		cwd, _ = val.(string)
	}
	return
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
