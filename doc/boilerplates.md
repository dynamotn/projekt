# Boilerplates

`b` (also reachable as `projekt boilerplate`) creates a whole project from a
recipe, and leaves it registered in your configuration, so the last step of
starting something new is `pj <name>` rather than another `cd`.

Where `t` writes files into a project, `b` creates the project: it renders the
files, works out where they belong, and tells projekt about the result.

## The boilerplate folder

Recipes live in `$XDG_DATA_HOME/projekt/boilerplates`
(`~/.local/share/projekt/boilerplates` by default), one YAML file each, named
after the recipe. Use `--boilerplate-dir` or the `PROJEKT_BOILERPLATE_DIR`
environment variable to point somewhere else.

```bash
b list                  # what the store holds
b show go-cli           # the recipe as it is written
b path go-cli           # where it lives, handy for $EDITOR
```

The repository ships a starter set in
[../examples/boilerplates](../examples/boilerplates):

```bash
b --boilerplate-dir examples/boilerplates --template-dir examples/templates list
```

## A recipe

```yaml
description: Go CLI with a Makefile, a README and CI-ready layout
source:
  template: go-cli          # a folder template of the `t` store
vars:                       # the same shape as a template's .vars.yaml
  - name: module
    prompt: Go module path
    default: "example.com/{{ .Name }}"
    required: true
  - name: goVersion
    default: "1.24"
register:
  workspace: ~/work         # a plain name is created in here
  prefix: work              # so the project answers to `pj work-myapp`
  tags: [go, work]
  priority: 10
```

- **`source.template`** names a folder template of the `t` store, which is
  rendered with `.Name` set to the project name — so a template path segment
  like `cmd/{{ .Name }}` lands under it. See
  [templates.md](templates.md) for what a template can do.
- **`vars`** is exactly a template manifest's list: prompt, templated default,
  `type` (string, int, bool, choice, list) and `required`. Leave it out and `b`
  asks whatever the template itself asks for.
- **`register`** is where the project belongs. `skip: true` is for a recipe
  that adds files to something that already exists, and has no project of its
  own to register.

A source is one of those two. There is no third: delegating to `cargo new` and
friends would be a worse `after:` hook, which already runs any command you
like in the new project.

## Starting from a repository

A recipe can clone a starting point instead of rendering one:

```yaml
source:
  repo: github:dynamotn/go-starter   # or a URL, or a path on this machine
  ref: main
  render: false
```

`repo` takes a URL (`https://…`, `git@host:group/name`, `ssh://…`), a path on
this machine, or `server:group/name` resolved against your configured
`gitServers` — so a shared recipe works for people whose remotes differ in
scheme or host.

The clone is **shallow**, and its `.git` is left behind: the new project is not
a fork of the starting point, and its first commit is its own.

`render` is **off by default**. Someone else's repository is full of braces
that are its own, and copying it verbatim is what a starting point usually
means. A repository written to be a template sets `render: true`, and then
every file and every path segment goes through the engine as a folder template
would.

## Running commands afterwards

```yaml
after:
  - go mod tidy
  - git init -q -b main
  - git add -A && git commit -q -m "Start {{ .Name }}"
```

Each line runs in the new project, through your shell, so pipes and `&&` work.
Each one is **rendered first**, so it can use the values, and **printed before
it runs** — a recipe that runs commands should never do so out of sight.
`PROJEKT_NAME`, `PROJEKT_PATH` and `PROJEKT_BOILERPLATE` are in the
environment.

The first failure stops the rest, because the second command of a recipe
usually assumes the first one worked. The files stay: only the hook failed.

> A recipe runs commands. `b show <name>` prints them, `b new --dry-run` lists
> them without running any, and `--no-hooks` skips them. Read a recipe you did
> not write before you run it.

## Pointing at a remote

```yaml
register:
  remote:
    host: github      # one of your gitServers
    group: dynamotn
    name: myapp       # defaults to the project name
    branch: main      # the initial branch, when it is not a repository yet
    inRepos: true     # record it under the workspace's git section
```

This runs `git init` when the project is not a repository yet and points
`origin` at the URL built from your configured server.

**The repository is not created on the server.** That needs an API, a token and
a network, none of which this tool holds — `gh repo create` or `glab repo
create` in an `after` hook is the way, and it is the way on purpose.

`inRepos` records the new repository under the workspace's `git:` section, so
`projekt folder sync` reproduces it on the next machine and `folder check`
notices when it goes missing.

## Creating

```bash
b new go-cli myapp                  # ~/work/myapp, registered, pj work-myapp
b new go-cli myapp -i               # ask for what the recipe needs
b new go-cli ./scratch/myapp        # a path is created exactly there
b new go-cli myapp -w ~/oss         # a different workspace, just this once
b new go-cli myapp --set module=example.com/myapp
b new go-cli myapp --dry-run        # the whole plan, nothing written
```

**A name or a path.** A target with a separator in it is a path and is used as
it is. A plain name is created inside the workspace — `--workspace`, else the
recipe's, else the current folder — which is what makes the new project
reachable by name straight away.

**Values** come from `--set key=value` and `--values file.yaml` exactly as in
`t new`, and `--interactive` asks for the rest. Questions go to standard
error, so the plan printed on standard output stays readable by a script.

**Nothing is created on top of something else.** A destination that already
has files in it is refused unless `--force` is given.

## Registering

Once the files are there — and only then, so that a failed create never leaves
an entry pointing at nothing — the project is added to your configuration with
the recipe's prefix, tags and priority.

Except when it does not need to be:

| Situation | What happens |
| --------- | ------------ |
| The project lands inside a configured workspace | Nothing is added; the workspace already reaches it, and `b` says which name it answers to |
| The folder is already in the configuration | Nothing is added |
| The recipe sets `register.skip` | Nothing is added |
| `--no-register` | Nothing is added |
| The configuration cannot be read | Nothing is added, and the reason is printed |

That first row is the one worth knowing: a second entry for a folder a
workspace already covers would only give the same folder a second name.

```console
$ b new oss-tool mytool
Created /home/me/oss/mytool from template:go-cli
  /home/me/oss/mytool/.gitignore
  /home/me/oss/mytool/Makefile
  /home/me/oss/mytool/README.md
  /home/me/oss/mytool/cmd/mytool/main.go
  /home/me/oss/mytool/go.mod
Inside the workspace /home/me/oss already, reachable with `pj oss-mytool`
```

## Writing a recipe

Write the template first — `t add ./some-project --name go-cli` turns a project
you already have into one — then point a recipe at it:

```bash
$EDITOR (b path)/go-cli.yaml     # fish
$EDITOR "$(b path)"/go-cli.yaml  # bash
b new go-cli /tmp/try --dry-run  # check it without writing anything
```
