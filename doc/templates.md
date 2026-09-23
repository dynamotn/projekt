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

Each entry of that folder is one template:

- a **file** template renders one file. A `.tmpl` suffix is stripped from the
  name, so `license.tmpl` is the template `license` and renders to `license`.
- a **folder** template renders a whole tree. Every file is rendered, and so is
  every path segment, so a folder named `{{ .Name }}` becomes the project name.

Entries starting with a dot are ignored, in the store and inside a folder
template, so editor and VCS leftovers never end up in the output.

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
| `.User`     | Current user name                                               |
| `.Now`      | Current time, with `.Date` (`2006-01-02`) and `.Year` besides it |

A value that was never set renders as empty rather than failing, so
`{{ .Values.license | default "MIT" }}` is the way to make one optional.

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
