package cli

import (
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func SetColorAndStyles(cmd *cobra.Command) {
	cmd.SetOutput(color.Output)

	cobra.AddTemplateFunc("StyleHeading", color.New(color.FgGreen).SprintFunc())

	usageTemplate := cmd.UsageTemplate()
	usageTemplate = strings.NewReplacer(
		`Usage:`, `{{StyleHeading "USAGE:"}}`,
		`Aliases:`, `{{StyleHeading "ALIASES:"}}`,
		`Available Commands:`, `{{StyleHeading "AVAILABLE COMMANDS:"}}`,
		`Global Flags:`, `{{StyleHeading "GLOBAL FLAGS:"}}`,
		`Flags:`, `{{StyleHeading "FLAGS:"}}`,
		`Additional help topics:`, `{{StyleHeading "ADDITIONAL HELP TOPICS:"}}`,
	).Replace(usageTemplate)
	re := regexp.MustCompile(`(?m)^Flags:\s*$`)
	usageTemplate = re.ReplaceAllLiteralString(usageTemplate, `{{StyleHeading "FLAGS:"}}`)

	cmd.SetUsageTemplate(usageTemplate)
}
