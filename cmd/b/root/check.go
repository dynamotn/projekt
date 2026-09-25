package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
)

const checkLongHelp = `Read a recipe the way creating from it would, and report what is wrong.

It parses the recipe without stopping at the first problem, resolves the
template it renders and checks that too, parses every ` + "`after`" + ` command line,
and looks at where the project would go and what it would point at.

Nothing is written and no project is created. Exits non-zero when there is an
error, so it can gate a CI job; warnings do not fail the check unless --strict
is given.

Examples:

  b check              # every recipe of the store
  b check go-cli       # one of them
  b check --strict`

func NewBoilerplateCheckCmd(out io.Writer) *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:               "check [boilerplate]",
		Short:             "Validate a recipe, or the whole store",
		Long:              checkLongHelp,
		Args:              cobra.MaximumNArgs(1),
		Aliases:           []string{"validate", "lint"},
		ValidArgsFunction: completeRecipeNames,
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

// collectReports checks the one recipe named, or the whole store.
func collectReports(args []string) ([]bplutil.Report, error) {
	if len(args) == 1 {
		return []bplutil.Report{bplutil.Check(args[0])}, nil
	}
	return bplutil.CheckAll()
}

// printReports writes every problem found, recipe by recipe.
func printReports(out io.Writer, reports []bplutil.Report, strict bool) error {
	var errors, warnings int

	for _, report := range reports {
		errors += report.Errors()
		warnings += report.Warnings()
		if len(report.Diagnostics) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s\n", report.Name); err != nil {
			return err
		}
		for _, diag := range report.Diagnostics {
			if _, err := fmt.Fprintf(out, "  [%s] %s\n", diag.Severity, diag.Message); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintf(out, "%d boilerplate(s): %d error(s), %d warning(s)\n",
		len(reports), errors, warnings); err != nil {
		return err
	}

	switch {
	case errors > 0:
		return fmt.Errorf("%d boilerplate error(s)", errors)
	case strict && warnings > 0:
		return fmt.Errorf("%d boilerplate warning(s) and --strict is set", warnings)
	default:
		return nil
	}
}
