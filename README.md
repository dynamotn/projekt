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
- **Your config is safe** — an unreadable or malformed config file is never silently overwritten, and `config check` tells you what is wrong with it.
- **Shell-native** — a one-line `eval` for bash or fish. No plugin manager required.
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

```fish
# ~/.config/fish/config.fish
projekt init fish | source
```

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
| [`config check`](doc/projekt_config_check.md)        | Validate the config file; exits non-zero, for CI           |
| [`config edit`](doc/projekt_config_edit.md)          | Open the config in `$EDITOR`, re-validate on exit          |
| [`init`](doc/projekt_init.md)                        | Emit the shell integration for bash or fish           |
| [`version`](doc/projekt_version.md)                  | Version, commit, tree state and build time                 |

### 🧰 Companion binaries

`make all` also installs two smaller commands. Both are scaffolded today — the
folder management above is the part that's ready for daily use.

| Binary              | Intent                                                   |
| ------------------- | --------------------------------------------------------- |
| [`t`](doc/t.md)     | Create a template file from various sources               |
| [`b`](doc/b.md)     | Create a boilerplate project folder for a language/framework |

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
│   └── templates/   # Shell integration templates
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
