package lazypath

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// defaultIndent is what a file with nothing indented in it yet is written with.
const defaultIndent = 2

// saveConfig writes the named top-level sections of this machine's
// configuration back to its file, and leaves everything else in it alone.
//
// The file is someone's dotfile, usually under version control, so it is
// merged into rather than regenerated: comments, key order, quoting and the
// sections no command writes survive, and an entry that did not change is
// written exactly as it was read. A command that adds one folder produces a
// diff of one folder.
func saveConfig(sections ...string) error {
	path := ConfigFile()
	if path == "" {
		return errors.New("no config file to write")
	}
	// A dotfiles manager may link the file from elsewhere; replacing the link
	// with a regular file would quietly take it out of that repository.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("cannot parse %s: %w", path, err)
	}
	root := documentRoot(&doc)

	var wanted yaml.Node
	if err := wanted.Encode(c); err != nil {
		return err
	}

	for _, section := range sections {
		value := mappingValue(&wanted, section)
		index := mappingIndex(root, section)
		switch {
		case value == nil && index >= 0:
			// An omitempty section that became empty.
			root.Content = append(root.Content[:index], root.Content[index+2:]...)
		case value == nil:
		case index >= 0:
			root.Content[index].Value = section
			root.Content[index+1] = mergeNode(root.Content[index+1], value)
		default:
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: section},
				pruneZero(value))
		}
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(detectIndent(data))
	if err := encoder.Encode(&doc); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}

	if err := replaceFile(path, restoreBlankLines(data, out.Bytes())); err != nil {
		return err
	}
	// Keep what viper holds in step with the file, so a later read in this
	// process sees what was just written.
	if err := viper.ReadInConfig(); err != nil {
		cli.Debug("Cannot re-read %s: %v", path, err)
	}
	return nil
}

// documentRoot returns the top-level mapping of a parsed file, turning an
// empty or comment-only file into a document that has one.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind != yaml.DocumentNode {
		*doc = yaml.Node{Kind: yaml.DocumentNode, HeadComment: doc.HeadComment}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return doc.Content[0]
}

// mappingIndex returns the index of the key node named key in a mapping, or
// -1. The match ignores case, because viper does: a file written by an older
// release says `gitservers` and still means `gitServers`.
func mappingIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if strings.EqualFold(mapping.Content[i].Value, key) {
			return i
		}
	}
	return -1
}

// mappingValue returns the value under key in a mapping, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if i := mappingIndex(mapping, key); i >= 0 {
		return mapping.Content[i+1]
	}
	return nil
}

// mergeNode returns want, the value a command means to write, expressed as
// changes to have, what the file holds, so that what did not change keeps its
// comments and formatting.
func mergeNode(have, want *yaml.Node) *yaml.Node {
	if have == nil {
		return pruneZero(want)
	}
	if have.Kind != want.Kind {
		return keepComments(pruneZero(want), have)
	}

	switch want.Kind {
	case yaml.MappingNode:
		mergeMapping(have, want)
	case yaml.SequenceNode:
		mergeSequence(have, want)
	case yaml.ScalarNode:
		mergeScalar(have, want)
	default:
		return keepComments(want, have)
	}
	return have
}

func mergeMapping(have, want *yaml.Node) {
	var content []*yaml.Node
	used := make(map[int]bool, len(have.Content)/2)

	// The file's order first: an existing key stays where the reader put it.
	for i := 0; i+1 < len(have.Content); i += 2 {
		key, value := have.Content[i], have.Content[i+1]
		j := mappingIndex(want, key.Value)
		if j < 0 {
			// Gone from what is written. An explicit `is_workspace: false`
			// means what its absence means, and is left the way it was typed.
			if isZero(value) {
				content = append(content, key, value)
			}
			continue
		}
		used[j] = true
		key.Value = want.Content[j].Value
		content = append(content, key, mergeNode(value, want.Content[j+1]))
	}

	// Then what is new, leaving out what would only spell a default.
	for j := 0; j+1 < len(want.Content); j += 2 {
		if used[j] || isZero(want.Content[j+1]) {
			continue
		}
		content = append(content, want.Content[j], pruneZero(want.Content[j+1]))
	}

	have.Content = content
}

