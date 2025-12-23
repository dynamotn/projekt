package cli

import (
	"fmt"
	"io"
	"text/template"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/internal/version"
)

type versionOptions struct {
	short    bool
	template string
}

func NewVersionCmd(out io.Writer) *cobra.Command {
	o := &versionOptions{}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Projekt tools",
		Long: `
Show the version for Projekt tools.

This will print a representation the version of Projekt.
The output will look something like this:

Projekt CLI version.BuildInfo{Version:"vx.x.x", GitCommit:"ffffffffffffffffffffffffffffffffffffffff", GitTreeState:"clean", GoVersion:"gox.x.x", BuildTime:"xxxx-xx-xxTxx:xx:xxZ"}

- Version is the semantic version of the release.
- GitCommit is the SHA for the commit that this version was built from.
- GitTreeState is "clean" if there are no local code changes when this binary was
  built, and "dirty" if the binary was built from locally modified code.
- GoVersion is the version of Go that was used to compile Projekt.
- BuildTime is the time when Projekt was compiled.

When using the --template flag the following properties are available to use in
the template:

- .Version contains the semantic version of Projekt
- .GitCommit is the git commit
- .GitTreeState is the state of the git tree when Projekt was built
- .GoVersion contains the version of Go that Projekt was compiled with
- .BuildTime contains the time when Projekt was compiled

For example, --template='Version: {{.Version}}' outputs 'Version: vx.x.x'.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.short, "short", false, "Print the version number")
	f.StringVar(&o.template, "template", "", "Template for version string format")

	return cmd
}

func (o *versionOptions) run(out io.Writer) error {
	if o.template != "" {
		tt, err := template.New("_").Parse(o.template)
		if err != nil {
			return err
		}
		return tt.Execute(out, version.GetBuildInfo())
	}
	_, err := fmt.Fprintln(out, formatVersion(o.short))
	return err
}

func formatVersion(short bool) string {
	v := version.GetBuildInfo()
	if short {
		if len(v.GitCommit) >= 7 {
			return fmt.Sprintf("Projekt CLI %s+git%s", v.Version, v.GitCommit[:7])
		}
		return version.GetVersionStr()
	}
	return fmt.Sprintf("Projekt CLI %#v", v)
}
