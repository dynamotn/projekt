package pkg

import (
	"strings"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/fatih/color"
)

func SetColorAndStyles(command *cobra.Command) {
	command.SetOutput(color.Output)

	cobra.AddTemplateFunc("StyleHeading", color.New(color.FgGreen).SprintFunc())
	usageTemplate := command.UsageTemplate()
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

	command.SetUsageTemplate(usageTemplate)
}
