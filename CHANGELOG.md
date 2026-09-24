# Changelog

## [3.0.0]

### Added

- **folder**: reach a project by half its name — `pj bak` finds `backend-api`,
  ranked by what you have been working in
- **folder**: choose a project from a list with `pj` alone or `folder select`,
  through fzf when you have it
- **folder**: ask which project a folder is in with `folder current`
- **folder**: move a project with `folder move`, keeping its prefix, tags and
  priority, and taking its working trees with it
- **folder**: add and remove tags with `folder tag`, and list what is in use
- **worktree**: forget the working trees whose folder is gone with `worktree
  prune`, on both sides — the config entry and git's own record
- **doctor**: read the report as JSON, TSV or a table with `--output`
- **b**: start a project from a repository with `source.repo`, cloned shallowly
  and with its history left behind
- **b**: run commands in the new project with `after:`, each one printed before
  it runs and skippable with `--no-hooks`
- **b**: point a new project at a remote with `register.remote`, and record it
  under the workspace with `inRepos`
- **t**: keep the values you never want to type again in a `.data.yaml`, at the
  root of the store for every template or inside one for itself; they sit under
  `--values`, `--set` and `--interactive`, so a value they hold is never asked
  for again
- **t**: share pieces between templates in a `.templates` folder, called with
  `{{ template "header" . }}` or `{{ includeTemplate "header" . | indent 2 }}`;
  a folder template may bring its own, which wins
- **t**: leave files out with a `.ignore`, rendered before it is read, so
  `{{ if not .Values.ci }}.github/{{ end }}` makes one template cover the
  variants of a project instead of four that drift apart
- **t**: say what a rendered file *is* with a prefix on its name —
  `executable_`, `private_`, `readonly_`, `symlink_`, `dot_` and `literal_`;
  read off the template's own name, so no value can add one
- **t**: ask a question at the point the answer is needed with `promptString`,
  `promptInt`, `promptBool` and `promptChoice`, asked once however many files
  ask it, and answered by the default when there is no `--interactive`
- **t**: new functions `include`, `includeTemplate`, `output`, `lookPath`,
  `stat`, `joinPath`, `toYaml` and `fromYaml`
- **t**: new variables `.Source`, `.Store`, `.Home`, `.Hostname`, `.OS`,
  `.Arch` and `.Env`

### Removed

- **b**: `source.command`, which was never implemented; an `after:` hook
  already runs any command you like in the new project


## [2.1.0]

### Added

- **shell**: add the zsh integration
- **worktree**: work on two branches of a project at once
- **folder**: drop the entries whose folder is gone
- **config**: compose the configuration from several files
- **doctor**: say whether this machine is set up
- **folder**: go back where you were with `pj -`
- **folder**: show the git state of every project
- **folder**: archive a project that is done
- **folder**: open a project in your editor
- **folder**: run one command in every project

### Fixed

- **shell**: offer a project fish has not seen created
- remove duplicate function outputLines
- lack of refresh after pruning stale entries


## [2.0.0]

### Added

- **config**: actually validate the configuration
- **folder**: honour the name configured for a folder
- **folder**: clone repositories concurrently on sync
- **folder**: add --output json and tsv to list
- **folder**: tag folders and filter commands by tag
- **folder**: discover a workspace's repositories from disk
- **config**: add config check and edit, plus per-repo remotes and worktrees
- **t**: render Go templates from a template store
- **t**: ask for the values a template needs
- **b**: create a project from a boilerplate and register it
- **release**: cut releases with a dybatpho-backed script

### Fixed

- **config**: never overwrite an unreadable config file
- **folder**: report an unknown short name instead of printing nothing
- **cli**: honour the log level set by LOG_LEVEL and --verbose
- **cli**: stop printing a Go stack trace on every warning
- **git**: keep the ssh:// scheme when building a clone URL
- **git**: fall back to SSH when preferGitSSH is false
- **git**: default a repo path to the repository name
- **git**: keep the cause when reading a remote URL fails
- **git**: do not stop sync at the first failing folder
- **git**: remove the empty folder left by a failed clone
- **folder**: let priority decide which folder keeps a short name
- **folder**: only follow a symlink that points at a directory
- **folder**: pass the completion error through a format verb
- **init**: reject a shell that has no template, and use the writer
- **folder**: store an absolute path when adding a folder
- **cli**: report a failure to write the table
- **shell**: use the exit code of `folder get`, add bash completion

