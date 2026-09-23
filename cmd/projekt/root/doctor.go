package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/doctor"
)

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
			return runDoctor(out, strict)
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as failures")

	cli.SetColorAndStyles(cmd)
	return cmd
}

func runDoctor(out io.Writer, strict bool) error {
	checks := doctor.Run()

	var failed, warned int
	for _, check := range checks {
		switch check.Status {
		case doctor.StatusFail:
			failed++
		case doctor.StatusWarn:
			warned++
		}
		if _, err := fmt.Fprintf(out, "[%s] %s: %s\n", check.Status, check.Name, check.Detail); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "%d check(s): %d failed, %d warning(s)\n",
		len(checks), failed, warned); err != nil {
		return err
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	if strict && warned > 0 {
		return fmt.Errorf("%d warning(s) and --strict is set", warned)
	}
	return nil
}
