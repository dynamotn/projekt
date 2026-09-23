# projekt

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
[![CI](https://github.com/dynamotn/projekt/actions/workflows/ci.yml/badge.svg)](https://github.com/dynamotn/projekt/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/release/dynamotn/projekt.svg)](https://github.com/dynamotn/projekt/releases/latest)
[![License](https://img.shields.io/github/license/dynamotn/projekt.svg)](LICENSE)

> **projekt** – Stop `cd`-ing around your disk.
> One short name jumps to any project you own, and the repos you don't have yet get cloned for you.

---

## ✨ From this

```bash
cd ~/work/clients/acme/services/backend-api || exit
cd ../../../../oss/some-library || exit
git clone git@github.com:myorg/myteam/frontend.git ~/work/myorg/frontend
```

## To this

```bash
pj backend-api      # anywhere on your machine, from any shell
pj oss-some-library # workspaces get a prefix, so names never collide
projekt folder sync # clone every repo your config knows about, you don't
```

A single static Go binary, no daemon, no index to rebuild — your config file *is* the index.

## 🚀 Why projekt?

- **Jump, don't navigate** — `pj <short-name>` resolves a project folder and takes you there. Deep trees stop mattering.
- **Workspaces, not just folders** — point at a parent directory once and every child inside it becomes a jumpable project, filtered by your own regex.
- **Git-aware** — declare your Git servers and repositories in config; `folder check` tells you what's missing or drifted, `folder sync` clones it — several repos at a time. Already have them cloned? `--discover` writes that config for you.
- **Predictable names** — prefixes keep names unique across workspaces, and `priority` decides the winner when two folders still collide.
- **Tagged, not just named** — label folders `work`, `oss`, `go`, then point any command at a subset with `--tags`.
- **Two branches at once** — `worktree add` checks out a branch beside your work and gives it a name, so `pj myapp@PROJ-123` is the whole context switch.
- **Your config is safe** — an unreadable or malformed config file is never silently overwritten, and `config check` tells you what is wrong with it.
- **Templates that name their own files** — `t new` renders a Go template, or a whole folder of them, from a store you own; the path segments are templates too.
- **New projects that are already on the map** — `b new` renders a boilerplate into the right workspace and registers it, so the next thing you type is `pj`.
- **Shell-native** — a one-line `eval` for bash, zsh or fish, with completion. No plugin manager required.
- **Boring to install** — `make all`, or grab a release binary. Linux and macOS, amd64 and arm64.

## 📖 What does the name mean?

`projekt` is simply *project* in German — the tool does one thing, and the name says it.
The shell function it installs is `pj`, because you'll type it a hundred times a day.

## ⚡️ Quick Start

**1. Install it**

From source — you need a working Go ≥ 1.24 environment:

```bash
make all
```

Binaries land in `$HOME/.local/bin`. Override with `INSTALL_PATH` for a system-wide install:

```bash
sudo make all INSTALL_PATH=/usr/local/bin
```

**2. Wire up your shell**

`pj` is a shell function, so it has to be sourced — a binary cannot change its parent's directory:

```bash
# ~/.bashrc
eval "$(projekt init bash)"
```

```zsh
# ~/.zshrc
eval "$(projekt init zsh)"
```

```fish
# ~/.config/fish/config.fish
projekt init fish | source
```

The zsh line can go anywhere in your `.zshrc`: when it runs before `compinit`,
the completion registers itself at the first prompt instead of being lost.

**3. Register your first folder**

```bash
projekt folder add ~/work/projekt        # one project
projekt folder add ~/oss -W -p oss -P 10 # a whole workspace
projekt folder list                      # see what resolves to what
pj projekt                               # and jump
```

That's it. Everything below is optional.

## ⚙️ Configuration

The config file lives at `$XDG_CONFIG_HOME/projekt/config.yaml`
(`~/.config/projekt/config.yaml` by default) and can be overridden with `--config`.
It is created empty on the first run.

```yaml
folders:
  # A single project folder, reachable as `pj projekt`
  - path: /home/me/work/projekt
    prefix: ""
    is_workspace: false
    priority: 0

  # A folder with an explicit short name, reachable as `pj dot`
  - path: /home/me/Dotfiles
    name: dot
    is_workspace: false

  # A workspace: every matching child folder becomes a project,
  # reachable as `pj oss-<child>`
  - path: /home/me/oss
    prefix: oss
    is_workspace: true
    regex: '^[^.].+'
    priority: 10
    tags: [oss, go]
```

| Key            | What it does                                                                        |
| -------------- | ----------------------------------------------------------------------------------- |
| `path`         | The folder itself, or the parent folder when `is_workspace` is set                    |
| `name`         | Short name for the folder itself. Defaults to the last element of `path`; ignored for a workspace, whose children are named after their own directory |
| `prefix`       | Prepended to the short name, separated by a `-`                                       |
| `is_workspace` | Treat every child folder as its own project                                           |
| `regex`        | Workspace only — which children count. Defaults to `^[^.].+`, so dotfiles are skipped |
| `priority`     | Tie-breaker: when two folders resolve to the same short name, the higher one wins     |
| `tags`         | Labels to filter on later. A workspace passes its tags to every folder inside it      |

A `worktrees:` section holds the working trees `projekt worktree` creates; see
[doc/worktrees.md](doc/worktrees.md).

## 🏷 Tags

`prefix` decides what a folder is *called*; `tags` decide which folders a command
*acts on*. Every command that selects folders takes the same `--tags`/`-t` flag:

```bash
projekt folder add ~/oss -W -p oss -t oss,go # tag on the way in
projekt folder list -t work                  # only work folders
projekt folder sync -t work                  # only clone work repos
projekt folder check -t work
```

Asking for several tags narrows the selection — `-t go,work` keeps the folders
carrying **both**, not either. Tags are matched exactly and are case sensitive;
surrounding whitespace is forgiven, and blank tags are ignored rather than
treated as a filter that matches nothing. `--tags` completes from the tags
already in your config, so a typo shows up as a missing suggestion.

## 🩺 Checking and editing the config

The config file is hand-written often enough to be worth checking on purpose,
rather than finding out from the next command that behaves oddly:

```bash
projekt config check          # every problem, exits non-zero on an error
projekt config check --strict # warnings count as errors too
projekt config edit           # open it in $VISUAL / $EDITOR / vi
```

`config check` reports the whole list rather than stopping at the first
problem, and separates the two kinds: an **error** makes an entry unusable (an
invalid regex, two repos checking out to the same path, a worktree with no
branch), a **warning** is worth knowing but harmless (a folder that does not
exist yet, an unknown git server). Only errors fail the command, which makes it
a usable CI gate; `--strict` tightens that.

It is also the one command that still runs when the file cannot be parsed at
all — every other command refuses, and this one tells you why:

```
$ projekt config check
Checking /home/me/.config/projekt/config.yaml
[ERROR] failed to read config file ...: yaml: line 1: did not find expected ',' or ']'
```

`config edit` re-reads the file once the editor exits, so a mistake is reported
straight away instead of on your next `pj`.

> Note: `projekt config check` validates the configuration. `projekt folder
> check` inspects the Git repositories on disk. Different jobs, similar names.

## 🔗 Git integration

Declare your Git servers once, list the repositories a workspace should contain, and
let projekt reconcile the difference:

```yaml
gitServers:
  - name: github
    type: github
    https: https://github.com
    ssh: git@github.com
    preferGitSSH: true

folders:
  - path: /home/me/work/myorg
    prefix: myorg
    is_workspace: true
    git:
      host: github # references gitServers[].name
      group: myorg/myteam
      repos:
        - name: backend
          path: api # local folder name, defaults to the repository name
        - name: frontend
```

```bash
projekt folder check          # [OK] / [MISSING] / [NOT GIT] / [WARNING] per repo
projekt folder sync --dry-run # what would be cloned
projekt folder sync           # clone the missing ones, 4 at a time
projekt folder sync --forks 1 # one after another instead
```

SSH and HTTPS are both supported, including `ssh://host:port` URLs, and
`preferGitSSH` decides which one is tried first — the other stays as a fallback.
Full details in [doc/git-integration.md](doc/git-integration.md).

### 🔍 Don't type that list — discover it

If the repositories are already on disk, `--discover` writes the `git` section
for you instead of making you transcribe it:

```bash
projekt folder add ~/work/myorg -W -p myorg --discover
```

Every child folder holding a `.git` is read, its `origin` remote decides which
configured server it belongs to, and the group and repository name are peeled
off the URL. A checkout whose directory name differs from the repository gets
its `path` recorded. A folder names one host and one group, so when the
workspace mixes several, the one most repositories share wins and the rest are
reported and skipped rather than quietly misfiled.

Configure your `gitServers` first — that is what a remote URL is matched
against. `--discover` needs `--as-workspace`, since it scans a workspace's
children.
### 🔀 Extra remotes and worktrees

A clone gives you `origin`. A fork-based workflow needs more than that, and a
long-running branch is easier to keep in its own working tree than to stash
around. Both are declared per repository and reconciled on every sync:

```yaml
      repos:
        - name: backend
          path: api
          remotes:
            upstream: git@github.com:upstream/backend.git
          worktrees:
            - path: api-next # beside the repos, like `path` above
              branch: next
```

| Key         | What it does                                                            |
| ----------- | ------------------------------------------------------------------------ |
| `remotes`   | Extra remotes by name. Added when missing, repointed when the URL changed |
| `worktrees` | Extra working trees. `path` is relative to the folder, `branch` required  |

```bash
projekt folder sync --dry-run # also reports the remotes and worktrees it would set up
projekt folder sync           # add them, on existing clones too
projekt folder check          # [REMOTE MISSING] / [REMOTE MISMATCH] / [WORKTREE MISSING]
```

Adding a remote to a repository you cloned last year is the ordinary case, so
sync reconciles every repository it knows about, not only the ones it just
cloned. An existing worktree is left alone — it may well have work in progress
in it — and a branch that already exists is checked out rather than recreated.
Listing `origin` under `remotes` overrides what the clone set up.

## 🌳 Working trees

Reviewing a pull request while your own branch is half-finished is the moment
`git stash` was invented for, and the moment it is worst at. A working tree is
another checkout of the same repository, on another branch — nothing to stash,
nothing to rebuild:

```bash
projekt worktree add myapp feature/PROJ-123
pj myapp@PROJ-123
```

The name after the `@` comes from the last element of the branch, because that
is the part that tells two branches apart. They live in
`<project>/.worktrees/` unless `--path` says otherwise, and the branch is
created from HEAD when it does not exist yet.

```bash
projekt worktree list            # with what git makes of each one
projekt worktree remove myapp@PROJ-123
```

Nothing in the shell integration knows about them: a working tree resolves
through the same short names as everything else, so `pj`, its completion and
`--tags` pick it up on their own. See
[doc/worktrees.md](doc/worktrees.md).

## 🧩 Templates

`t` (also `projekt template`) renders [Go templates](https://pkg.go.dev/text/template),
with the [sprig](https://masterminds.github.io/sprig/) functions on top, from a
folder of templates you own — `$XDG_DATA_HOME/projekt/templates` by default,
or wherever `--template-dir` / `PROJEKT_TEMPLATE_DIR` points.

```bash
t add ./LICENSE                  # turn a file you already have into a template
t list                           # what the store holds
t new license LICENSE --set author='Jane Doe'
t new go-cli ./myapp --name myapp --set module=example.com/myapp
t new invoice ./INV-001.md -i    # or let it ask you
t new dockerfile --dry-run       # render to stdout, write nothing
```

A template is either one file or a whole folder. In a folder template the
**path segments are rendered too**, so the template names the files it creates:

```
~/.local/share/projekt/templates
├── license.tmpl                 # t new license
└── go-cli                       # t new go-cli ./myapp --name myapp
    ├── go.mod.tmpl
    └── cmd
        └── {{ .Name }}          # becomes cmd/myapp/
            └── main.go.tmpl     # becomes main.go
```

Values come from repeatable `--set key=value` (dots nest, and the value keeps
its YAML type, so `port=8080` is a number), `--values file.yaml`, merged deeply
with `--set` winning, or `--interactive`, which asks for whatever is still
missing — from the template's `.vars.yaml` when it has one, and otherwise from
the `.Values` keys read out of the template itself. A template reaches them through `.Values`, and is
handed `.Name`, `.Project`, `.Dir`, `.Path`, `.Template`, `.User`, `.Now`,
`.Date` and `.Year` besides. A value nobody set renders empty rather than
failing, so `{{ .Values.license | default "MIT" }}` makes one optional.

Nothing is overwritten without `--force`, and a path segment that renders to a
`..` or to anything containing a separator is refused — a value can never write
outside the destination.

A starter set ships with the repository — Go CLI, ADR, pre-commit, GitHub
Actions, Compose, Terraform module, SECURITY.md, security pipeline, threat
model, incident report, daily note, zettel, budget, invoice:

```bash
t --template-dir examples/templates list
```

See [examples/README.md](examples/README.md) for what each one does, and
[doc/templates.md](doc/templates.md) for the rest.

## 🧱 Boilerplates

`t` writes files into a project; `b` (also `projekt boilerplate`) creates the
project. It renders a template, works out where the project belongs, and
registers it — so starting something new ends with `pj`, not another `cd`.

```bash
b list                          # the recipes you have
b new go-cli myapp              # ~/work/myapp, registered, then pj work-myapp
b new go-cli myapp -i           # ask for what the recipe needs
b new go-cli ./scratch/myapp    # a path is created exactly there
b new go-cli myapp --dry-run    # the whole plan, nothing written
```

A recipe is one YAML file in `$XDG_DATA_HOME/projekt/boilerplates`:

```yaml
description: Go CLI with a Makefile, a README and CI-ready layout
source:
  template: go-cli        # a folder template of the `t` store
vars:                     # the same shape as a template's .vars.yaml
  - name: module
    default: "example.com/{{ .Name }}"
    required: true
register:
  workspace: ~/work       # a plain name is created in here
  prefix: work            # so it answers to `pj work-myapp`
  tags: [go, work]
```

The project is added to your config once the files are there — unless it lands
inside a workspace that already reaches it, in which case there is nothing to
add and `b` tells you the name it already answers to. A starter set of recipes
ships in [examples/boilerplates](examples/boilerplates); see
[doc/boilerplates.md](doc/boilerplates.md) for the rest.

## 📚 Commands

### 📁 `projekt` — project folders

| Command                                              | What it does                                              |
| ---------------------------------------------------- | --------------------------------------------------------- |
| [`folder add`](doc/projekt_folder_add.md)            | Register a folder or workspace; `--discover` reads its repos off disk |
| [`folder list`](doc/projekt_folder_list.md)          | List every project folder, as a table, JSON or TSV         |
| [`folder get`](doc/projekt_folder_get.md)            | Resolve a short name to a path — what `pj` calls           |
| [`folder remove`](doc/projekt_folder_remove.md)      | Drop a folder from the config                              |
| [`folder check`](doc/projekt_folder_check.md)        | Verify configured Git repos, remotes and worktrees on disk |
| [`folder sync`](doc/projekt_folder_sync.md)          | Clone missing repositories in parallel, with `--dry-run`   |
| [`worktree add`](doc/projekt_worktree_add.md)        | Check out a branch beside your work, reachable as `pj p@name` |
| [`worktree list`](doc/projekt_worktree_list.md)      | List the working trees, and what git makes of them          |
| [`worktree remove`](doc/projekt_worktree_remove.md)  | Put one away; the branch is left alone                      |
| [`config check`](doc/projekt_config_check.md)        | Validate the config file; exits non-zero, for CI           |
| [`config edit`](doc/projekt_config_edit.md)          | Open the config in `$EDITOR`, re-validate on exit          |
| [`init`](doc/projekt_init.md)                        | Emit the shell integration for bash, zsh or fish      |
| [`version`](doc/projekt_version.md)                  | Version, commit, tree state and build time                 |

### 🧩 `t` — templates

Every one of these is also reachable as `projekt template <command>`.

| Command                          | What it does                                                |
| -------------------------------- | ------------------------------------------------------------ |
| [`t new`](doc/t_new.md)          | Render a template, with `--set`, `--values`, `--dry-run`     |
| [`t list`](doc/t_list.md)        | List the templates of the store, as a table, JSON or TSV     |
| [`t add`](doc/t_add.md)          | Save an existing file or folder as a template                |
| [`t show`](doc/t_show.md)        | Print the source of a template                               |
| [`t path`](doc/t_path.md)        | Print the store path, or one template's — handy for `$EDITOR` |

### 🧱 `b` — boilerplates

Every one of these is also reachable as `projekt boilerplate <command>`.

| Command                          | What it does                                                 |
| -------------------------------- | ------------------------------------------------------------- |
| [`b new`](doc/b_new.md)          | Create a project from a recipe, and register it               |
| [`b list`](doc/b_list.md)        | List the recipes of the store, as a table, JSON or TSV        |
| [`b show`](doc/b_show.md)        | Print a recipe as it is written                               |
| [`b path`](doc/b_path.md)        | Print the store path, or one recipe's — handy for `$EDITOR`   |

### 📤 Output formats

`folder list` renders as a table for reading, and in two machine-readable
formats for everything else:

```bash
projekt folder list                                  # bordered table (default)
projekt folder list -o json | jq -r '.[].shortName'  # array of objects
projekt folder list -o tsv --short-only --no-headers # one short name per line
```

JSON keeps real types — `priority` is a number, `isWorkspace` a boolean — and
an empty listing is `[]`, never `null`. TSV is what the shell integration uses
for `pj` completion: a folder whose name contains a `|` would be mangled by
anything that tried to read the table instead.

### 🔊 Logging

Every command takes `--verbose`/`-v`, and honours `LOG_LEVEL` (or `PROJEKT_LOG_LEVEL`)
from the environment. Levels: `trace`, `debug`, `info`, `warn`, `error`, `fatal` —
`info` by default.

## 🗂 Directory Structure

```
.
├── bin/           # Built binaries (make build)
├── cmd/           # Entry points: projekt, t, b
├── doc/           # Generated command reference + guides
├── internal/      # Version stamping
├── pkg/
│   ├── cli/         # Root command, logging, output, version
│   ├── folderutil/  # Folder parsing, discovery, Git helpers
│   ├── lazypath/    # Config loading and XDG paths
│   ├── bplutil/     # Boilerplate recipes: create a project and register it
│   ├── templates/   # Shell integration templates
│   └── tplutil/     # Template store and Go template rendering
└── Makefile       # build, install, lint, test, doc
```

## 🛠 Development

```bash
make lint  # gofmt + go vet
make test  # go test -race ./...
make build # all three binaries into bin/
make doc   # regenerate the doc folder from the cobra commands
make info  # tag, commit and tree state of this checkout
```

CI runs formatting, vet, build and the race-enabled test suite on every push and
pull request; tagged pushes are released with GoReleaser.

## 💬 Contribution & Support

- Open an Issue or Pull Request — ideas, bug reports and new commands are all welcome.
- Found a folder layout `projekt` can't express? That's a bug report worth filing.

---

**Register your folders once, and never type a long `cd` again.**

<p align="center">
  <a href="https://github.com/dynamotn/projekt/stargazers">
    <img src="https://img.shields.io/github/stars/dynamotn/projekt?style=social" alt="Star projekt" />
  </a>
</p>
