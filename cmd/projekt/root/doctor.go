package root

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/doctor"
)

// outputText is the report as a person reads it, and the default. The other
// formats are the ones every listing here offers.
const outputText = "text"

// doctorFormats are what --output accepts.
var doctorFormats = append([]string{outputText}, cli.OutputFormats...)

const doctorLongHelp = `Check that this machine is set up to use projekt.

A fresh machine, or one where something was moved, raises the same question:
is any of this going to work. This answers it in one place, instead of leaving
it to be discovered one failed command at a time.

  projekt doctor
  projekt doctor --strict   # a warning fails the check too

It looks at git, the binaries, whether the shell integration is sourced, the
configuration and its diagnostics, whether the configured folders are still on
disk, and the template and boilerplate stores.

Exits non-zero when something will not work, so it can gate a setup script.

Nothing is changed. It only reads.`

func NewDoctorCmd(out io.Writer) *cobra.Command {
	strict := false
	output := outputText
	noColor := false

	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Check that this machine is set up to use projekt",
		Long:    doctorLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"check-setup"},
		// Reporting that the configuration is unreadable is part of the job,
		// so this has to run even then.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(out, strict, output, noColor)
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as failures")
	cmd.Flags().StringVarP(&output, "output", "o", outputText,
		fmt.Sprintf("Output format, one of: %s", strings.Join(doctorFormats, ", ")))
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Don't use color")

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return doctorFormats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --output: %v", err)
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}

func runDoctor(out io.Writer, strict bool, output string, noColor bool) error {
	checks := doctor.Run()

	var failed, warned int
	for _, check := range checks {
		switch check.Status {
		case doctor.StatusFail:
			failed++
		case doctor.StatusWarn:
			warned++
		}
	}

	if err := reportChecks(out, checks, output, noColor); err != nil {
		return err
	}
	if output == outputText {
		if _, err := fmt.Fprintf(out, "%d check(s): %d failed, %d warning(s)\n",
			len(checks), failed, warned); err != nil {
			return err
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	if strict && warned > 0 {
		return fmt.Errorf("%d warning(s) and --strict is set", warned)
	}
	return nil
}

// reportChecks writes the checks, as a person reads them or as a script does.
func reportChecks(out io.Writer, checks []doctor.Check, output string, noColor bool) error {
	if output == outputText {
		for _, check := range checks {
			if _, err := fmt.Fprintf(out, "[%s] %s: %s\n", check.Status, check.Name, check.Detail); err != nil {
				return err
			}
		}
		return nil
	}

	format, err := cli.ParseOutputFormat(output)
	if err != nil {
		return fmt.Errorf("%w; or %q", err, outputText)
	}

	// The checks come in the order they are worth reading, which a table must
	// not sort away.
	view := cli.ListView{Ordered: true, Columns: []cli.ListColumn{
		{Header: "CHECK", Key: "check"},
		{Header: "STATUS", Key: "status"},
		{Header: "DETAIL", Key: "detail"},
	}}
	for _, check := range checks {
		view.AppendRow(check.Name, string(check.Status), check.Detail)
	}

	return cli.EncodeList(out, view, cli.ListOutputOption{Output: format, NoColor: noColor})
}
