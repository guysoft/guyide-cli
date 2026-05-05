package cli

import (
	"github.com/guysoft/guyide-cli/internal/discover"
	"github.com/guysoft/guyide-cli/pkg/schema"
	"github.com/spf13/cobra"
)

func newDoctorCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the guyide environment health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			env := discover.Resolve(discover.Options{
				ExplicitSocket:  g.Socket,
				ExplicitSession: g.Session,
			})

			report := schema.DoctorReport{
				Envelope: schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
			}
			add := func(group, name, status, msg string) {
				report.Checks = append(report.Checks, schema.DoctorCheck{
					Group: group, Name: name, Status: status, Message: msg,
				})
				switch status {
				case "ok":
					report.Passed++
				case "warn":
					report.Warnings++
				case "fail":
					report.Failures++
				}
			}

			// tmux group
			if env.TmuxSession != "" {
				add("tmux", "server reachable", "ok", "session "+env.TmuxSession)
			} else {
				add("tmux", "server reachable", "warn", "no tmux session detected")
			}

			// nvim group
			if env.Socket == "" {
				add("nvim", "socket discovered", "fail", "no NVIM_IDE_SOCK or scan match")
			} else {
				add("nvim", "socket discovered", "ok", env.Socket+" via "+env.SocketSource)
				if env.NvimReachable {
					add("nvim", "RPC reachable", "ok", "")
				} else {
					add("nvim", "RPC reachable", "fail", "socket present but not accepting connections")
				}
			}

			report.Ready = report.Failures == 0

			if g.JSON {
				out.JSON(report)
				return nil
			}

			out.Header("guyide doctor")
			groupSeen := map[string]bool{}
			step := 0
			groups := []string{"tmux", "nvim"}
			for _, group := range groups {
				if groupSeen[group] {
					continue
				}
				groupSeen[group] = true
				step++
				out.Step(step, len(groups), group)
				for _, ch := range report.Checks {
					if ch.Group != group {
						continue
					}
					switch ch.Status {
					case "ok":
						out.Success("%s  %s", ch.Name, ch.Message)
					case "warn":
						out.Warning("%s  %s", ch.Name, ch.Message)
					case "fail":
						out.Error("%s  %s", ch.Name, ch.Message)
					}
				}
			}
			out.Summary("Summary", map[string]string{
				"Checks passed": itoa(report.Passed),
				"Warnings":      itoa(report.Warnings),
				"Failures":      itoa(report.Failures),
			})
			if report.Ready {
				out.Success("Ready")
			} else {
				out.Error("Not ready: see failures above")
			}
			return nil
		},
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
