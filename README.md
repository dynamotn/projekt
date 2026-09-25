# projekt

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
[![CI](https://github.com/dynamotn/projekt/actions/workflows/ci.yml/badge.svg)](https://github.com/dynamotn/projekt/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/release/dynamotn/projekt.svg)](https://github.com/dynamotn/projekt/releases/latest)
[![License](https://img.shields.io/github/license/dynamotn/projekt.svg)](LICENSE)

> **projekt** – Stop `cd`-ing around your disk. One short name reaches any
> project you own — including the ones it clones for you, the ones it scaffolds
> from your templates, and the second branch it checks out beside your work.

```bash
pj backend-api                  # jump, from anywhere, in any shell
pj bak                          # half a name is enough
pj                              # or choose from a list
pj oss-some-library             # workspaces get a prefix, so names never collide
pj -                            # back where you came from, like `cd -`
projekt folder sync             # clone every repo your config declares
projekt folder status --dirty   # what did I leave half-done
projekt folder current          # and which project am I in right now
b new go-cli new-service        # scaffolded, registered, jumpable
t new adr doc/adr/0007-db.md    # a file from your own template store
projekt worktree add backend-api review/PROJ-123 && pj backend-api@PROJ-123
```

Three static Go binaries, no daemon, no index to rebuild — your config file *is*
the index. `t` and `b` are also `projekt template` and `projekt boilerplate`.

| | |
| --- | --- |
| `projekt` | folders, working trees, git, config — and the `pj` shell function |
| `t` | render a file, or a whole tree, from your own template store |
| `b` | create a project from a template or a repository, and put it in your config |

*(`projekt` is simply *project* in German; the shell function is `pj`, because
you'll type it a hundred times a day.)*

## ⚡️ Quick start

**1. Install** — from source, with Go ≥ 1.24. Binaries land in
`$HOME/.local/bin`; override with `INSTALL_PATH`. Release binaries exist for
Linux and macOS, amd64 and arm64.

```bash
make all
```

**2. Wire up your shell** — `pj` is a shell function, since a binary cannot
change its parent's directory:

```bash
eval "$(projekt init bash)"   # ~/.bashrc
eval "$(projekt init zsh)"    # ~/.zshrc — anywhere, before or after compinit
projekt init fish | source    # ~/.config/fish/config.fish
```

Completion reads your projects at tab time, so something you just created with
`b new` or `worktree add` is offered without reloading anything.

**3. Check it took** — `projekt doctor` verifies git, the binaries, the shell
integration, the config and its folders, and the template and boilerplate
stores. It changes nothing and exits non-zero when something won't work, so a
setup script can gate on it.

**4. Register your first folder**

```bash
projekt folder add ~/work/projekt        # one project
projekt folder add ~/oss -W -p oss -P 10 # a whole workspace
projekt folder list                      # see what resolves to what
pj projekt                               # and jump
```

That's it. Everything below is optional.

## ⚙️ Configuration

`$XDG_CONFIG_HOME/projekt/config.yaml` (`~/.config/projekt/config.yaml`),
overridable with `--config`, created empty on first run.

```yaml
folders:
  - path: /home/me/work/projekt   # reachable as `pj projekt`

  - path: /home/me/Dotfiles       # reachable as `pj dot`
    name: dot

  - path: /home/me/oss            # every child is a project: `pj oss-<child>`
    prefix: oss
    is_workspace: true
    regex: '^[^.].+'
    priority: 10
    tags: [oss, go]
```

| Key | What it does |
| --- | --- |
| `path` | The folder itself, or the parent folder when `is_workspace` is set |
| `name` | Short name. Defaults to the last element of `path`; ignored for a workspace, whose children are named after their own directory |
| `prefix` | Prepended to the short name, separated by a `-` |
| `is_workspace` | Treat every child folder as its own project |
| `regex` | Workspace only — which children count. Defaults to `^[^.].+` |
| `priority` | Tie-breaker when two folders resolve to the same short name |
| `tags` | Labels to filter on. A workspace passes its tags to its children |

Top-level `include:` merges other config files (read only), and a
`worktrees:` section holds what `projekt worktree` creates
([doc/worktrees.md](doc/worktrees.md)).

**Tags** decide which folders a command *acts on*, where `prefix` decides what
they're *called*. Every selecting command takes `--tags`/`-t`, and asking for
several narrows — `-t go,work` keeps the folders carrying **both**:

```bash
projekt folder add ~/oss -W -p oss -t oss,go
projekt folder list -t work
projekt folder sync -t work
```

**Checking and pruning.** `config check` reports every problem rather than
stopping at the first, separating **errors** (an invalid regex, two repos
checking out to the same path) from **warnings** (a folder that doesn't exist
yet). Only errors fail the command, which makes it a CI gate; `--strict`
tightens that. It is also the one command that still runs when the file cannot
be parsed at all. `config edit` re-reads the file when the editor exits.

```bash
projekt config check --strict
projekt config edit
projekt folder prune --dry-run  # drop entries whose folder is gone
```

Prune only drops a path that is definitely absent — a folder on an unmounted
drive reads as an error and is left alone.

> `config check` validates the configuration; `folder check` inspects the Git
> repositories on disk. Different jobs, similar names.

**One config, two machines.** `include` composes configuration instead of
forking it, and a `config.<hostname>.yaml` beside the main file is merged
without being listed. Includes are **read only** — everything that writes
writes the main file and nothing else.

```yaml
include:
  - ~/dotfiles/projekt/shared.yaml
  - team.yaml # relative to the file that names it
```

The file you're looking at comes first, then its includes in order — the order
`priority` already resolves collisions in. A file is read once even when two
others include it, a cycle is harmless, and a missing include is a warning.

## 🔗 Git integration

Declare your servers once, list what a workspace should contain, and let
projekt reconcile the difference. Extra `remotes` and per-repo `worktrees` are
declared the same way and reconciled on **every** sync, existing clones
included.

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
          remotes:
            upstream: git@github.com:upstream/backend.git
          worktrees:
            - path: api-next
              branch: next
        - name: frontend
```

```bash
projekt folder check          # [OK] / [MISSING] / [NOT GIT] / [REMOTE MISMATCH] / …
projekt folder sync --dry-run # what would be cloned and set up
projekt folder sync           # clone the missing ones, 4 at a time (--forks)
```

An existing worktree is left alone — it may have work in progress — and an
existing branch is checked out rather than recreated.

**Don't type that list.** If the repos are already on disk, `--discover` writes
the `git` section for you: each child holding a `.git` is read, its `origin`
matched against your configured servers, and the group, name and any differing
`path` recorded. A folder names one host and one group, so when a workspace
mixes several, the majority wins and the rest are reported and skipped.

```bash
projekt folder add ~/work/myorg -W -p myorg --discover
```

Full details in [doc/git-integration.md](doc/git-integration.md).

## 🌳 Working trees

Reviewing a pull request while your own branch is half-finished is the moment
`git stash` was invented for, and the moment it is worst at:

```bash
projekt worktree add myapp feature/PROJ-123
pj myapp@PROJ-123
projekt worktree list
projekt worktree remove myapp@PROJ-123
```

The name after the `@` is the last element of the branch. Trees live in
`<project>/.worktrees/` unless `--path` says otherwise, and the branch is
created from HEAD when it doesn't exist. Nothing in the shell integration knows
about them — they resolve through the same short names as everything else, so
`pj`, completion and `--tags` pick them up ([doc/worktrees.md](doc/worktrees.md)).

## 🧩 Templates and 🧱 boilerplates

`t` writes files into a project; `b` creates the project.

`t` renders [Go templates](https://pkg.go.dev/text/template) with
[sprig](https://masterminds.github.io/sprig/) from a store you own
(`$XDG_DATA_HOME/projekt/templates`, or `--template-dir` /
`PROJEKT_TEMPLATE_DIR`). A template is one file or a whole folder — and in a
folder template the **path segments are rendered too**, so the template names
the files it creates:

```
templates/
├── license.tmpl          # t new license LICENSE --set author='Jane Doe'
└── go-cli                # t new go-cli ./myapp --name myapp
    ├── go.mod.tmpl
    └── cmd/{{ .Name }}/main.go.tmpl  # becomes cmd/myapp/main.go
```

Values come from a `.data.yaml` of the store or the template (the things not
worth typing twice), repeatable `--set key=value` (dots nest, values keep their
YAML type), `--values file.yaml` (deep-merged, `--set` wins), or
`--interactive`, which asks for what's missing — from the template's
`.vars.yaml`, or else from the `.Values` keys in the template itself. Templates
also get `.Name`, `.Project`, `.Dir`, `.Path`, `.Template`, `.Source`, `.Store`,
`.User`, `.Home`, `.Hostname`, `.OS`, `.Arch`, `.Env`, `.Now`, `.Date` and
`.Year`. An unset value does not fail the render, so
`{{ .Values.license | default "MIT" }}` makes one optional. Nothing is
overwritten without `--force`, and a path segment rendering to `..`, to an
unset value or to anything containing a separator is refused.

Four names inside a folder template describe it rather than belong to it, and
between them one template covers what would otherwise be four:

| | |
| --- | --- |
| `.vars.yaml` | what to ask for |
| `.data.yaml` | what is already known — the lowest layer of `.Values` |
| `.ignore` | what *not* to write, rendered first: `{{ if not .Values.ci }}.github/{{ end }}` |
| `.templates/` | pieces several templates share: `{{ template "header" . }}` |

A prefix on a file name says what the file *is* rather than what is in it —
`executable_`, `private_`, `readonly_`, `symlink_`, `dot_`, `literal_` — and a
template can ask a question at the point it needs the answer with
`promptString`, `promptInt`, `promptBool` and `promptChoice`, alongside
`include`, `includeTemplate`, `output`, `lookPath`, `stat`, `toYaml` and
`fromYaml`.

A project keeps a record of what it was rendered from, in
`.projekt/template.yaml` — the template, the values and the hash of every file
written — so a template can be applied to it again when the template moves on:

```bash
t diff                    # what would change, as a unified diff; exits 1 if any
t apply                   # do it, replaying the recorded values
t apply go-cli -C ./myapp # one template, somewhere else

projekt folder exec -t work -- t diff    # which of my projects have drifted
```

Named no template, `apply` and `diff` cover every template the project records,
so nothing needs to be remembered about a project to bring it up to date.

A file that is byte for byte what was written is updated; one that was edited
by hand is a `conflict` and kept, unless `--force`; one the template no longer
writes is kept, unless `--prune`.

`b` renders a template, works out where the project belongs and registers it,
so starting something new ends with `pj`, not another `cd`:

```bash
b new go-cli myapp           # ~/work/myapp, registered, then pj work-myapp
b new go-cli ./scratch/myapp # a path is created exactly there
b new go-cli myapp -i        # ask for what the recipe needs
b new go-cli myapp --dry-run # the whole plan, nothing written
```

A recipe is one YAML file in `$XDG_DATA_HOME/projekt/boilerplates`:

```yaml
description: Go CLI with a Makefile, a README and CI-ready layout
source:
  template: go-cli
vars: # the same shape as a template's .vars.yaml
  - name: module
    default: "example.com/{{ .Name }}"
    required: true
register:
  workspace: ~/work # a plain name is created in here
  prefix: work      # so it answers to `pj work-myapp`
  tags: [go, work]
```

If the project lands inside a workspace that already reaches it, there is
nothing to add and `b` tells you the name it already answers to.

A starter set of both ships with the repository — Go CLI, ADR, pre-commit,
GitHub Actions, Compose, Terraform module, SECURITY.md, security pipeline,
threat model, incident report, daily note, zettel, budget, invoice
([examples/README.md](examples/README.md)). See [doc/templates.md](doc/templates.md)
and [doc/boilerplates.md](doc/boilerplates.md).

## 🎯 Half a name is enough

What you type is tried as the name, then as a prefix, then as anything inside
it, then letter by letter:

```bash
pj backend-api    # the name
pj backend        # a prefix
pj gateway        # somewhere in the middle
pj bak            # b, a, k — in order, in backend-api
pj                # no idea? choose from a list
```

**A whole name is never reinterpreted.** If what you typed is a project, that is
where you go, however many other names contain it — nothing typed in full can
take you somewhere unexpected.

When several match equally well, the project you have been in most recently and
most often wins; failing that the shorter name, which is the tighter match.
`--exact` turns it all off for a script that would rather be told it was wrong
than sent somewhere close:

```bash
projekt folder get backend --exact   # no, that is not a project
```

### Choosing from a list

`pj` with no argument, or `projekt folder select`, asks:

```bash
pj                          # all of them
projekt folder select api   # narrowed first
projekt folder select -t work
```

`fzf` or `sk` is used when one is on PATH; `$PROJEKT_PICKER` or `--with` names
another — anything that reads lines and writes one back. Otherwise the projects
are numbered and the answer is read from the terminal. One candidate needs no
question, and neither does a query that is already a whole name.

Only the path is printed and the list is drawn on standard error, so
`cd "$(projekt folder select)"` does what it looks like it does.

Want it on a key? Bind it yourself — nothing here touches your keymap:

```bash
# ~/.bashrc
bind -x '"\C-g": "cd \"$(projekt folder select)\""'
```

```fish
# ~/.config/fish/config.fish
bind \cg 'cd (projekt folder select); commandline -f repaint'
```

## 🩹 Across every project

`folder status` looks at every folder it can reach — workspace children and
working trees included, which have no config entry of their own:

```bash
projekt folder status --dirty
projekt folder status -o json | jq -r '.[] | select(.behind > 0) | .name'
```

Columns: `BRANCH` (or `detached`), `CHANGED` (untracked included), `AHEAD` /
`BEHIND` of upstream, `STASH`, `LAST COMMIT`. A missing folder says `missing`,
a non-repository says `not a repo`, and `--dirty` keeps both.

`folder exec` answers the question you cannot jump to: *which of them*.
Everything after `--` is the command. Folders are worked several at a time
(`--forks`, 4 by default) but **reported in configuration order**, so two runs
can be compared. A folder that isn't on disk is reported as missing rather than
as a failure, and the exit code is non-zero when the command failed anywhere:

```bash
projekt folder exec -- git status --short
projekt folder exec -t work --quiet -- git diff --quiet || echo "something is dirty"
```

**When a project is done**, `folder archive` **moves** it (never deletes) and
drops the entry afterwards, so a failed move leaves the project where it was.
It refuses while the repo has uncommitted changes, unpushed commits, no
upstream or a stash — the one mistake here that moving the folder back doesn't
undo; `--force` says you mean it. Destination: `$XDG_DATA_HOME/projekt/archive`,
or `--archive-dir` / `$PROJEKT_ARCHIVE_DIR` / `--to`.

**Where you've been**: every jump is remembered, one entry per project, so
`pj -` toggles between the two you're actually working on. `projekt folder
recent [--limit N|--clear]` lists them. The history lives in
`$XDG_STATE_HOME/projekt/history.tsv` — state, not configuration — capped at
200 projects. `folder get --no-record` looks a project up without counting it.

## ✏️ Keeping the config honest

A configuration is not written once. Projects move, tags change, and folders go
away — and until now each of those meant editing YAML or losing the entry.

```bash
projekt folder current              # which project am I in?
projekt folder current -q || echo "not in a project"

projekt folder move myapp ~/work/myapp   # moves the files, keeps the entry
projekt folder tag myapp go cli          # add tags
projekt folder tag myapp -r old          # take one away
projekt folder tag                       # what is in use, and how much

projekt worktree prune              # forget the working trees that are gone
```

`folder current` names the **deepest** match, so standing in a working tree
names the working tree rather than the project it hangs off, and it exits
non-zero when you are not in a project — which is what makes it usable in a
prompt or a script.

`folder move` does both halves of the job, because both happen: when the folder
is still at the old path it is moved, and when it has already been moved by
hand only the configuration catches up. The prefix, priority and tags survive,
which `remove` and `add` would have lost. Working trees follow — including
their name, since moving a project renames it.

`worktree prune` is the other half of `folder prune`: it also runs
`git worktree prune`, so git stops listing a working tree that was removed with
`rm -rf`.

A project found inside a workspace has no entry of its own, so it cannot be
moved or tagged on its own — projekt says so rather than doing nothing.

## 📚 Commands

### 📁 `projekt` — project folders

| Command | What it does |
| --- | --- |
| [`folder add`](doc/projekt_folder_add.md) | Register a folder or workspace; `--discover` reads its repos off disk |
| [`folder list`](doc/projekt_folder_list.md) | List every project folder, as a table, JSON or TSV |
| [`folder get`](doc/projekt_folder_get.md) | Resolve a name to a path, loosely — what `pj` calls |
| [`folder current`](doc/projekt_folder_current.md) | Which project is this folder in? Exits non-zero when none |
| [`folder move`](doc/projekt_folder_move.md) | Move a project and keep its prefix, tags and priority |
| [`folder tag`](doc/projekt_folder_tag.md) | Add or remove tags; list what is in use |
| [`folder select`](doc/projekt_folder_select.md) | Choose from a list — what `pj` with no argument calls |
| [`folder recent`](doc/projekt_folder_recent.md) | The projects you jumped to, most recent first |
| [`folder open`](doc/projekt_folder_open.md) | Open a project in `$VISUAL`, `$EDITOR` or vi |
| [`folder remove`](doc/projekt_folder_remove.md) | Drop a folder from the config |
| [`folder prune`](doc/projekt_folder_prune.md) | Drop every entry whose folder is gone, with `--dry-run` |
| [`folder archive`](doc/projekt_folder_archive.md) | Move a finished project away and drop it from the config |
| [`folder check`](doc/projekt_folder_check.md) | Verify configured Git repos, remotes and worktrees on disk |
| [`folder status`](doc/projekt_folder_status.md) | Branch, changes, ahead/behind, stashes, across every folder |
| [`folder sync`](doc/projekt_folder_sync.md) | Clone missing repositories in parallel, with `--dry-run` |
| [`folder exec`](doc/projekt_folder_exec.md) | Run one command in every project folder, several at a time |
| [`worktree add`](doc/projekt_worktree_add.md) | Check out a branch beside your work, reachable as `pj p@name` |
| [`worktree list`](doc/projekt_worktree_list.md) | List the working trees, and what git makes of them |
| [`worktree remove`](doc/projekt_worktree_remove.md) | Put one away; the branch is left alone |
| [`config check`](doc/projekt_config_check.md) | Validate the config file; exits non-zero, for CI |
| [`config edit`](doc/projekt_config_edit.md) | Open the config in `$EDITOR`, re-validate on exit |
| [`init`](doc/projekt_init.md) | Emit the shell integration for bash, zsh or fish |
| [`doctor`](doc/projekt_doctor.md) | Check this machine is set up; exits non-zero, for a script |
| [`version`](doc/projekt_version.md) | Version, commit, tree state and build time |

### 🧩 `t` — templates · 🧱 `b` — boilerplates

Each is also reachable as `projekt template <command>` / `projekt boilerplate <command>`.

| Command | What it does |
| --- | --- |
| [`t new`](doc/t_new.md) | Render a template, with `--set`, `--values`, `--dry-run` |
| [`t apply`](doc/t_apply.md) | Render a project's templates over it again, replaying its recorded values |
| [`t diff`](doc/t_diff.md) | What `apply` would change, as a unified diff; exits 1 when it found something |
| [`t list`](doc/t_list.md) | List the templates of the store, as a table, JSON or TSV |
| [`t add`](doc/t_add.md) | Save an existing file or folder as a template |
| [`t show`](doc/t_show.md) | Print the source of a template |
| [`t path`](doc/t_path.md) | Print the store path, or one template's — handy for `$EDITOR` |
| [`t check`](doc/t_check.md) | Validate a template, or the whole store; exits 1 on an error |
| [`t init`](doc/t_init.md) | Clone a repository of templates into the store |
| [`t sync`](doc/t_sync.md) | Bring the store up to date with its remote |
| [`b new`](doc/b_new.md) | Create a project from a recipe, and register it |
| [`b list`](doc/b_list.md) | List the recipes of the store, as a table, JSON or TSV |
| [`b show`](doc/b_show.md) | Print a recipe as it is written |
| [`b path`](doc/b_path.md) | Print the store path, or one recipe's — handy for `$EDITOR` |

### 📤 Output and logging

Listings render as a bordered table by default, plus JSON and TSV. JSON keeps
real types (`priority` a number, `isWorkspace` a boolean) and an empty listing
is `[]`, never `null`. TSV is what the shell integration uses for completion.

```bash
projekt folder list -o json | jq -r '.[].shortName'
projekt folder list -o tsv --short-only --no-headers
```

Every command takes `--verbose`/`-v` and honours `LOG_LEVEL` (or
`PROJEKT_LOG_LEVEL`): `trace`, `debug`, `info` (default), `warn`, `error`, `fatal`.

## 🛠 Development

```
cmd/          # entry points: projekt, t, b
pkg/cli/      # root command, logging, output, version
pkg/folderutil/  # folder parsing, discovery, Git helpers
pkg/lazypath/    # config loading and XDG paths
pkg/bplutil/     # boilerplate recipes
pkg/tplutil/     # template store and rendering
pkg/templates/   # shell integration templates
doc/          # generated command reference + guides
```

```bash
make lint  # gofmt + go vet
make test  # go test -race ./...
make build # all three binaries into bin/
make doc   # regenerate doc/ from the cobra commands
```

CI runs formatting, vet, build and the race-enabled test suite on every push
and pull request; tagged pushes are released with GoReleaser.

## 💬 Contributing

Issues and pull requests welcome — ideas, bug reports and new commands alike.
Found a folder layout `projekt` can't express? That's a bug report worth filing.

---

**Register your folders once, and never type a long `cd` again.**

<p align="center">
  <a href="https://github.com/dynamotn/projekt/stargazers">
    <img src="https://img.shields.io/github/stars/dynamotn/projekt?style=social" alt="Star projekt" />
  </a>
</p>
