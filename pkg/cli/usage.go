package cli

import (
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// SetColorAndStyles applies color and style templates to cobra commands
func SetColorAndStyles(cmd *cobra.Command) {
	cobra.AddTemplateFunc("StyleHeading", color.New(color.FgGreen).SprintFunc())
	cobra.AddTemplateFunc("Name", color.New(color.FgBlue).SprintFunc())

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
