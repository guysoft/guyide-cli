package cli

import (
	"github.com/guysoft/guyide-cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, schema, and platform info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			info := version.Info()
			if g.JSON {
				out.JSON(info)
				return nil
			}
			out.Header("guyide version")
			out.KeyValue("version", info.Version)
			out.KeyValue("commit", info.Commit)
			out.KeyValue("build_date", info.BuildDate)
			out.KeyValue("go", info.GoVersion)
			out.KeyValue("platform", info.Platform)
			out.KeyValue("schema", info.Schema)
			return nil
		},
	}
}
