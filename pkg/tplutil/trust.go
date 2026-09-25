package tplutil

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// Origin is where a command comes from: the template, recipe or repository
// whose text asked for it.
//
// A template runs commands in two ways, its `after` hooks and the `output`
// function, and both are code somebody else may have written: a store cloned
// with `t init` changes under you on every `t sync`. So a command runs only
// when its origin is one you wrote yourself — nothing in it comes from a
// repository with a remote — or one whose exact content you trusted.
type Origin struct {
	// Kind and Name say what it is in a message: the template go-cli.
	Kind, Name string
	// Key is what a trust decision is recorded under. It defaults to the
	// first path.
	Key string
	// Paths are the files and folders whose content is being trusted. A path
	// that does not exist is simply not part of it.
	Paths []string
	// Hint is the command that trusts this origin, for the message that says
	// it is not.
	Hint string
}

// TemplateOrigin is the origin of what a template of the store runs: the
// template itself and the shared pieces of the store it can call.
func TemplateOrigin(tpl Template) Origin {
	if tpl.Path == "" {
		return Origin{}
	}
	paths := []string{tpl.Path}
	if dir, err := Dir(); err == nil {
		paths = append(paths, filepath.Join(dir, PartialsDir))
	}
	return Origin{Kind: "template", Name: tpl.Name, Key: tpl.Path, Paths: paths, Hint: "t trust " + tpl.Name}
}

func (o Origin) key() string {
	if o.Key != "" {
		return o.Key
	}
	if len(o.Paths) > 0 {
		return o.Paths[0]
	}
	return ""
}

func (o Origin) String() string {
	return strings.TrimSpace(o.Kind + " " + o.Name)
}

// UntrustedError is returned for a command whose origin is not trusted.
type UntrustedError struct {
	Origin  Origin
	Command string
}

func (e *UntrustedError) Error() string {
	how := "run it from a terminal to be asked"
	if e.Origin.Hint != "" {
		how = fmt.Sprintf("review it, then run `%s`", e.Origin.Hint)
	}
	return fmt.Sprintf("not running %q: the %s comes from a repository and this version of it is not trusted; %s",
		e.Command, e.Origin, how)
}

// AskTrust asks whether to trust an origin before the command runs. It says
// no without asking when nobody is at a terminal to answer.
//
// A variable so a test can answer instead.
var AskTrust = askOnTerminal

var (
	trustMu sync.Mutex
	// approved remembers the origins allowed in this process, so a template
	// calling `output` in ten files asks once.
	approved = map[string]bool{}
)

// Authorize returns nil when command, asked for by origin, may run.
func Authorize(origin Origin, command string) error {
	key := origin.key()
	if key == "" {
		// Every caller says where its text comes from; one that does not is a
		// bug, and a bug is no reason to run something.
		return fmt.Errorf("not running %q: nothing says where it comes from", command)
	}

	trustMu.Lock()
	defer trustMu.Unlock()

	if approved[key] {
		return nil
	}
	if !origin.fromRemote() {
		approved[key] = true
		return nil
	}

	sum, err := origin.fingerprint()
	if err != nil {
		return err
	}
	if trusted, err := isTrusted(key, sum); err != nil {
		return err
	} else if trusted {
		approved[key] = true
		return nil
	}

	yes, err := AskTrust(origin, command)
	if err != nil {
		return err
	}
	if !yes {
		return &UntrustedError{Origin: origin, Command: command}
	}
	if err := recordTrust(key, sum); err != nil {
		return err
	}
	approved[key] = true
	return nil
}

// Trust records the origin as trusted as it is now, whatever it holds.
func Trust(origin Origin) error {
	key := origin.key()
	if key == "" {
		return errors.New("nothing to trust")
	}
	sum, err := origin.fingerprint()
	if err != nil {
		return err
	}

	trustMu.Lock()
	defer trustMu.Unlock()
	if err := recordTrust(key, sum); err != nil {
		return err
	}
	approved[key] = true
	return nil
}

// resetApproved forgets what this process allowed, for a test.
func resetApproved() {
	trustMu.Lock()
	defer trustMu.Unlock()
	approved = map[string]bool{}
}

// fromRemote reports whether any of the origin's paths sits in a git
// repository with a remote, which is how content reaches it that you did not
// write. A folder outside any repository, or in one that only lives here, is
// yours.
func (o Origin) fromRemote() bool {
	for _, path := range o.Paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		dir := path
		if !info.IsDir() {
			dir = filepath.Dir(path)
		}
		out, err := exec.Command("git", "-C", dir, "remote").Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return true
		}
	}
	return false
}

// fingerprint hashes everything the origin holds, so that trust given to one
// version of it does not carry over to the next.
func (o Origin) fingerprint() (string, error) {
	hash := sha256.New()
	for i, root := range o.Paths {
		info, err := os.Stat(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			if err := hashEntry(hash, fmt.Sprintf("%d", i), root, info.Mode()); err != nil {
				return "", err
			}
			continue
		}

		var files []string
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == ".git" {
				return fs.SkipDir
			}
			if !entry.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		sort.Strings(files)
		for _, path := range files {
			info, err := os.Lstat(path)
			if err != nil {
				return "", err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return "", err
			}
			if err := hashEntry(hash, fmt.Sprintf("%d/%s", i, filepath.ToSlash(rel)), path, info.Mode()); err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// hashEntry adds one file to a fingerprint: its name, whether it runs, and
// what it holds — or, for a link, where it points.
func hashEntry(hash io.Writer, name, path string, mode fs.FileMode) error {
	fmt.Fprintf(hash, "%s\x00%t\x00", name, mode&0o111 != 0)
	if mode&fs.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "link:%s\x00", target)
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	_, err = hash.Write([]byte{0})
	return err
}

// trustRecord is one trusted origin, as the trust file stores it.
type trustRecord struct {
	Key       string    `yaml:"key"`
	SHA256    string    `yaml:"sha256"`
	TrustedAt time.Time `yaml:"trustedAt"`
}

// TrustFile is where trust decisions are kept: state, not configuration, so
// it stays out of the dotfiles the configuration lives in.
func TrustFile() string {
	return filepath.Join(lazypath.StateHome(), "projekt", "trust.yaml")
}

func readTrust() ([]trustRecord, error) {
	data, err := os.ReadFile(TrustFile())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []trustRecord
	if err := yaml.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", TrustFile(), err)
	}
	return records, nil
}

func isTrusted(key, sum string) (bool, error) {
	records, err := readTrust()
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.Key == key && record.SHA256 == sum {
			return true, nil
		}
	}
	return false, nil
}

// recordTrust keeps one record per key: trusting a new version of an origin
// replaces the old one rather than trusting both.
func recordTrust(key, sum string) error {
	records, err := readTrust()
	if err != nil {
		return err
	}
	kept := records[:0]
	for _, record := range records {
		if record.Key != key {
			kept = append(kept, record)
		}
	}
	kept = append(kept, trustRecord{Key: key, SHA256: sum, TrustedAt: time.Now().UTC()})

	data, err := yaml.Marshal(kept)
	if err != nil {
		return err
	}
	path := TrustFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".trust-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	cli.Debug("Trusted %s (%s)", key, sum)
	return os.Rename(temp.Name(), path)
}

// askOnTerminal is the question AskTrust asks by default.
func askOnTerminal(origin Origin, command string) (bool, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "The %s wants to run:\n  %s\n"+
		"It comes from a repository, and you have not trusted this version of it.\n"+
		"Trust it and run its commands? [y/N] ", origin, command)

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
