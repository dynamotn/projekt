package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const applyLongHelp = `Render a template over a project it already created.

` + "`t new`" + ` writes a project once. ` + "`t apply`" + ` writes it again, which is how a
change to a template reaches the projects made from it: fix the CI job in one
place, then apply it wherever it belongs.

Named no template, it applies every template the project records, so it needs
to be told nothing about a project to bring it up to date:

  projekt folder exec -t work -- t apply

A project remembers what it was rendered from in .projekt/template.yaml — the
template, the values, and the hash of every file written. That record is what
tells an out-of-date file apart from one somebody edited:

  added      the template writes it and the project does not have it
  updated    the template writes it differently, and it is untouched since
  unchanged  it is already what the template says
  conflict   it differs *and* it was edited by hand — kept, unless --force
  removed    the template no longer writes it — kept, unless --prune

The recorded values are replayed, so applying needs no flags; --set and
--values change one answer and keep the rest.

Examples:

  t apply                           # everything this project records
  t apply -C ./myapp                # somewhere else
  t apply go-cli                    # one of its templates
  t apply go-cli --set ci=true      # change one answer, replay the others
  t apply --force                   # rewrite what was edited by hand too
  t apply --prune                   # and delete what it no longer writes`

const diffLongHelp = `Show what applying a template would change.

Exactly what ` + "`t apply`" + ` would do, written as a unified diff and touching
nothing. It exits 1 when there is something to do, so a CI job can gate on a
project having drifted from its template.

Examples:

  t diff                                    # everything this project records
  t diff go-cli -C ./myapp --set ci=true
  t diff --name-only
  projekt folder exec -t work -- t diff     # which projects have drifted`

// NewTemplateApplyCmd renders a template over a project again.
func NewTemplateApplyCmd(out io.Writer) *cobra.Command {
	return applyCommand(out, false)
}

// NewTemplateDiffCmd is the same thing, seen rather than done.
func NewTemplateDiffCmd(out io.Writer) *cobra.Command {
	return applyCommand(out, true)
}

func applyCommand(out io.Writer, diff bool) *cobra.Command {
	var (
		sets        []string
		valueFiles  []string
		interactive bool
		namesOnly   bool
		context     int
	)
	o := tplutil.ApplyOptions{DryRun: diff}

	use, short, long := "apply [template...]", "Render a template over a project again", applyLongHelp
	aliases := []string{"up", "update"}
	if diff {
		use, short, long = "diff [template...]", "Show what applying a template would change", diffLongHelp
		aliases = []string{"status"}
	}

	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		Long:              long,
		Args:              cobra.ArbitraryArgs,
		Aliases:           aliases,
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range args {
				tpl, err := tplutil.Get(name)
				if err != nil {
					return err
				}
				o.Templates = append(o.Templates, tpl)
			}

			fileValues, err := tplutil.LoadValuesFiles(valueFiles)
			if err != nil {
				return err
			}
			setValues, err := tplutil.ParseSet(sets)
			if err != nil {
				return err
			}
			o.Values = tplutil.MergeValues(fileValues, setValues)
			o.In = cmd.InOrStdin()
			o.Prompt = cmd.ErrOrStderr()
			o.Interactive = interactive

			changes, err := tplutil.Apply(o)
			if err != nil {
				return err
			}
			return report(cmd, out, changes, o, diff, namesOnly, context)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Dest, "project", "C", "", "Project to work on (default is the current folder)")
	f.StringVarP(&o.Name, "name", "n", "", "Name to render with, overriding the recorded one")
	f.StringArrayVarP(&sets, "set", "s", nil, "Set a template value, like -s key=value (repeatable)")
	f.StringArrayVarP(&valueFiles, "values", "f", nil, "YAML file of template values (repeatable)")
	f.BoolVarP(&interactive, "interactive", "i", false, "Ask for the values the template needs")
	f.BoolVar(&namesOnly, "name-only", false, "Print the paths alone, one per line")
	f.IntVarP(&context, "unified", "U", 3, "Lines of context around each change")
	if !diff {
		f.BoolVarP(&o.Force, "force", "F", false, "Rewrite the files that were edited by hand too")
		f.BoolVar(&o.Prune, "prune", false, "Delete the files the template no longer writes")
	}

	return cmd
}

// report prints what happened, or what would.
//
// `t diff` exits non-zero when it found something, so a CI job can gate on a
// project still matching its template.
func report(cmd *cobra.Command, out io.Writer, changes []tplutil.Change, o tplutil.ApplyOptions, diff, namesOnly bool, context int) error {
	several := templateCount(changes) > 1
	pending := 0
	for _, change := range changes {
		if !change.Writes() {
			continue
		}
		pending++

		if namesOnly {
			if _, err := fmt.Fprintln(out, change.Path); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "%s\t%s%s%s\n", change.Status, change.Path, from(change, several), kept(change, o)); err != nil {
			return err
		}
		if !diff {
			continue
		}
		if _, err := fmt.Fprint(out, tplutil.UnifiedDiff(change.Path, change.Before, change.After, context)); err != nil {
			return err
		}
	}

	if !namesOnly {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), tplutil.Summary(changes)); err != nil {
			return err
		}
	}
	if diff && pending > 0 {
		return fmt.Errorf("%s has drifted from its templates: %s", project(o), tplutil.Summary(changes))
	}
	return nil
}

// templateCount is how many templates the changes come from, which decides
// whether each line has to say which one it is.
func templateCount(changes []tplutil.Change) int {
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.Template] = true
	}
	return len(seen)
}

// from names the template a change comes from, once there is more than one.
func from(change tplutil.Change, several bool) string {
	if !several {
		return ""
	}
	return "  (" + change.Template + ")"
}

// project names the folder a diff was run against, for the closing message.
func project(o tplutil.ApplyOptions) string {
	if o.Dest == "" {
		return "the current folder"
	}
	return o.Dest
}

// kept says why a change was left alone, so a conflict is never silent.
func kept(change tplutil.Change, o tplutil.ApplyOptions) string {
	switch {
	case change.Status == tplutil.ChangeConflict && !o.Force:
		return "  (edited since, kept — use --force)"
	case change.Status == tplutil.ChangeRemoved && !o.Prune:
		return "  (no longer written, kept — use --prune)"
	default:
		return ""
	}
}
