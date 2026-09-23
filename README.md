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
cd ~/work/clients/acme/services/backend-api
cd ../../../../oss/some-library
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
- **Git-aware** — declare your Git servers and repositories in config; `folder check` tells you what's missing or drifted, `folder sync` clones it — several repos at a time.
- **Predictable names** — prefixes keep names unique across workspaces, and `priority` decides the winner when two folders still collide.
- **Your config is safe** — an unreadable or malformed config file is never silently overwritten.
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
projekt folder add ~/work/projekt                     # one project
projekt folder add ~/oss -W -p oss -P 10              # a whole workspace
projekt folder list                                   # see what resolves to what
pj projekt                                            # and jump
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
```

| Key            | What it does                                                                        |
| -------------- | ----------------------------------------------------------------------------------- |
| `path`         | The folder itself, or the parent folder when `is_workspace` is set                    |
| `name`         | Short name for the folder itself. Defaults to the last element of `path`; ignored for a workspace, whose children are named after their own directory |
| `prefix`       | Prepended to the short name, separated by a `-`                                       |
| `is_workspace` | Treat every child folder as its own project                                           |
| `regex`        | Workspace only — which children count. Defaults to `^[^.].+`, so dotfiles are skipped |
| `priority`     | Tie-breaker: when two folders resolve to the same short name, the higher one wins     |

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

## 📚 Commands

### 📁 `projekt` — project folders

| Command                                              | What it does                                              |
| ---------------------------------------------------- | --------------------------------------------------------- |
| [`folder add`](doc/projekt_folder_add.md)            | Register a folder or workspace, with prefix and priority   |
| [`folder list`](doc/projekt_folder_list.md)          | List every project folder, as a table, JSON or TSV         |
| [`folder get`](doc/projekt_folder_get.md)            | Resolve a short name to a path — what `pj` calls           |
| [`folder remove`](doc/projekt_folder_remove.md)      | Drop a folder from the config                              |
| [`folder check`](doc/projekt_folder_check.md)        | Verify configured Git repos exist, are repos, match remote |
| [`folder sync`](doc/projekt_folder_sync.md)          | Clone missing repositories in parallel, with `--dry-run`   |
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
projekt folder list                                   # bordered table (default)
projekt folder list -o json | jq -r '.[].shortName'   # array of objects
projekt folder list -o tsv --short-only --no-headers  # one short name per line
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
make lint   # gofmt + go vet
make test   # go test -race ./...
make build  # all three binaries into bin/
make doc    # regenerate the doc folder from the cobra commands
make info   # tag, commit and tree state of this checkout
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
