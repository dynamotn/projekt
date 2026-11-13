package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// useConfig installs a real config file: ReloadConfig drops anything set
// with SetTestConfig. It also keeps the jump history off the real one.
func useConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	prevCfg, prevHistory := lazypath.CfgFile, lazypath.HistoryFileOverride
	lazypath.CfgFile = path
	lazypath.HistoryFileOverride = filepath.Join(dir, "history.tsv")
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile, lazypath.HistoryFileOverride = prevCfg, prevHistory
		lazypath.ResetTestConfig()
		viper.Reset()
	})
	return path
}

// twoProjects makes two folders on disk, "api" tagged go and "web", and
// returns the config text and the folder of api.
func twoProjects(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	api, web := filepath.Join(root, "api"), filepath.Join(root, "web")
	for _, d := range []string{api, web} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return fmt.Sprintf("folders:\n  - path: %s\n    name: api\n    tags: [go]\n  - path: %s\n    name: web\n", api, web), api
}

// callText runs one tools/call through Serve.
func callText(t *testing.T, s *Server, name, args string) (string, bool) {
	t.Helper()
	m := roundTrip(t, s, call(name, args))
	res := m[0]["result"].(map[string]any)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	isErr, _ := res["isError"].(bool)
	return text, isErr
}

func decodeList(t *testing.T, text string) []map[string]any {
	t.Helper()
	var l []map[string]any
	if err := json.Unmarshal([]byte(text), &l); err != nil {
		t.Fatalf("not a JSON array: %q", text)
	}
	return l
}

func TestProjectList_MatchesTheCLI(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	var want bytes.Buffer
	if err := folderutil.ListFolders(&want, &folderutil.ListOption{Output: cli.OutputJSON}); err != nil {
		t.Fatal(err)
	}
	got, isErr := callText(t, NewServer(false), "project_list", `{}`)
	if isErr || got != want.String() {
		t.Fatalf("got %q (error %v), want %q", got, isErr, want.String())
	}
}

func TestProjectList_TagsAndQuery(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	s := NewServer(false)
	text, _ := callText(t, s, "project_list", `{"tags":["go"]}`)
	if l := decodeList(t, text); len(l) != 1 || l[0]["shortName"] != "api" {
		t.Fatalf("tags: %s", text)
	}
	text, _ = callText(t, s, "project_list", `{"query":"WE"}`)
	if l := decodeList(t, text); len(l) != 1 || l[0]["shortName"] != "web" {
		t.Fatalf("query: %s", text)
	}
	text, _ = callText(t, s, "project_list", `{"query":"zzz"}`)
	if l := decodeList(t, text); len(l) != 0 {
		t.Fatalf("no match: %s", text)
	}
}

func TestProjectShow(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	s := NewServer(false)
	text, isErr := callText(t, s, "project_show", `{"name":"api"}`)
	var info map[string]any
	if err := json.Unmarshal([]byte(text), &info); isErr || err != nil || info["name"] != "api" {
		t.Fatalf("got %q (error %v)", text, isErr)
	}
	if _, isErr := callText(t, s, "project_show", `{"name":"nope"}`); !isErr {
		t.Fatal("an unknown project must be an error")
	}
}

func TestProjectStatus_NeverAsksTheNetwork(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	s := NewServer(false)
	if _, isErr := callText(t, s, "project_status", `{"requests":true}`); !isErr {
		t.Fatal("requests must be an unknown argument")
	}
	text, isErr := callText(t, s, "project_status", `{}`)
	if isErr {
		t.Fatal(text)
	}
	decodeList(t, text)
	if _, isErr := callText(t, s, "project_status", `{"since":"soon"}`); !isErr {
		t.Fatal("a bad since must be an error")
	}
}

