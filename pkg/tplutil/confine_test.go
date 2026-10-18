package tplutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// secretOutside writes a file outside the store, the kind a malicious
// template would like to read, and returns its path.
func secretOutside(t *testing.T) string {
	t.Helper()
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return secret
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
}

func TestRender_RefusesAStoreLinkPointingOutside(t *testing.T) {
	store := useStore(t)
	secret := secretOutside(t)
	writeTemplate(t, filepath.Join(store, "leak", "README.md.tmpl"), "ok\n")
	mustSymlink(t, secret, filepath.Join(store, "leak", "leak.txt.tmpl"))

	tpl, err := Get("leak")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "proj")
	_, err = Render(RenderOptions{Template: tpl, Dest: dest, Name: "proj"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Render() error = %v, want the link refused", err)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "leak.txt")); !os.IsNotExist(statErr) {
		t.Error("Render() wrote the file the link points at")
	}
}

func TestRender_FollowsAStoreLinkThatStaysInside(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "shared.txt"), "shared\n")
	writeTemplate(t, filepath.Join(store, "ok", "README.md.tmpl"), "ok\n")
	mustSymlink(t, filepath.Join(store, "shared.txt"), filepath.Join(store, "ok", "copy.txt.tmpl"))

	tpl, err := Get("ok")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "proj")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "proj"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "copy.txt")); got != "shared\n" {
		t.Errorf("copy.txt = %q, want the linked file inside the store read", got)
	}
}

func TestInclude_RefusesALinkPointingOutside(t *testing.T) {
	store := useStore(t)
	secret := secretOutside(t)
	writeTemplate(t, filepath.Join(store, "inc", "README.md.tmpl"), `{{ include "key" }}`)
	mustSymlink(t, secret, filepath.Join(store, "inc", "key"))

	tpl, err := Get("inc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "proj"), Name: "proj"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Render() error = %v, want include to refuse the link", err)
	}
}

func TestShowTemplate_RefusesALinkPointingOutside(t *testing.T) {
	store := useStore(t)
	secret := secretOutside(t)
	mustSymlink(t, secret, filepath.Join(store, "show.tmpl"))

	tpl, err := Get("show")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	var out bytes.Buffer
	if err := ShowTemplate(&out, tpl); err == nil {
		t.Fatal("ShowTemplate() error = nil, want the link refused")
	}
	if strings.Contains(out.String(), "hunter2") {
		t.Error("ShowTemplate() printed the file the link points at")
	}
}

func TestRender_RefusesToWriteThroughALinkOutTheDestination(t *testing.T) {
	store := useStore(t)
	outside := t.TempDir()
	// `cfg` is written first as a link to a folder outside the destination;
	// the folder template `{{ "cfg" }}` sorts after it and renders to the
	// same name, so the file under it would land there.
	writeTemplate(t, filepath.Join(store, "esc", "symlink_cfg.tmpl"), outside)
	writeTemplate(t, filepath.Join(store, "esc", `{{ "cfg" }}`, "dropped.txt.tmpl"), "pwned\n")

	tpl, err := Get("esc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "proj")
	_, err = Render(RenderOptions{Template: tpl, Dest: dest, Name: "proj", Force: true})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("Render() error = %v, want the write through the link refused", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "dropped.txt")); !os.IsNotExist(statErr) {
		t.Error("Render() wrote a file outside the destination")
	}
}

func TestRender_DryRunRunsNoCommand(t *testing.T) {
	store := useStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	writeTemplate(t, filepath.Join(store, "touchy.tmpl"), `{{ output "touch" "`+marker+`" }}done`)

	tpl, err := Get("touchy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	var out bytes.Buffer
	if _, err := Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "x.txt"), DryRun: true, Out: &out}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Error("a dry run ran the template's command")
	}
	if !strings.Contains(out.String(), "done") {
		t.Errorf("dry run output = %q, want the rest of the template rendered", out.String())
	}
}

func TestRenderStringNoExec_RunsNothing(t *testing.T) {
	useStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	origin := Origin{Kind: "test", Name: "t", Key: t.TempDir()}
	got, err := RenderStringNoExec(origin, "line", `echo {{ output "touch" "`+marker+`" }}x`, nil)
	if err != nil {
		t.Fatalf("RenderStringNoExec() error = %v", err)
	}
	if got != "echo x" {
		t.Errorf("RenderStringNoExec() = %q", got)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Error("RenderStringNoExec() ran the command")
	}
}

