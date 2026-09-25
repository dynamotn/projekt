package bplutil

import (
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// TrustOrigin is the origin of what a recipe runs: the recipe file and the
// template of the store it renders, because trusting a recipe is trusting
// what it creates from.
func TrustOrigin(recipe Recipe) tplutil.Origin {
	origin := tplutil.Origin{
		Kind:  "recipe",
		Name:  recipe.Name,
		Key:   recipe.Path,
		Paths: []string{recipe.Path},
		Hint:  "b trust " + recipe.Name,
	}
	if tpl, ok := recipeTemplate(recipe); ok {
		origin.Paths = append(origin.Paths, tplutil.TemplateOrigin(tpl).Paths...)
	}
	return origin
}

// Trust records a recipe as trusted as it is now, and the template it
// renders with it, whose own commands run as part of the same `b new`.
func Trust(recipe Recipe) error {
	if err := tplutil.Trust(TrustOrigin(recipe)); err != nil {
		return err
	}
	if tpl, ok := recipeTemplate(recipe); ok {
		return tplutil.Trust(tplutil.TemplateOrigin(tpl))
	}
	return nil
}

// repoOrigin is the origin of a starting point cloned to be rendered. It is
// recorded under the repository and its ref rather than the temporary folder
// it lands in, so that trusting one clone trusts the next one of the same
// commit.
func repoOrigin(recipe Recipe, url, clone string) tplutil.Origin {
	return tplutil.Origin{
		Kind:  "repository",
		Name:  url,
		Key:   "repo:" + url + "@" + recipe.Source.Ref,
		Paths: []string{clone},
	}
}

// recipeTemplate returns the template of the store a recipe renders, if it
// renders one that exists.
func recipeTemplate(recipe Recipe) (tplutil.Template, bool) {
	if recipe.Source.Template == "" {
		return tplutil.Template{}, false
	}
	tpl, err := tplutil.Get(recipe.Source.Template)
	return tpl, err == nil
}
