package root

import (
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// completeTemplateNames completes the first argument with the templates of the
// store, so `t new <TAB>` offers what can actually be rendered.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	templates, err := tplutil.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	names := make([]string, 0, len(templates))
	for _, tpl := range templates {
		names = append(names, tpl.Name+"\t"+string(tpl.Kind)+" template")
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeSetKeys completes `--set` with what the templates in play actually
// ask for, and `--set key=` with the answers a choice or a bool accepts.
//
// The keys are the one thing a person cannot guess from the command line, and
// they are already written down in the template.
func completeSetKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	templates, err := completionTemplates(cmd, args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var vars []tplutil.Var
	for _, tpl := range templates {
		found, err := tplutil.Vars(tpl)
		if err != nil {
			continue
		}
		vars = append(vars, found...)
	}
	return CompleteValueAssignment(vars, toComplete)
}

// completionTemplates works out which templates a completion is about: the
// ones named on the command line, or everything the project records.
func completionTemplates(cmd *cobra.Command, args []string) ([]tplutil.Template, error) {
	if len(args) > 0 {
		templates := make([]tplutil.Template, 0, len(args))
		for _, name := range args {
			tpl, err := tplutil.Get(name)
			if err != nil {
				return nil, err
			}
			templates = append(templates, tpl)
		}
		return templates, nil
	}

	// `t apply --set <TAB>` with no template named: the project knows.
	project, _ := cmd.Flags().GetString("project")
	return tplutil.Targets(tplutil.ApplyOptions{Dest: project})
}

// CompleteValueAssignment turns a list of variables into `--set` completions.
func CompleteValueAssignment(vars []tplutil.Var, toComplete string) ([]string, cobra.ShellCompDirective) {
	if key, _, found := strings.Cut(toComplete, "="); found {
		return completeAnswers(vars, key)
	}

	seen := map[string]bool{}
	keys := make([]string, 0, len(vars))
	for _, v := range vars {
		if seen[v.Name] {
			continue
		}
		seen[v.Name] = true

		key := v.Name + "="
		if description := describeVar(v); description != "" {
			key += "\t" + description
		}
		keys = append(keys, key)
	}
	// No space, because what follows the `=` is the point.
	return keys, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// completeAnswers offers the answers a variable actually accepts.
func completeAnswers(vars []tplutil.Var, key string) ([]string, cobra.ShellCompDirective) {
	for _, v := range vars {
		if v.Name != key {
			continue
		}
		switch v.Type {
		case tplutil.VarChoice:
			answers := make([]string, 0, len(v.Choices))
			for _, choice := range v.Choices {
				answers = append(answers, key+"="+choice)
			}
			return answers, cobra.ShellCompDirectiveNoFileComp
		case tplutil.VarBool:
			return []string{key + "=true", key + "=false"}, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// describeVar is what the shell shows beside a key.
//
// A default that is itself a template is left out rather than shown raw: `[{{
// .User }}]` in a completion menu tells nobody anything, and rendering it
// would need a destination that does not exist yet.
func describeVar(v tplutil.Var) string {
	description := v.Prompt
	if description == "" && v.Type != "" && v.Type != tplutil.VarString {
		description = string(v.Type)
	}
	if v.Default != "" && !strings.Contains(v.Default, "{{") {
		if description != "" {
			description += " "
		}
		description += "[" + v.Default + "]"
	}
	return description
}

// completeMoreTemplateNames completes a variadic template argument, leaving
// out the ones already named.
func completeMoreTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	templates, err := tplutil.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	named := map[string]bool{}
	for _, name := range args {
		named[name] = true
	}

	names := make([]string, 0, len(templates))
	for _, tpl := range templates {
		if named[tpl.Name] || !tpl.IsDir() {
			// Only a folder template can be applied to a project.
			continue
		}
		names = append(names, tpl.Name+"\t"+string(tpl.Kind)+" template")
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeDirs offers folders alone, for the flags that name a project.
func completeDirs(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// registerValueCompletion wires the flags whose values are worth completing.
func registerValueCompletion(cmd *cobra.Command, set func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) {
	if cmd.Flag("set") != nil {
		_ = cmd.RegisterFlagCompletionFunc("set", set)
	}
	if cmd.Flag("project") != nil {
		_ = cmd.RegisterFlagCompletionFunc("project", completeDirs)
	}
}
