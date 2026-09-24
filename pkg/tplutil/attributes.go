package tplutil

import (
	"io/fs"
	"strings"
)

// Attribute prefixes say what a rendered file *is*, where the template itself
// can only say what is in it.
//
// They are read off the front of a name in the template, before it is
// rendered, so a value can never turn a file into an executable by accident:
// `executable_{{ .Name }}.sh` renders to `myapp.sh`, mode 0755.
const (
	// AttrExecutable makes the file runnable.
	AttrExecutable = "executable_"
	// AttrPrivate keeps the file to its owner.
	AttrPrivate = "private_"
	// AttrReadonly takes the write bit away.
	AttrReadonly = "readonly_"
	// AttrSymlink writes a symbolic link whose target is what the template
	// rendered.
	AttrSymlink = "symlink_"
	// AttrDot renames the file to a dotfile, for a store or a repository that
	// would rather not carry hidden files.
	AttrDot = "dot_"
	// AttrLiteral stops the reading, so a file really called `executable_x`
	// can be written as `literal_executable_x`.
	AttrLiteral = "literal_"
)

// Attributes are what the prefixes of one name asked for.
type Attributes struct {
	// Executable, Private and Readonly change the mode the file is written
	// with.
	Executable bool
	Private    bool
	Readonly   bool
	// Symlink writes a link instead of a file.
	Symlink bool
}

// ParseAttributes strips the attribute prefixes off a name and reports what
// they asked for. A name with none of them comes back unchanged.
func ParseAttributes(name string) (string, Attributes) {
	var attrs Attributes
	for {
		switch {
		case strings.HasPrefix(name, AttrLiteral):
			return strings.TrimPrefix(name, AttrLiteral), attrs
		case strings.HasPrefix(name, AttrExecutable):
			attrs.Executable = true
			name = strings.TrimPrefix(name, AttrExecutable)
		case strings.HasPrefix(name, AttrPrivate):
			attrs.Private = true
			name = strings.TrimPrefix(name, AttrPrivate)
		case strings.HasPrefix(name, AttrReadonly):
			attrs.Readonly = true
			name = strings.TrimPrefix(name, AttrReadonly)
		case strings.HasPrefix(name, AttrSymlink):
			attrs.Symlink = true
			name = strings.TrimPrefix(name, AttrSymlink)
		case strings.HasPrefix(name, AttrDot):
			name = "." + strings.TrimPrefix(name, AttrDot)
			// A dotfile is a name, not a mode: nothing else can follow it.
			return name, attrs
		default:
			return name, attrs
		}
	}
}

// Any reports whether the name carried an attribute at all.
func (a Attributes) Any() bool {
	return a.Executable || a.Private || a.Readonly || a.Symlink
}

// FileMode is the mode a rendered file is written with.
func (a Attributes) FileMode() fs.FileMode {
	mode := fs.FileMode(0o644)
	if a.Private {
		mode = 0o600
	}
	if a.Executable {
		mode |= (mode & 0o444) >> 2
	}
	if a.Readonly {
		mode &^= 0o222
	}
	return mode
}

// DirMode is the mode a folder carrying attributes is created with.
func (a Attributes) DirMode() fs.FileMode {
	mode := fs.FileMode(0o755)
	if a.Private {
		mode = 0o700
	}
	if a.Readonly {
		mode &^= 0o222
	}
	return mode
}
