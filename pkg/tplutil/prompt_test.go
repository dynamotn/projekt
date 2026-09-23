package tplutil

import (
	"bytes"
	"strings"
	"testing"
)

// answer runs a prompter over the given typed lines.
func answer(t *testing.T, vars []Var, given Values, typed string, base map[string]any) (Values, string, error) {
	t.Helper()
	var out bytes.Buffer
	p := Prompter{In: strings.NewReader(typed), Out: &out}
	values, err := p.Ask(vars, given, base)
	return values, out.String(), err
}

func TestPrompter_AsksAndParses(t *testing.T) {
	vars := []Var{
		{Name: "title", Prompt: "What is it called"},
		{Name: "port", Type: VarInt},
		{Name: "debug", Type: VarBool},
		{Name: "tags", Type: VarList},
		{Name: "status", Type: VarChoice, Choices: []string{"draft", "final"}},
		{Name: "author.name"},
	}

	values, out, err := answer(t, vars, nil, "A note\n8080\nyes\ngo, work ,\nfinal\nJane\n", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if values["title"] != "A note" {
		t.Errorf("title = %#v", values["title"])
	}
	if values["port"] != 8080 {
		t.Errorf("port = %#v, want the int 8080", values["port"])
	}
	if values["debug"] != true {
		t.Errorf("debug = %#v, want the bool true", values["debug"])
	}
	tags, ok := values["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "go" || tags[1] != "work" {
		t.Errorf("tags = %#v, want [go work]", values["tags"])
	}
	if values["status"] != "final" {
		t.Errorf("status = %#v", values["status"])
	}
	author, ok := values["author"].(Values)
	if !ok || author["name"] != "Jane" {
		t.Errorf("author = %#v, want a nested name", values["author"])
	}

	// The question is what the prompt says, and a choice lists its options.
	if !strings.Contains(out, "What is it called") {
		t.Errorf("output = %q, want the prompt text", out)
	}
	if !strings.Contains(out, "(draft/final)") {
		t.Errorf("output = %q, want the choices", out)
	}
}

func TestPrompter_EmptyAnswerTakesTheDefault(t *testing.T) {
	vars := []Var{
		{Name: "author", Default: "{{ .User }}"},
		{Name: "port", Type: VarInt, Default: "8080"},
		{Name: "note"},
	}
	base := map[string]any{"User": "jane"}

	values, out, err := answer(t, vars, nil, "\n\n\n", base)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if values["author"] != "jane" {
		t.Errorf("author = %#v, want the rendered default", values["author"])
	}
	if values["port"] != 8080 {
		t.Errorf("port = %#v, want the parsed default", values["port"])
	}
	// A variable with no default and no answer stays unset, so the template's
	// own `| default` still applies.
	if _, set := values["note"]; set {
		t.Errorf("note = %#v, want it left unset", values["note"])
	}
	if !strings.Contains(out, "[jane]") {
		t.Errorf("output = %q, want the default shown", out)
	}
}

func TestPrompter_SkipsWhatIsAlreadySet(t *testing.T) {
	vars := []Var{{Name: "title"}, {Name: "author"}}
	given := Values{"title": "already"}

	values, out, err := answer(t, vars, given, "typed\n", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if values["title"] != "already" {
		t.Errorf("title = %#v, want the value that was given", values["title"])
	}
	if values["author"] != "typed" {
		t.Errorf("author = %#v, want the typed answer", values["author"])
	}
	if strings.Contains(out, "title") {
		t.Errorf("output = %q, want no question for a value already set", out)
	}
	// The caller's values are not modified.
	if len(given) != 1 {
		t.Error("Ask() modified the values it was given")
	}
}

func TestPrompter_AsksAgainAfterABadAnswer(t *testing.T) {
	vars := []Var{{Name: "port", Type: VarInt}}

	values, out, err := answer(t, vars, nil, "eighty\n80\n", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if values["port"] != 80 {
		t.Errorf("port = %#v", values["port"])
	}
	if !strings.Contains(out, "not a whole number") {
		t.Errorf("output = %q, want the reason", out)
	}
	if strings.Count(out, "port") < 2 {
		t.Errorf("output = %q, want the question asked again", out)
	}
}

func TestPrompter_RequiredIsAskedAgain(t *testing.T) {
	vars := []Var{{Name: "title", Required: true}}

	values, out, err := answer(t, vars, nil, "\nAt last\n", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if values["title"] != "At last" {
		t.Errorf("title = %#v", values["title"])
	}
	if !strings.Contains(out, "is required") {
		t.Errorf("output = %q, want the reason", out)
	}
}

func TestPrompter_RequiredWithNothingToRead(t *testing.T) {
	vars := []Var{{Name: "title", Required: true}}

	if _, _, err := answer(t, vars, nil, "", nil); err == nil {
		t.Error("Ask() error = nil, want an error when a required value cannot be read")
	}
}

func TestPrompter_EndOfInputTakesTheDefaults(t *testing.T) {
	vars := []Var{
		{Name: "first"},
		{Name: "second", Default: "fallback"},
		{Name: "third"},
	}

	// Answers can be piped in; what is left over falls back.
	values, _, err := answer(t, vars, nil, "typed\n", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if values["first"] != "typed" {
		t.Errorf("first = %#v", values["first"])
	}
	if values["second"] != "fallback" {
		t.Errorf("second = %#v, want the default", values["second"])
	}
	if _, set := values["third"]; set {
		t.Errorf("third = %#v, want it left unset", values["third"])
	}
}

func TestPrompter_NoVars(t *testing.T) {
	values, out, err := answer(t, nil, Values{"a": 1}, "", nil)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if values["a"] != 1 {
		t.Errorf("values = %#v, want them returned untouched", values)
	}
	if out != "" {
		t.Errorf("output = %q, want nothing asked", out)
	}
}

func TestPrompter_BrokenDefault(t *testing.T) {
	vars := []Var{{Name: "author", Default: "{{ .User "}}

	if _, _, err := answer(t, vars, nil, "\n", nil); err == nil {
		t.Error("Ask() error = nil, want the broken default reported")
	}
}
