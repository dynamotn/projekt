package config

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewConfigCheckCmd(out io.Writer) *cobra.Command {
	strict := false

	cmd := &cobra.Command{
		Use:     "check",
		Aliases: []string{"validate", "lint"},
		Short:   "Validate the configuration file",
		Long: `Validate the configuration file and report every problem found.

Exits non-zero when there is an error, so it can gate a CI job. Warnings are
reported but do not fail the check unless --strict is given.

Unlike 'projekt folder check', which inspects the Git repositories on disk,
this only reads the configuration itself.`,
		Args: cobra.NoArgs,
		// The root command refuses to run anything on a config it could not
		// read. This is the one command that has to run anyway: reporting why
		// the file is unreadable is exactly its job.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigCheck(out, strict)
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors")

	cli.SetColorAndStyles(cmd)
	return cmd
}

// loadConfigQuietly reads the configuration without letting the loader log its
// own validation warnings. The report this command prints is the whole point of
// running it, and the loader's copy would only interleave a second, partial one.
func loadConfigQuietly() lazypath.Config {
	env := cli.GetEnv()
	previous := env.LogLevel
	env.LogLevel = cli.FATAL
	defer func() { env.LogLevel = previous }()

	return lazypath.GetConfig()
}

func runConfigCheck(out io.Writer, strict bool) error {
	path := lazypath.ConfigFile()
	if _, err := fmt.Fprintf(out, "Checking %s\n", path); err != nil {
		return err
	}

	if err := lazypath.LoadError(); err != nil {
		// An unreadable file has no contents to diagnose, so this is the whole
		// report rather than one entry in it.
		if _, printErr := fmt.Fprintf(out, "[%s] %v\n", lazypath.SeverityError, err); printErr != nil {
			return printErr
		}
		return fmt.Errorf("configuration is not usable: %w", err)
	}

	config := loadConfigQuietly()
	diags := append(config.Diagnose(), folderutil.DiagnoseWorktrees(config)...)

	var errorCount, warningCount int
	for _, diag := range diags {
		if diag.Severity == lazypath.SeverityError {
			errorCount++
		} else {
			warningCount++
		}
		if _, err := fmt.Fprintf(out, "[%s] %s\n", diag.Severity, diag.Message); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "%d folder(s), %d worktree(s), %d git server(s): %d error(s), %d warning(s)\n",
		len(config.Folders), len(config.Worktrees), len(config.GitServers), errorCount, warningCount); err != nil {
		return err
	}

	if errorCount > 0 {
		return fmt.Errorf("configuration has %d error(s)", errorCount)
	}
	if strict && warningCount > 0 {
		return fmt.Errorf("configuration has %d warning(s) and --strict is set", warningCount)
	}

	return nil
}
