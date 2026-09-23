# Projekt

A smart command to work with your project folder

## Features
Projekt has some complete solution for your terminal to
- Add/update/remove your Git project folders, and you can switch between project folder by `pj` (backed by `projekt folder get`) easily
- Create some template files in your project folder
- Create your project folder from various boilerplate sources

## Installing

### From source (Linux, macOS)

You must have a working Go environment and run this command
```bash
make all
```

Binaries are installed into `$HOME/.local/bin`. Use `INSTALL_PATH` to change it,
for example a system-wide install:
```bash
sudo make all INSTALL_PATH=/usr/local/bin
```

## Shell integration

`pj` jumps to a project folder. Add the matching line to your shell config:

```bash
# ~/.bashrc
eval "$(projekt init bash)"
```

```fish
# ~/.config/fish/config.fish
projekt init fish | source
```

## Configuration

The config file lives at `$XDG_CONFIG_HOME/projekt/config.yaml`
(`~/.config/projekt/config.yaml` by default) and can be overridden with
`--config`. It is created empty on the first run, and an unreadable or malformed
file is never overwritten.

```yaml
folders:
  # A single project folder, reachable as `pj projekt`
  - path: /home/me/work/projekt
    prefix: ""
    is_workspace: false
    priority: 0
  # A single project folder with an explicit short name, reachable as `pj dot`
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

- `name` is the short name of the folder itself, and defaults to the last
  element of its path. It does not apply to a workspace, whose child folders are
  named after their own directory.
- `prefix` is prepended to the short name, separated by a `-`.
- `regex` only applies to a workspace and defaults to `^[^.].+` (skip dotfiles).
- `priority` breaks ties: when two folders resolve to the same short name, the
  higher priority wins.

See [doc/git-integration.md](doc/git-integration.md) for cloning and checking
Git repositories, and the `doc` folder for the full command reference.

## Logging

The log level defaults to `info` and can be set with `--verbose`/`-v`, or with
the `LOG_LEVEL` environment variable (`PROJEKT_LOG_LEVEL` is also honoured).
Available levels: `trace`, `debug`, `info`, `warn`, `error`, `fatal`.

## Development

```bash
make lint   # gofmt + go vet
make test   # go test -race ./...
make doc    # regenerate the doc folder
```
