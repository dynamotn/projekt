# Changelog

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