func TestProjectCurrent(t *testing.T) {
	cfg, api := twoProjects(t)
	useConfig(t, cfg)
	sub := filepath.Join(api, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewServer(false)
	text, isErr := callText(t, s, "project_current", fmt.Sprintf(`{"path":%q}`, sub))
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); isErr || err != nil || obj["shortName"] != "api" {
		t.Fatalf("got %q (error %v)", text, isErr)
	}
	if _, isErr := callText(t, s, "project_current", `{"path":"/"}`); !isErr {
		t.Fatal("a path outside every project must be an error")
	}
}

func TestTools_WrongArguments(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	s := NewServer(false)
	for args, mentions := range map[string]string{`{}`: "name", `{"name":5}`: "name", `{"name":"a","x":1}`: "x"} {
		if text, isErr := callText(t, s, "project_show", args); !isErr || !strings.Contains(text, mentions) {
			t.Fatalf("%s: got %q (error %v), want it to mention %q", args, text, isErr, mentions)
		}
	}
	// A call with no arguments key at all.
	m := roundTrip(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"project_show"}}`)
	res := m[0]["result"].(map[string]any)
	if text := res["content"].([]any)[0].(map[string]any)["text"].(string); res["isError"] != true || !strings.Contains(text, "name") {
		t.Fatalf("no arguments key: %v", res)
	}
	if _, isErr := callText(t, s, "project_list", `{}`); isErr {
		t.Fatal("project_list must still work")
	}
}

func TestTools_SeeAConfigEditedBetweenCalls(t *testing.T) {
	cfg, _ := twoProjects(t)
	path := useConfig(t, cfg)
	s := NewServer(false)
	if text, _ := callText(t, s, "project_list", `{}`); len(decodeList(t, text)) != 2 {
		t.Fatalf("before: %s", text)
	}
	extra := t.TempDir()
	if err := os.WriteFile(path, []byte(cfg+fmt.Sprintf("  - path: %s\n    name: extra\n", extra)), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, _ := callText(t, s, "project_list", `{}`); len(decodeList(t, text)) != 3 {
		t.Fatalf("after: %s", text)
	}
}

func TestTools_BrokenConfigIsReportedAndRecovers(t *testing.T) {
	cfg, _ := twoProjects(t)
	path := useConfig(t, cfg)
	s := NewServer(false)
	if err := os.WriteFile(path, []byte("folders: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, isErr := callText(t, s, "project_list", `{}`)
	if !isErr || !strings.Contains(text, path) {
		t.Fatalf("got %q (error %v)", text, isErr)
	}
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, isErr := callText(t, s, "project_list", `{}`); isErr {
		t.Fatalf("did not recover: %s", text)
	}
}

func TestTools_LoggingStaysOffTheWire(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	var out bytes.Buffer
	in := strings.NewReader(call("project_recent", `{}`) + "\n")
	if err := NewServer(false).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one line, got %q", out.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil || m["jsonrpc"] != "2.0" {
		t.Fatalf("not a JSON-RPC message: %q", lines[0])
	}
	text := m["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if strings.TrimSpace(text) != "[]" {
		t.Fatalf("text = %q", text)
	}
}

func TestNewServer_ReadOnlyByDefault(t *testing.T) {
	var names []string
	for _, tool := range NewServer(false).Tools {
		if !tool.ReadOnly {
			t.Errorf("%s is not read-only", tool.Name)
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{"project_current", "project_list", "project_recent", "project_show", "project_status"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestNewServer_WriteToolsOnlyWhenAsked(t *testing.T) {
	cfg, _ := twoProjects(t)
	useConfig(t, cfg)
	write := []string{"note_add", "describe_set", "worktree_add"}
	listed := func(s *Server) map[string]map[string]any {
		res := roundTrip(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)[0]["result"].(map[string]any)
		got := map[string]map[string]any{}
		for _, tool := range res["tools"].([]any) {
			m := tool.(map[string]any)
			got[m["name"].(string)] = m
		}
		return got
	}
	off := NewServer(false)
	for _, name := range write {
		if _, ok := listed(off)[name]; ok {
			t.Fatalf("%s listed without --write", name)
		}
	}
	for _, name := range write {
		m := roundTrip(t, off, call(name, `{}`))[0]
		if m["error"].(map[string]any)["code"] != float64(-32602) {
			t.Fatalf("%s callable without --write: %v", name, m)
		}
	}
	on := listed(NewServer(true))
	for _, name := range write {
		tool, ok := on[name]
		if !ok || tool["annotations"].(map[string]any)["readOnlyHint"] != false {
			t.Fatalf("%s: %v", name, tool)
		}
	}
}

func noteLines(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, folderutil.NoteFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestNoteAdd(t *testing.T) {
	cfg, api := twoProjects(t)
	useConfig(t, cfg)
	text, isErr := callText(t, NewServer(true), "note_add", `{"name":"api","text":"half done"}`)
	if isErr || !strings.Contains(text, "noted") {
		t.Fatalf("got %q (error %v)", text, isErr)
	}
	lines := noteLines(t, api)
	if want := "- " + time.Now().Format("2006-01-02") + ": half done"; lines[len(lines)-1] != want {
		t.Fatalf("lines = %q, want last %q", lines, want)
	}
}

func TestNoteAdd_OneLineAndANameRequired(t *testing.T) {
	cfg, api := twoProjects(t)
	useConfig(t, cfg)
	s := NewServer(true)
	if _, isErr := callText(t, s, "note_add", `{"name":"api","text":"first"}`); isErr {
		t.Fatal("first note failed")
	}
	before := len(noteLines(t, api))
	if _, isErr := callText(t, s, "note_add", `{"name":"api","text":"a\n- 2020-01-01: forged\r\n\r\nmore"}`); isErr {
		t.Fatal("multi-line text must be flattened, not refused")
	}
	lines := noteLines(t, api)
	if len(lines) != before+1 || strings.Count(strings.Join(lines, "\n"), "\n- 2020-01-01") != 0 {
		t.Fatalf("forged lines: %q", lines)
	}
	for _, args := range []string{`{"name":"","text":"x"}`, `{"name":"  ","text":"x"}`, `{"name":"api","text":" \n"}`} {
		if text, isErr := callText(t, s, "note_add", args); !isErr {
			t.Fatalf("%s accepted: %q", args, text)
		}
	}
	if got := len(noteLines(t, api)); got != before+1 {
		t.Fatalf("file changed on refused calls: %d lines", got)
	}
}

func TestDescribeSet(t *testing.T) {
	cfg, _ := twoProjects(t)
	path := useConfig(t, cfg)
	s := NewServer(true)
	if text, isErr := callText(t, s, "describe_set", `{"name":"api","description":"The API"}`); isErr || !strings.Contains(text, "described") {
		t.Fatalf("got %q (error %v)", text, isErr)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "The API") {
		t.Fatalf("config: %s", b)
	}
	for _, args := range []string{`{"name":"","description":"x"}`, `{"name":"api","description":" "}`} {
		if _, isErr := callText(t, s, "describe_set", args); !isErr {
			t.Fatalf("%s accepted", args)
		}
	}
}

func TestWorktreeAdd(t *testing.T) {
	cfg, api := twoProjects(t)
	useConfig(t, cfg)
	for _, args := range [][]string{
		{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "x"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", api}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	s := NewServer(true)
	text, isErr := callText(t, s, "worktree_add", `{"project":"api","branch":"next"}`)
	if isErr {
		t.Fatal(text)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(text), &got); err != nil || got["name"] != "api@next" {
		t.Fatalf("result %q: %v", text, err)
	}
	if _, err := os.Stat(got["path"]); err != nil {
		t.Fatal(err)
	}
	if show, _ := callText(t, s, "project_show", `{"name":"api"}`); !strings.Contains(show, got["path"]) {
		t.Fatalf("project_show does not list the working tree: %s", show)
	}
	if _, isErr := callText(t, s, "worktree_add", `{"project":" ","branch":"x"}`); !isErr {
		t.Fatal("a blank project must be refused")
	}
}