// mergeSequence pairs each wanted item with the item of the file it stands
// for — the same `path` or `name`, the same scalar, or failing that the one in
// the same place, which is what a renamed or moved entry is — and merges the
// two.
func mergeSequence(have, want *yaml.Node) {
	matched := make([]*yaml.Node, len(want.Content))
	taken := make([]bool, len(have.Content))

	for i, item := range want.Content {
		id := itemIdentity(item)
		if id == "" {
			continue
		}
		for j, old := range have.Content {
			if !taken[j] && itemIdentity(old) == id {
				matched[i], taken[j] = old, true
				break
			}
		}
	}
	for i := range want.Content {
		if matched[i] == nil && i < len(have.Content) && !taken[i] &&
			have.Content[i].Kind == want.Content[i].Kind {
			matched[i], taken[i] = have.Content[i], true
		}
	}

	content := make([]*yaml.Node, len(want.Content))
	for i, item := range want.Content {
		content[i] = mergeNode(matched[i], item)
	}
	have.Content = content
}

// itemIdentity names the entry a sequence item describes, or returns "" when
// there is nothing to tell it by.
func itemIdentity(item *yaml.Node) string {
	switch item.Kind {
	case yaml.ScalarNode:
		return "=" + item.Value
	case yaml.MappingNode:
		for _, key := range []string{"path", "name"} {
			if value := mappingValue(item, key); value != nil && value.Kind == yaml.ScalarNode {
				return key + "=" + value.Value
			}
		}
	}
	return ""
}

func mergeScalar(have, want *yaml.Node) {
	if have.Value == want.Value && have.ShortTag() == want.ShortTag() {
		return
	}
	quoted := have.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0
	have.Value, have.Tag = want.Value, want.Tag
	// Keep the quotes someone chose, unless the new value needs its own.
	if !(quoted && want.Style == 0 && want.ShortTag() == "!!str") {
		have.Style = want.Style
	}
}

// keepComments carries the comments of the node being replaced over to the
// one replacing it.
func keepComments(node, from *yaml.Node) *yaml.Node {
	node.HeadComment, node.LineComment, node.FootComment =
		from.HeadComment, from.LineComment, from.FootComment
	return node
}

// pruneZero drops, from a node about to be written for the first time, the
// keys whose value is only a default: what a new folder would otherwise carry
// as `prefix: ""`, `regex: ""` and `priority: 0`.
func pruneZero(node *yaml.Node) *yaml.Node {
	switch node.Kind {
	case yaml.MappingNode:
		content := node.Content[:0]
		for i := 0; i+1 < len(node.Content); i += 2 {
			if isZero(node.Content[i+1]) {
				continue
			}
			content = append(content, node.Content[i], pruneZero(node.Content[i+1]))
		}
		node.Content = content
	case yaml.SequenceNode:
		for i, item := range node.Content {
			node.Content[i] = pruneZero(item)
		}
	}
	return node
}

// isZero reports whether a value decodes to the zero value of whatever it is
// read into, so leaving it out of the file changes nothing.
func isZero(node *yaml.Node) bool {
	switch node.Kind {
	case yaml.ScalarNode:
		switch node.ShortTag() {
		case "!!null":
			return true
		case "!!str":
			return node.Value == ""
		case "!!bool":
			return strings.EqualFold(node.Value, "false")
		case "!!int", "!!float":
			return strings.Trim(node.Value, "+-0.") == ""
		}
	case yaml.SequenceNode, yaml.MappingNode:
		return len(node.Content) == 0
	}
	return false
}

// detectIndent returns the indentation a file already uses: the smallest one
// found on a line that holds something, or the default for a file without any.
func detectIndent(data []byte) int {
	indent := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if n := len(line) - len(trimmed); n > 0 && (indent == 0 || n < indent) {
			indent = n
		}
	}
	if indent < 2 || indent > 8 {
		return defaultIndent
	}
	return indent
}

// replaceFile writes data to path through a temporary file beside it, so an
// interrupted write leaves the old configuration rather than half of a new
// one. The permissions of the file being replaced are kept.
func replaceFile(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// restoreBlankLines puts back the blank lines of the original file, which a
// YAML parser does not keep: wherever two lines that a blank line separated
// are still next to each other, they are separated again.
func restoreBlankLines(original, rendered []byte) []byte {
	type pair struct{ before, after string }
	gaps := map[pair]int{}

	previous := ""
	blank := false
	for _, line := range strings.Split(string(original), "\n") {
		if strings.TrimSpace(line) == "" {
			blank = previous != ""
			continue
		}
		if blank {
			gaps[pair{previous, line}]++
		}
		previous, blank = line, false
	}
	if len(gaps) == 0 {
		return rendered
	}

	lines := strings.Split(string(rendered), "\n")
	out := make([]string, 0, len(lines)+len(gaps))
	for i, line := range lines {
		if i > 0 {
			key := pair{lines[i-1], line}
			if gaps[key] > 0 {
				gaps[key]--
				out = append(out, "")
			}
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}
