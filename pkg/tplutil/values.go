package tplutil

import (
	"fmt"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Values holds the user supplied variables of a rendering, the ones a template
// reaches through `.Values`.
type Values map[string]any

// ParseSet turns the `--set` arguments into nested values.
//
// A key is split on dots, so `--set author.name=me` is the same YAML as
// `author: {name: me}`. Values are parsed as YAML scalars, which gives typed
// booleans and numbers for free; anything that does not parse stays a string.
func ParseSet(assignments []string) (Values, error) {
	values := Values{}
	for _, assignment := range assignments {
		key, raw, found := strings.Cut(assignment, "=")
		if !found {
			return nil, fmt.Errorf("invalid --set %q: expected key=value", assignment)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid --set %q: empty key", assignment)
		}
		if err := assign(values, strings.Split(key, "."), parseScalar(raw)); err != nil {
			return nil, fmt.Errorf("invalid --set %q: %w", assignment, err)
		}
	}
	return values, nil
}

// parseScalar gives `--set port=8080` an int and `--set debug=true` a bool,
// while leaving everything YAML cannot read as the plain string it was typed as.
func parseScalar(raw string) any {
	var parsed any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	if parsed == nil {
		// Both an empty value and a literal `null`: an empty string is what the
		// user typed and the more useful of the two in a template.
		return raw
	}
	return parsed
}

// assign writes value at the nested path, creating the intermediate maps.
func assign(values Values, path []string, value any) error {
	current := values
	for i, key := range path {
		if key == "" {
			return fmt.Errorf("empty path segment")
		}
		if i == len(path)-1 {
			current[key] = value
			return nil
		}
		switch next := current[key].(type) {
		case Values:
			current = next
		case map[string]any:
			current = Values(next)
		case nil:
			child := Values{}
			current[key] = child
			current = child
		default:
			return fmt.Errorf("key %q is already a value, it cannot hold %q", key, path[i+1])
		}
	}
	return nil
}

// LoadValuesFiles reads and merges YAML value files, in the order they are given.
func LoadValuesFiles(paths []string) (Values, error) {
	values := Values{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read values file %s: %w", path, err)
		}
		parsed := map[string]any{}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("cannot parse values file %s: %w", path, err)
		}
		values = MergeValues(values, parsed)
	}
	return values, nil
}

// MergeValues deep merges override into base and returns the result. Maps are
// merged key by key; anything else in override replaces what base had.
func MergeValues(base, override map[string]any) Values {
	merged := Values{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		overrideMap, overrideIsMap := toMap(value)
		baseMap, baseIsMap := toMap(merged[key])
		if overrideIsMap && baseIsMap {
			merged[key] = MergeValues(baseMap, overrideMap)
			continue
		}
		merged[key] = value
	}
	return merged
}

func toMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case Values:
		return typed, true
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}
