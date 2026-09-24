package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const checkLongHelp = `Read a template the way rendering would, and report what is wrong.

A template carries four files that describe it — .vars.yaml, .data.yaml,
.ignore and the .templates folder — so there is more to get wrong than there
used to be, and a typo found here is one nobody has to undo afterwards.

It parses every file and every path segment with the template's own
delimiters, resolves the shared templates it calls, renders the whole tree with
the manifest's own defaults, and compares what the manifest asks for with what
the template actually reads.

Nothing is written. Exits non-zero when there is an error, so it can gate a CI
job; warnings do not fail the check unless --strict is given.

Examples:

  t check              # every template of the store
  t check go-cli       # one of them
  t check --strict`

func NewTemplateCheckCmd(out io.Writer) *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:               "check [template]",
		Short:             "Validate a template, or the whole store",
		Long:              checkLongHelp,
		Args:              cobra.MaximumNArgs(1),
		Aliases:           []string{"validate", "lint"},
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			reports, err := collectReports(args)
			if err != nil {
				return err
			}
			return printReports(out, reports, strict)
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors")

	return cmd
}

// collectReports checks the one template named, or the whole store.
func collectReports(args []string) ([]tplutil.Report, error) {
	if len(args) == 1 {
		tpl, err := tplutil.Get(args[0])
		if err != nil {
			return nil, err
		}
		return []tplutil.Report{tplutil.Check(tpl)}, nil
	}
	return tplutil.CheckAll()
}

// printReports writes every problem found, template by template.
func printReports(out io.Writer, reports []tplutil.Report, strict bool) error {
	var errors, warnings int

	for _, report := range reports {
		errors += report.Errors()
		warnings += report.Warnings()
		if len(report.Diagnostics) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s\n", report.Template.Name); err != nil {
			return err
		}
		for _, diag := range report.Diagnostics {
			if _, err := fmt.Fprintf(out, "  [%s] %s\n", diag.Severity, diag.Message); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintf(out, "%d template(s): %d error(s), %d warning(s)\n",
		len(reports), errors, warnings); err != nil {
		return err
	}

	switch {
	case errors > 0:
		return fmt.Errorf("%d template error(s)", errors)
	case strict && warnings > 0:
		return fmt.Errorf("%d template warning(s) and --strict is set", warnings)
	default:
		return nil
	}
}