func TestEnv_YourOwnStoreReadsTheEnvironment(t *testing.T) {
	store := useStore(t)
	resetApproved()
	t.Setenv("PROJEKT_TEST_TOKEN", "s3cret")
	writeTemplate(t, filepath.Join(store, "envy.tmpl"), `{{ env "PROJEKT_TEST_TOKEN" }}|{{ .Env.PROJEKT_TEST_TOKEN }}|{{ expandenv "$PROJEKT_TEST_TOKEN" }}`)

	tpl, err := Get("envy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "envy.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := readFile(t, dest); got != "s3cret|s3cret|s3cret" {
		t.Errorf("rendered %q, want the environment of a store of your own", got)
	}
}

func TestEnv_AnUntrustedCloneSeesNothing(t *testing.T) {
	store := useClonedStore(t)
	asked := answerTrust(t, false)
	t.Setenv("PROJEKT_TEST_TOKEN", "s3cret")
	writeTemplate(t, filepath.Join(store, "envy.tmpl"), `[{{ .Env.PROJEKT_TEST_TOKEN }}]`)
	writeTemplate(t, filepath.Join(store, "envf.tmpl"), `{{ env "PROJEKT_TEST_TOKEN" }}`)

	tpl, err := Get("envy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "envy.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := readFile(t, dest); got != "[]" {
		t.Errorf("rendered %q, want .Env empty for a clone nobody trusted", got)
	}
	if *asked != 0 {
		t.Errorf(".Env asked %d times; it must never ask", *asked)
	}

	tpl, err = Get("envf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "envf.txt")})
	if err == nil || !strings.Contains(err.Error(), "not trusted") {
		t.Fatalf("Render() error = %v, want env refused until trusted", err)
	}
	if *asked != 1 {
		t.Errorf("env asked %d times, want once", *asked)
	}
}

func TestEnv_ATrustedCloneReadsTheEnvironment(t *testing.T) {
	store := useClonedStore(t)
	t.Setenv("PROJEKT_TEST_TOKEN", "s3cret")
	writeTemplate(t, filepath.Join(store, "envy.tmpl"), `{{ .Env.PROJEKT_TEST_TOKEN }}|{{ env "PROJEKT_TEST_TOKEN" }}`)

	tpl, err := Get("envy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := Trust(TemplateOrigin(tpl)); err != nil {
		t.Fatalf("Trust() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "envy.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := readFile(t, dest); got != "s3cret|s3cret" {
		t.Errorf("rendered %q, want the environment once trusted", got)
	}
}

func TestSprig_HasNoHostLookup(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "dns.tmpl"), `{{ getHostByName "example.com" }}`)
	tpl, err := Get("dns")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "x")}); err == nil {
		t.Fatal("Render() error = nil, want getHostByName unknown")
	}
}

func TestInclude_RefusesALinkedFolderPointingOutside(t *testing.T) {
	store := useStore(t)
	outside := filepath.Dir(secretOutside(t))
	writeTemplate(t, filepath.Join(store, "incdir", "README.md.tmpl"), `{{ include "keys/secret.txt" }}`)
	mustSymlink(t, outside, filepath.Join(store, "incdir", "keys"))

	tpl, err := Get("incdir")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "proj"), Name: "proj"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Render() error = %v, want a file under a linked folder refused", err)
	}
}

func TestEnv_DryRunRendersNoValue(t *testing.T) {
	store := useStore(t)
	t.Setenv("PROJEKT_TEST_TOKEN", "s3cret")
	writeTemplate(t, filepath.Join(store, "envdry.tmpl"), `[{{ env "PROJEKT_TEST_TOKEN" }}|{{ expandenv "x$PROJEKT_TEST_TOKEN" }}|{{ .Env.PROJEKT_TEST_TOKEN }}]`)

	tpl, err := Get("envdry")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	var out bytes.Buffer
	if _, err := Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "x"), DryRun: true, Out: &out}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if out.String() != "[|x|]" {
		t.Errorf("dry run rendered %q, want no value of the environment", out.String())
	}
}
