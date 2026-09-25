# Templates

`t` (also reachable as `projekt template`) renders [Go
templates](https://pkg.go.dev/text/template) from a folder of templates you
own, with the [sprig](https://masterminds.github.io/sprig/) function set
available on top of the standard ones.

The repository ships a starter set in [../examples/templates](../examples/templates),
covering developer, devops, devsecops, security, note-taking and finance files.
Try it without installing anything:

```bash
t --template-dir examples/templates list
t --template-dir examples/templates new invoice ./INV-001.md -f examples/values/invoice.yaml
```

## The template folder

Templates live in `$XDG_DATA_HOME/projekt/templates`
(`~/.local/share/projekt/templates` by default). Use `--template-dir` or the
`PROJEKT_TEMPLATE_DIR` environment variable to point somewhere else, for
example a folder you keep in a dotfiles repository.

```bash
t path              # where the templates are
t path license      # where one template is, handy for $EDITOR (t path license)
```

### Carrying the store between machines

A store is a folder of files, so it can simply be a repository — which is how
the same templates reach your other machine, and how a team shares one set:

```bash
t init git@github.com:me/templates.git   # clone it into the template folder
t init github:me/templates               # or the server:group/name shorthand
t sync                                   # bring it up to date later
t sync --dry-run                         # say what would be pulled
```

`t init` clones with the history, so a template can be edited and pushed back.
It refuses a folder that already holds templates rather than clone over them,
and a store that is already a repository is `t sync`'s business.

`t sync` pulls fast-forward only: a store you have edited without committing is
something to sort out by hand, not something for a sync to guess at. A folder
that merely *sits inside* some other checkout is not a store to sync — pulling
it would pull that checkout — and `t sync` says so instead.

Each entry of that folder is one template:

- a **file** template renders one file. A `.tmpl` suffix is stripped from the
  name, so `license.tmpl` is the template `license` and renders to `license`.
- a **folder** template renders a whole tree. Every file is rendered, and so is
  every path segment, so a folder named `{{ .Name }}` becomes the project name.

Entries starting with a dot are ignored in the store, so editor and VCS
leftovers never become a template. Inside a folder template a dotfile *is* part
of what a project needs — `.gitignore`, `.github` — and is written like any
other file, except for the four names that describe the template rather than
belong to it: `.vars.yaml` (what to ask), `.data.yaml` (what is already known),
`.ignore` (what not to write) and the `.templates` folder (what is shared).

```
~/.local/share/projekt/templates
├── license.tmpl                       # t new license
└── go-cli                             # t new go-cli
    ├── go.mod.tmpl
    └── cmd
        └── {{ .Name }}
            └── main.go.tmpl
```

## Rendering

```bash
# A file template, into the current folder or a path you name
t new license LICENSE --set author='Jane Doe'

# A folder template: --name feeds both the path segments and .Name
t new go-cli ./myapp --name myapp --set module=example.com/myapp

# See the result before it touches the disk
t new dockerfile --dry-run
```

`t new` prints every file it wrote. It never overwrites an existing file
unless `--force` is given, and a path segment that renders to an empty name, a
`..` or anything containing a separator is refused, so a value can never make a
template write outside the destination.

## Values

User values are reachable through `.Values`:

```bash
t new license LICENSE --set author.name='Jane Doe' --set year=2026
t new go-cli ./myapp --values company.yaml --values myapp.yaml --set name=myapp
```

- `--set key=value` is repeatable. Dots nest, so `--set author.name=me` is the
  same as `author: {name: me}`. Only the first `=` separates the key, which
  keeps URLs and expressions intact.
- The value is read as a YAML scalar, so `--set port=8080` is a number and
  `--set debug=true` a boolean; anything else stays a string.
- `--values file.yaml` is repeatable too. Files are merged in order, deeply,
  and `--set` wins over all of them.

Besides `.Values`, a template is given:

| Variable    | Content                                                        |
|-------------|----------------------------------------------------------------|
| `.Name`     | Output name, from `--name` or the destination file name         |
| `.Project`  | Folder the files are written to, or `.Name` for a folder template |
| `.Dir`      | Absolute destination folder                                     |
| `.Path`     | Absolute path of the file being rendered                        |
| `.Template` | Name of the template                                            |
| `.Source`   | Folder the template itself lives in                             |
| `.Store`    | Template folder the store is read from                          |
| `.User`     | Current user name, with `.Home` besides it                      |
| `.Hostname` | Name of this machine, with `.OS` and `.Arch` besides it         |
| `.Env`      | The environment, as `.Env.EDITOR`                               |
| `.Now`      | Current time, with `.Date` (`2006-01-02`) and `.Year` besides it |

A value that was never set does not fail the render: it comes out as
`<no value>`, which is why `{{ .Values.license | default "MIT" }}` is the way
to make one optional. In a *path segment* it is refused outright — a folder
called `<no value>` is never what anyone meant — so a template that names its
files after a value says so the moment it is run.

## Answering questions instead of passing flags

`--interactive` (`-i`) asks for what is missing, one question per value:

```console
$ t new invoice ./INV-001.md -i
Values for invoice:
  Invoice number [INV-2026-001]: INV-2026-021
  Who is invoicing [jane]:
  Who is billed: Acme GmbH
  currency (EUR/USD/GBP/VND) [EUR]:
  Payment terms, in days [14]: 30
  Tax rate, in percent [0]: 19
```

- An empty answer takes the default in brackets, and a value with no default
  stays unset, so the template's own `| default` still applies.
- Anything already given with `--set` or `--values` is never asked again, which
  makes `-i` a way to fill in the rest rather than all of it.
- An answer of the wrong shape is refused and asked again — `8o8o` for a number,
  or a choice that is not on the list.
- Questions go to standard error, so `t new x -i --dry-run > file` still writes
  only the rendered template to the file.
- Answers can be piped in, one line each. When the input runs out the remaining
  values fall back to their defaults, and a required one without an answer is
  an error rather than an empty file.

### Saying what to ask

A template declares its questions in a `.vars.yaml`: next to a file template as
`<name>.vars.yaml`, and inside a folder template as `.vars.yaml`, where it
travels with the folder. It is never listed as a template and never rendered.

```yaml
vars:
  - name: module              # the key under .Values; dots nest
    prompt: Go module path    # the question; defaults to the name
    default: "example.com/{{ .Name }}"   # rendered, so .Name and .User work
    required: true            # ask again rather than accept an empty answer
  - name: runner
    type: choice
    choices: [ubuntu-latest, macos-latest, windows-latest]
    default: ubuntu-latest
  - name: netDays
    type: int
    default: "14"
  - name: publishCoverage
    type: bool                # y/yes/true/1, n/no/false/0
    default: "no"
  - name: tags
    type: list                # comma separated
    default: inbox
```

A template without a manifest is still usable interactively: the questions are
then the `.Values` keys read out of the template itself, in the order they
appear. Keys the template loops over are left out — a list or a map is not
something to type at a prompt, and belongs in a `--values` file.

### Running something afterwards

A rendered project is rarely finished: a Go project wants `go mod tidy`, a
repository wants `git init`, a hook config wants installing. The manifest says
so, and `t new` runs them in the folder it just wrote:

```yaml
# templates/go-cli/.vars.yaml
after:
  - '{{ if lookPath "go" }}go mod tidy{{ end }}'
  - '{{ if .Values.git }}git init -q && git add -A{{ end }}'
```

Each line is rendered with the values first, so a command can ask whether it is
worth running, and a line that renders empty is skipped rather than run. They
go through a shell, so a pipe or an `&&` works, and they run with
`PROJEKT_NAME`, `PROJEKT_PATH` and `PROJEKT_TEMPLATE` in the environment.

Each command is printed before it runs — a hook that runs out of sight is one
nobody can debug — and the first failure stops the rest, because the second
command usually assumes the first one worked. `--dry-run` describes them
without running them, and `--no-hooks` skips them:

```bash
t new go-cli ./myapp --no-hooks
```

`b` runs them too, before the recipe's own `after:`, because they belong to the
template rather than to the recipe.

### Delimiters of your own

A template that writes GitHub Actions expressions, a Helm chart or another Go
template spends its life escaping the syntax it is written in. The manifest can
move the delimiters out of the way instead:

```yaml
# templates/chart/.vars.yaml
delims: ["<%", "%>"]
vars:
  - name: registry
    default: ghcr.io
```

```gotemplate
name: <% .Name %>
image: {{ .Values.image }}     # left exactly as it is
```

The pair applies to the whole template — file contents, path segments, the
`.ignore` and the defaults in the manifest. The shared templates of
`.templates` keep the usual `{{ }}`: they belong to the store, not to the
template calling them, so one fragment stays usable from templates that spell
their delimiters differently.

## Values a template already has

Some values are not worth typing twice: your name, your company, the licence
you always pick. A `.data.yaml` holds them.

```
~/.local/share/projekt/templates
├── .data.yaml                # every template gets these
├── license.tmpl
├── license.data.yaml         # and the licence template these
└── go-cli
    └── .data.yaml            # and this folder template these
```

```yaml
# ~/.local/share/projekt/templates/.data.yaml
author: Jane Doe
company: Acme GmbH
license: MIT
```

They are read lowest first — the store, then the template — and everything
given on the command line goes on top, so a data file is a default and never an
override:

```
store .data.yaml  <  template .data.yaml  <  --values  <  --set  <  --interactive
```

A value a data file already holds is never asked for again, which is what makes
one worth writing: `t new license LICENSE -i` then asks only for what is
particular to this file. `.data.yml` and `.data.json` are read too, for a file
generated from somewhere else.

## Pieces several templates share

A `.templates` folder holds the fragments templates call rather than repeat: a
licence header, a CI job, a block of Makefile. One at the root of the store is
reachable from every template; one inside a folder template travels with it and
wins when both define the same name.

```
~/.local/share/projekt/templates
├── .templates
│   ├── header.tmpl             # {{ template "header" . }}
│   └── ci/go.tmpl              # {{ template "ci/go" . }}
└── go-cli
    └── .templates/header.tmpl  # go-cli's own header
```

A fragment is called the way text/template calls one, and `includeTemplate`
renders it into a string when it needs piping:

```gotemplate
{{ template "header" . }}

{{ includeTemplate "ci/go" . | indent 4 }}
```

The folder is never listed as a template and never written to the output.

## Leaving files out

A folder template writes every file it holds. A `.ignore` says which ones it
does not — and because it is rendered like everything else, that decision can
be made from the values:

```gotemplate
# templates/go-cli/.ignore
{{ if not .Values.ci }}.github/{{ end }}
{{ if ne .Values.license "MIT" }}LICENSE{{ end }}
*.local
```

The patterns read the way a `.gitignore` does: `#` starts a comment, a trailing
`/` matches a folder, a leading or inner `/` anchors the pattern at the root of
the output, `*` matches inside a segment and `**` any number of them, and a `!`
line brings back what an earlier one dropped. They are matched against the
paths the template *would have written*, after the names were rendered, and an
ignored folder takes its contents with it.

This is how one template covers the variants of a project, instead of four
templates that drift apart.

## What a file is

A template says what is *in* a file. A prefix on its name says what the file
*is*:

| Prefix | What the rendered file becomes |
| --- | --- |
| `executable_` | runnable — mode `755`, or `700` with `private_` |
| `private_` | readable by its owner alone — mode `600`, a folder `700` |
| `readonly_` | not writable — the write bits are dropped |
| `symlink_` | a symbolic link, pointing at whatever the file rendered to |
| `dot_` | a dotfile: `dot_gitignore` is written as `.gitignore` |
| `literal_` | nothing — it stops the reading, for a file really called `executable_x` |

They combine, and they apply to folders as well as files:

```
templates/go-cli
├── executable_scripts/executable_{{ .Name }}.sh.tmpl   # scripts/myapp.sh, 755
├── private_dot_env.tmpl                                # .env, 600
└── symlink_latest.tmpl                                 # a link to what it rendered
```

The prefixes are read off the template's own name *before* it is rendered, so a
value can never turn a file into an executable or a link by accident:
`--set file=executable_run.sh` writes a plain file honestly called
`executable_run.sh`.

## Asking while rendering

`.vars.yaml` asks everything up front. A template can also ask at the point it
needs the answer, which suits a question only one branch of the template
reaches:

```gotemplate
module {{ promptString "Go module path" (printf "example.com/%s" .Name) }}

{{ if promptBool "Add a Dockerfile" "no" }}...{{ end }}
{{ promptChoice "Licence" (list "MIT" "Apache-2.0" "BSD-3-Clause") "MIT" }}
{{ promptInt "Port" 8080 }}
```

The answer is remembered for the whole render, so the same question asked by
ten files is asked once. Without `--interactive` the default is taken instead,
and a question with no default is then an error rather than an empty file, so a
scripted run cannot quietly write the wrong thing.

## Functions

On top of the [sprig](https://masterminds.github.io/sprig/) set:

| Function | What it does |
| --- | --- |
| `include "path"` | the contents of a file of the template, unrendered |
| `includeTemplate "name" .` | a shared template of `.templates`, rendered into a string |
| `output "cmd" "arg"` | the standard output of a command |
| `lookPath "git"` | where an executable is, or empty when it is not installed |
| `stat "go.mod"` | what is at a path, or nothing — `{{ if stat "go.mod" }}` |
| `joinPath "a" "b"` | `a/b` |
| `toYaml` / `fromYaml` | a value as YAML, and YAML back as a value |
| `promptString` / `promptInt` / `promptBool` / `promptChoice` | ask, as above |

`include` reads relative to the template and refuses to leave it, so a template
cannot be made to read a file somewhere else on the disk.

## Checking a template

A template now carries four files that describe it and a folder of shared
pieces, so there is more to get wrong than there used to be. `t check` reads
one the way rendering would, and reports everything, rather than the first
thing:

```bash
t check              # every template of the store
t check go-cli       # one of them
t check --strict     # warnings count as errors too
```

It writes nothing and exits non-zero when it found an error, so it can gate a
CI job on a store of templates staying usable.

**Errors** — the template will not render:

- a file or a path segment that does not parse, with the template's own
  delimiters
- `{{ template "x" }}` or `includeTemplate "x"` naming a shared template that
  is in no `.templates` folder
- a `.vars.yaml`, `.data.yaml` or `.ignore` that does not parse
- a `delims` that is not a usable pair
- a path segment that renders to nothing, to a value nobody set, or to a name
  that would escape the destination

**Warnings** — it renders, but probably not as meant:

- a variable the manifest asks for that the template never reads
- a value the template reads that the manifest never asks for, so
  `--interactive` will not offer it. A value the template already handles the
  absence of — behind a `default` or a `with` — is optional by design and not
  reported, and neither is a list or a map, which belongs in a `--values` file
  rather than at a prompt. A value a `.data.yaml` already holds counts as
  answered
- a default that is not valid for the type its variable declares
- an `.ignore` that leaves every single file out

## Applying a template again

`t new` writes a project once. A template is not written once: the CI job gets
a step, the Makefile gets a target, the licence header changes. `t apply`
writes the project again, which is how that reaches the projects already made
from it.

```bash
t diff                           # what would change, as a unified diff
t diff -C ./myapp                # somewhere other than here
t apply                          # do it
t apply go-cli --set ci=true     # one template, one answer changed
```

Named no template, it applies **every** template the project records, so it
needs to be told nothing about a project to bring it up to date. That is what
makes the fleet-wide question answerable with what `projekt` already has:

```bash
projekt folder exec -t work -- t diff     # which of my projects have drifted
projekt folder exec -t work -- t apply    # bring them all up to date
```

`folder exec` exits non-zero when the command failed anywhere, and `t diff`
exits non-zero when it found something, so the first of those is a check a CI
job can run over every project at once.

### What the project remembers

`t new` leaves a record in the project, at `.projekt/template.yaml`:

```yaml
version: 1
renders:
  - template: go-cli
    name: myapp
    renderedAt: 2026-09-24T09:54:52Z
    values:
      module: example.com/myapp
      ci: true
    files:
      Makefile: sha256:80d32196…
      cmd/myapp/main.go: sha256:0d0ccec0…
```

It lives in the project rather than in your configuration, so cloning the
project brings it along and a colleague's `t apply` reaches the same answer as
yours. Commit it.

Two things come out of it. The **values** are replayed, so applying needs no
flags — `--set` and `--values` change one answer and keep the rest. And the
**hashes** are what tell an out-of-date file apart from one somebody edited:

| Status | What it means | What `apply` does |
| --- | --- | --- |
| `added` | the template writes it, the project has not got it | writes it |
| `updated` | the template writes it differently, and it is byte for byte what was written last time | writes it |
| `unchanged` | it is already what the template says | nothing |
| `conflict` | it differs *and* it was edited since | keeps it, unless `--force` |
| `removed` | the template no longer writes it | keeps it, unless `--prune` |

Nothing that was edited by hand is touched without `--force`, nothing is
deleted without `--prune`, and the template's `after` commands are not run
again without `--hooks`: they ran when the project was created, and repeating
a `git init` unasked is how an apply loses somebody's trust. A file left alone stays in the record exactly as it
was written, so the next apply reaches the same conclusion instead of
forgetting the file.

A project with no record at all — one created before there was one — is not a
problem: every file it already has comes out as a `conflict` and is left alone,
so the first apply only fills in what is missing.

### Gating on it

`t diff` changes nothing and exits non-zero when there is something to do,
which is what a CI job wants:

```bash
t diff || echo "this project has drifted from its templates"
t diff --name-only                 # the paths alone, for a script
t diff -U0                         # no context around the changes
t diff go-cli                      # only one of the project's templates
```

Once a project records more than one template, each line says which one the
change comes from.

## Writing a template

The quickest start is to import a file or a folder you already have; its
content is copied as it is, so any template action already in it is kept.

```bash
t add ./LICENSE                  # becomes the template `LICENSE`
t add ./myapp --name go-cli      # a whole folder, minus its .git
t add ./Makefile --force         # replace a template of the same name
```

Then edit it in place:

```bash
$EDITOR (t path go-cli)          # fish
$EDITOR "$(t path go-cli)"       # bash
```

`t list` shows what the store holds and `t show <template>` prints the source
of one, a folder template file by file. To see what a template will ask for,
run it with `-i --dry-run`. The examples folder is the other way in:
copy what you want out of it, or read it for what a template can do. Like `projekt folder list`, the listing
renders in the format you ask for:

```bash
t list                                  # bordered table (default)
t list -o json | jq -r '.[].path'       # array of objects
t list -o tsv --names-only --no-headers # one template name per line
```
