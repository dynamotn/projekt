package tplutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// RecordDir and RecordFile are where a project remembers what it was rendered
// from.
//
// It lives in the project rather than in the configuration, so that cloning
// the project brings it along: the next person to run `t apply` gets the same
// answer as the person who created it.
const (
	RecordDir  = ".projekt"
	RecordFile = "template.yaml"
	// RecordVersion is the shape of the file, so a later projekt can read an
	// earlier project.
	RecordVersion = 1
)

// RenderRecord is one template a project was rendered from.
type RenderRecord struct {
	// Template is the store entry that was rendered.
	Template string `yaml:"template"`
	// Name is the `.Name` it was rendered with.
	Name string `yaml:"name,omitempty"`
	// RenderedAt is when it last ran, in UTC.
	RenderedAt time.Time `yaml:"renderedAt"`
	// Values are the values it ran with, so `t apply` can replay them without
	// being told again.
	Values Values `yaml:"values,omitempty"`
	// Files maps each written path, relative to the project, to the hash of
	// what was written. A file whose hash still matches has not been touched
	// since, and can be replaced without asking.
	Files map[string]string `yaml:"files"`
}

// Record is everything a project was rendered from.
type Record struct {
	Version int            `yaml:"version"`
	Renders []RenderRecord `yaml:"renders"`
}

// recordPath returns where a project keeps its record.
func recordPath(project string) string {
	return filepath.Join(project, RecordDir, RecordFile)
}

// LoadRecord reads what a project was rendered from.
//
// A project without a record is the normal case for anything created before
// there was one, and comes back empty rather than as an error.
func LoadRecord(project string) (Record, error) {
	path := recordPath(project)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Record{Version: RecordVersion}, nil
		}
		return Record{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var record Record
	if err := yaml.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	if record.Version > RecordVersion {
		return Record{}, fmt.Errorf("%s was written by a newer projekt (version %d)", path, record.Version)
	}
	return record, nil
}

// SaveRecord writes a project's record, replacing whatever was there.
func SaveRecord(project string, record Record) error {
	record.Version = RecordVersion
	sort.Slice(record.Renders, func(i, j int) bool {
		return record.Renders[i].Template < record.Renders[j].Template
	})

	data, err := yaml.Marshal(record)
	if err != nil {
		return fmt.Errorf("cannot write the record of %s: %w", project, err)
	}

	path := recordPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	cli.Debug("Recorded %s", path)
	return nil
}

// Find returns what a project remembers about one template.
func (r Record) Find(template string) (RenderRecord, bool) {
	for _, render := range r.Renders {
		if render.Template == template {
			return render, true
		}
	}
	return RenderRecord{}, false
}

// put replaces what is remembered about a template, or adds it.
func (r *Record) put(render RenderRecord) {
	for i, existing := range r.Renders {
		if existing.Template == render.Template {
			r.Renders[i] = render
			return
		}
	}
	r.Renders = append(r.Renders, render)
}

// recordRender remembers one render in the project it wrote to.
//
// A record that cannot be written is worth saying so: the files are there
// either way, but `t apply` will not know about them.
func recordRender(project string, render RenderRecord) error {
	record, err := LoadRecord(project)
	if err != nil {
		return err
	}
	record.put(render)
	return SaveRecord(project, record)
}

// hashOf is how a file is recognised as untouched later.
func hashOf(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// hashFile returns the hash of what is on disk, or an empty string when there
// is nothing there.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read %s: %w", path, err)
	}
	return hashOf(data), nil
}
