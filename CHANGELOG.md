# Changelog

## [Unreleased]

### Added

- **folder describe**: `--generate` drafts the description with the AI
  command you name in `ai.command` (or `PROJEKT_AI`), shows it, and keeps it
  on a yes or with `--yes`; `--show-prompt` prints what would be sent.
  projekt sends no credential file and runs nothing the AI answers
- **docs**: `doc/agent-materials.md` — which of `AGENTS.md`, a skill and a
  project note is for what, how to write a skill an agent uses, and how to
  keep one source for every agent with `t new agent-docs` and `t apply`
- **examples**: `agent-docs` writes one `AGENTS.md` for a project, with
  `CLAUDE.md` and `GEMINI.md` as links to it, and `agent-skill` scaffolds a
  skill; both read `.Folder`.
- **t**, **b**: templates get `.Folder` — the short name, path, tags,
  description and workspace projekt has for the project they render into,
  also for one being created under a workspace — so a template can say what
  the project is. Its fields are empty outside a configured project
- **mcp**: `projekt mcp --write` also offers `note_add`, `describe_set` and
  `worktree_add`; without the flag they are neither listed nor callable
- **mcp**: `projekt mcp` serves what projekt knows to AI agents over MCP on
  stdio: `project_list`, `project_show`, `project_status`, `project_recent`
  and `project_current`, all read-only. It never offers a tool that runs a
  command, moves or removes a project, or changes trust
- **folder**: `folder show [name]` prints everything projekt knows about one
  project — path, tags, description, the whole note, working trees, git
  state, hooks and the templates it was rendered from — as a listing or with
  `-o json` as one object
- **t**, **b**: every value reaches an `after:` line as
  `PROJEKT_VALUE_<NAME>`, and the template function `shquote` makes a value
  one shell word, so a line can use a value without the value becoming code
- **folder**: `folder sync --pull` fetches the repositories that are already
  on disk, configured working trees included, and fast-forwards each clean
  branch onto its upstream. A tree with uncommitted changes, a detached HEAD,
  no upstream or commits of its own is fetched, reported as skipped with the
  reason, and left alone; nothing is merged or rebased. `--dry-run`, `--forks`
  and `--tags` apply as they do to cloning
- **folder**: say what a project is with `description:` in the configuration,
  or `folder describe myapp "The billing API"`. It is a column of `folder
  list`, is offered beside the name when you press tab after `pj` in zsh and
  fish, and is drawn next to the candidates `pj` asks you to choose from. A
  workspace passes its description to its children, like its tags
- **folder**: keep a note with a project — `folder note -a "migration half
  done"` appends a dated line to `.projekt/notes.md` in the current project,
  `folder note [name]` reads it back, `--edit` opens it, and `folder status
  --notes` shows the first line of every note
- **pj**: a project can declare `hooks:` — `enter:` and `leave:` lines that
  `pj` evaluates in your shell on the way in and out, so `nvm use`, a
  `KUBECONFIG` or a virtualenv follow the jump. A top-level `hooks:` list
  attaches them by tag. `folder hooks <name>` shows what would run and why,
  `PROJEKT_NO_HOOKS=1` turns them off. Re-run `projekt init` for your shell
  to pick this up
- **folder**: `folder status --since 7d` keeps the folders with a commit or a
  jump that recent; `--branches` adds the default branch, how many of its
  commits the checked-out branch has not got and how long ago the two parted;
  `--requests` adds the open merge or pull requests of the branch, counted
  through `glab` or `gh` — the one column that goes to the network, so it is
  off unless asked for and reads `n/a` without the tool
- **folder**: `folder add --from-remote --host <server> --group <group>`
  asks a GitLab group or a GitHub organisation which repositories it has and
  writes them into the workspace's `git` section, so a new machine is
  `folder add --from-remote` and `folder sync` away from a full checkout.
  `--recursive` takes GitLab subgroups, `--archived` keeps archived
  repositories, `--include`/`--exclude` filter by name. A token comes from
  `GITLAB_TOKEN`/`GITHUB_TOKEN` or a logged-in `glab`/`gh`, and is kept nowhere
- **t**: `t apply` merges a template's change into a file that was edited by
  hand, when the two do not touch the same lines, and reports it as `merged`;
  only an overlap is still a `conflict`, and nothing is written with conflict
  markers in it. The merge is a three-way one against the copy of the last
  render a project now keeps in `.projekt/base/`, written by `t new` and
  `t apply` and worth committing with the record. `t diff` previews the merge

### Changed

- **t**: `t` reads the projekt configuration, with `--config` like `projekt`
  and `b`, so `t init gl:group/store` resolves a configured git server. It
  only reads: a missing or broken configuration never stops `t` or makes it
  warn, and `t` never creates the file
- **t**, **b**: `after:` lines run with `sh -c`, `sh` found on `PATH`, instead
  of your login shell, so a POSIX line means the same under fish, nu or none;
  this works on Termux too, where `/usr/bin/env` does not exist
- **t trust**, **b trust**, **config trust**: with no names they stop and ask
  for the names or `--all`; `--all` trusts every template, recipe or included
  file, and names together with `--all` is an error
- **t**: `t update` runs `t apply`, as its help says, instead of pulling the
  template store; `t sync` and `t pull` still pull it
- **t**: a dry run, `t check` and `b check` no longer run anything: `output`
  and `env` in a template or an `after:` line render empty while only being
  looked at, instead of executing a command or asking for trust
- **b**: `b new` says where the project is going before it writes anything,
  and `--dry-run` refuses the same non-empty folder a real run would
- **version**: a binary built with `go install module@version` reports that
  version, and one built from a checkout reports a development version
  instead of a constant nobody updated. `make build` off a tag stamps
  `git describe` instead of an empty string
- **config**: only a `PROJEKT_`-prefixed environment variable can override a
  configuration key; a bare `INCLUDE` or `FOLDERS` in the shell is left alone
- **go**: built with Go 1.26.9 and sprig v3; `getHostByName` is no longer
  available to templates

### Fixed

- **folder describe**: a description holding a control character is refused,
  like one holding a line break
- **config**: two commands that write the configuration at the same time, or
  a long `folder sync` or `b new` while you edit the file, no longer drop each
  other's changes — a write locks the file, re-reads it and applies only its
  own change — and a write that fails leaves the configuration as it was. A
  `PROJEKT_` override is never written into the file
- **folder add --from-remote**: on GitHub, your own user lists your private
  repositories too when the token is yours; the public listing GitHub gives
  for a user name left them out
- **folder list**, **folder status**, **worktree list**, **t list**,
  **b list**, **doctor**: a table piped into another command or a file, or
  printed with `NO_COLOR` set, has no color escapes; a terminal still gets
  color unless `--no-color` says otherwise
- **t add**: `t add <store>/x.tmpl --force` refuses instead of deleting the
  template, and a folder that holds the template store, such as `t add ~`, is
  refused instead of copied into itself
- **t add**, **t new**: the `.git` file of a worktree or a submodule is left
  out, as a `.git` folder already was, so a new project no longer points at
  the original repository
- **t**: files named like `.vars.yaml`, `.ignore` or `*.data.json` in a
  subfolder of a folder template are rendered like any other file; only the
  ones at the template's root describe the template
- **t**: a template file that is executable, such as `gradlew` or `run.sh`,
  renders executable
- **t add --replace**: a replacement no longer rewrites inside an action an
  earlier one inserted, so `my-service=Values.service` with
  `service=Values.kind` gives a working template
- **t**: a `symlink_` file with no final newline no longer shows as a
  conflict in `t diff` and `t apply` forever
- **t**: a link in the store to a folder inside the store is listed and
  rendered as a folder template
- **t apply --prune**: says when it cannot remove a folder it emptied,
  instead of nothing
- **config**: writing the configuration — `folder add`, `folder tag`,
  `folder describe`, prune, `b new` — keeps the keys projekt does not know: a
  key of your own, one from a newer release, a YAML merge key `<<: *anchor`
- **config**: `--config` naming a file that does not exist stops with
  "config file not found" instead of creating an empty file and listing
  nothing; the default file is still created on first use
- **config**: a file included twice, once through a link and once by its real
  path, is read once instead of listing its folders twice
- **folder add**, **b new**: a folder an included file already declares is
  left alone, so `doctor` no longer reports it as configured more than once
- **b apply**: `b apply --set key=value` remembers the new value, so the next
  plain `b apply` keeps it instead of rendering the old answer back
- **folder**: `--output tsv` keeps each row on one line: a tab or line break
  inside a cell, such as a multi-line description, becomes a space
- **folder exec**: flags after the command belong to the command:
  `folder exec rg -q foo` runs `rg -q foo` in every folder instead of taking
  `-q` as `--quiet`, and `-t go` is no longer read as `--tags`. Put projekt's
  own flags before the command
- **folder current**: `folder current -q` outside a project prints nothing at
  all and answers with the exit code alone, as its help promises
- **t new**, **b new**: tab after `--set` on `t new app ./myapp` or
  `b new app myapp` offers the template's keys again, instead of failing on
  the destination as if it were another template
- **projekt**, **b**: `version`, `completion <shell>`, `help` and every
  `projekt template` command run even when the configuration cannot be read.
  A configuration that is valid YAML but does not decode now stops the other
  commands up front, like an unreadable one
- **config edit**: after editing, `config edit` reports a working tree whose
  project no longer exists, the same report `config check` gives
- **doctor**: `projekt doctor -o <unknown>` refuses the format straight away
  instead of running every check first
- **worktree list**: tab after `--tags` offers the tags sorted, like every
  other `--tags`
- **folder**, **worktree**, **t**: paths are compared with their links
  resolved, so a project under a linked folder — `/var` on macOS is
  `/private/var` — is recognised by `folder current`, `worktree list` no
  longer reports `NOT A WORKTREE`, `t sync` sees the store as a repository,
  and `folder add` does not register one folder twice under two spellings
- **config**: a file that parses but does not decode — `priority: abc`,
  `tags: foo` — stops every command that writes, instead of being written back
  as the fragment that did decode
- **init**: `projekt init <shell>` prints the integration even when the
  configuration is broken, so `pj` stays available to go and fix it
- **pj**: `pj --anything` and a project whose name starts with a dash reach
  `folder get` as a name rather than a flag; the fish function no longer
  splits a path holding a newline; `cd` is passed `--`
- **config edit**, **folder open**: `EDITOR` may hold quotes, so an editor
  whose path has a space in it works
- **folder prune**, **worktree prune**: an entry that is gone but comes from an
  included file is reported rather than claimed forgotten, and no longer stops
  the rest of the prune
- **folder sync**: clones running beside each other never wait on a terminal
  question nobody can see; an unreadable repository path is reported as such
  rather than as "already exists"
- **pj -**: goes back only to the exact project visited, never to a loose match
  when that project is gone
- **config**, **history**, **trust**: files are flushed to disk before they
  replace the old one, and a name or path holding a tab or newline is not
  recorded in the jump history, where it could not be read back
- **b**: `b check` rejects a name that is a path; two recipes differing only
  in `.yaml`/`.yml` are listed once; `~user` is not mistaken for a path
- **release**: `scripts/release.sh` no longer leaves a stray blank line at
  the top of the changelog; `make lint` fails on unformatted files
- **t**, **b**, **folder**: a user name and password in a git URL are hidden
  in every message that prints the URL
- **folder**, **b**: a folder that merely sits inside another repository is
  no longer taken for a repository. `b new` initialises the new project
  instead of rewriting the outer repository's `origin`; `folder sync` refuses
  an existing folder that is not a repository instead of adding remotes,
  worktrees and pulls to the one around it; `folder status`, `folder archive`
  and `worktree prune` stop reporting on or pruning the outer repository
- **folder check**: an origin is accepted only when it is the configured
  repository on the configured server, by SSH or HTTPS, with or without
  `.git` — not any URL that happens to contain the name. An unknown server, a
  folder that is not a repository and a wrong origin now fail the command;
  a repository not cloned yet is still only reported
- **folder move**, **folder archive**: only a move across filesystems falls
  back to copying. Any other refused rename is reported and the folder left
  where it was, instead of being copied and then half-removed; a copy that
  fails takes its partial target with it, so the command can be retried
- **folder sync**: in parallel runs, a `GIT_SSH_COMMAND` of your own — a key,
  a jump host — is kept and told not to ask questions, instead of replaced;
  a `GIT_SSH` program is left alone
- **folder add --from-remote**: a GitLab server that keeps answering with the
  same next page is reported instead of asked forever

### Performance

- **t diff**, **t apply**: a large file with a small change is compared in a
  few MB instead of hundreds, the common start and end being matched first
- **folder sync --pull**: a repository is fetched once, and its working trees
  fast-forwarded from that fetch, instead of fetched again for every tree
- **pj**, completion: the workspace regex is compiled once per pattern per
  process and the configuration is no longer validated — every folder stat'ed —
  on every jump; `config check` and `doctor` still do that
- **t**: the environment, user, host and store are read once per render
  instead of once per path segment and per file, and the data files once
  instead of twice

### Security

- **worktree add**: a `.worktrees` folder that is a symbolic link is
  refused, so a cloned repository cannot make a new working tree land outside
  the project; a branch or a name starting with a dash, and a name that is a
  path, are refused before git is asked
- **t**, **b**, **folder archive**: a relative `XDG_DATA_HOME` is ignored and
  a missing `HOME` is an error, so the template store, the boilerplate store
  and the archive can no longer land in the current folder
- **t**, **b**: the record, the base copies under `.projekt/base` and a
  template's manifest are never read or written through a link, and a
  record entry that is a link is read as the link; a committed
  `.projekt/base` link can no longer carry `t apply --prune` outside the
  project. An inherited `GIT_DIR` no longer decides whether a store is yours
- **folder add --from-remote**: `GH_ENTERPRISE_TOKEN` goes only to the
  `GH_HOST` it names, a redirect to another host or to plain http is refused,
  and a repository name holding a `.git` segment is skipped
- **t apply**: `--force` and `--prune` no longer delete, and `t diff` no
  longer reads, a file outside the project that a committed
  `.projekt/template.yaml` names through `..`, an absolute path or a link; the
  apply stops and names the entry
- **t trust**: trusting a file template covers the `x.vars.yaml` and
  `x.data.yaml` beside it, so a `t sync` that adds an `after:` hook there asks
  again before it runs
- **t**, **b**: a template in a repository git cannot read — a worktree whose
  main checkout is gone, a `safe.directory` refusal, git missing — is treated
  as coming from a remote and asks before running a command, instead of being
  trusted as your own
- **t**, **b**: with no home directory, trust is neither read nor written,
  instead of going to `projekt/trust.yaml` under the current folder
- **t check**, **t new --dry-run**: the `output` commands in a manifest's
  defaults and in its `name:` are no longer run
- **config trust**: `projekt config trust` accepts any spelling of an
  included file — relative, through a link — and trusts the file its hooks
  are checked against; a path the configuration does not include is refused.
  An include reached through a linked path is asked about once more
- **config**, **pj**: a relative `XDG_CONFIG_HOME` or `XDG_STATE_HOME` is
  ignored, as the XDG specification says, and an unset `HOME` is an error, so
  the configuration and the jump history can no longer land in the current
  folder, where a repository could ship its own
- **folder add --from-remote**: a token from the environment goes only to
  the host it is for, as with `gh` and `glab`: `GITHUB_TOKEN`/`GH_TOKEN` to
  github.com, `GH_ENTERPRISE_TOKEN` to any other GitHub server, `GITLAB_TOKEN`
  to gitlab.com or the `GITLAB_HOST` it names — and no token over plain
  `http://`. A repository name from the server that is absolute or holds `..`
  is skipped rather than checked out outside the workspace
- **folder**, **t**, **doctor**: `git` and `cp` are resolved to an absolute
  path on `PATH` before they run, in one place, and one that resolves through
  a relative `PATH` entry is refused rather than run from the folder being
  worked on
- **pj**: a hook declared in an included configuration file that sits in a
  repository is handed to your shell only once that version of the file is
  trusted — `pj` asks on a terminal, refuses and says so elsewhere — and
  `projekt config trust <file>` allows it beforehand. Your own file is yours
- **t**, **b**: a symbolic link in a template store that points outside it is
  refused wherever the store is read — rendering, `include`, `t show`, the
  manifest, data and ignore files, shared partials — so a cloned store cannot
  copy a file of this machine into the output. A link a template writes is
  never written *through*: a later file of the same template cannot land
  outside the destination, by `t new` or by `t apply`
- **t**, **b**: `.Env`, `env` and `expandenv` are behind the same trust as
  `output`: a template from a cloned store reads the environment only once
  you trust it, so a token in the shell cannot be copied into a file
- **b**, **t**, **folder**: a repository URL, a branch and a path are passed
  to git after `--`, and a URL starting with a dash is rejected, so a recipe's
  `repo:` cannot smuggle an option such as `--upload-pack`
- **pj**: bash completion no longer passes project names through `compgen
  -W`, which expands each one as shell code; a workspace child named
  `$(...)` is offered literally instead of run
- **folder note**, **folder status --notes**: a project note that is a
  symbolic link, or sits under a linked `.projekt`, is neither read nor
  written, so a cloned repository cannot point the note at another file

## [4.1.0]

### Added

- **t**, **b**: `t trust [template...]` and `b trust [recipe...]` let a
  template or recipe from a cloned store run its commands, as it is now —
  beforehand, so CI or `folder exec` can use it without a terminal to ask on.
  Trusting a recipe trusts the template it renders too
- **t**: a template can say what its output is called with `name:` in its
  `.vars.yaml`, so `t new license` writes `LICENSE` without `--name`. The name
  is rendered like a default, so `name: "{{ .Values.slug }}.md"` names the
  file after an answer, and `--name` still overrides it

### Fixed

- **config**: a command that changes the configuration — `folder add`,
  `remove`, `move`, `tag`, `prune`, `worktree add`/`remove`, `add --discover`
  — now changes only the entries it touches. Comments, blank lines, key order,
  quoting, indentation and the spelling of keys such as `gitServers` survive,
  a new entry no longer spells out `prefix: ""`, `regex: ""` and
  `priority: 0`, and a configuration linked in from a dotfiles repository is
  written through the link instead of being replaced by a copy
- **release**: `scripts/release.sh --no-push` stops before the push again,
  instead of tagging and pushing anyway — the option parser reserves `--no-*`
  for the negated form of a switch, so the flag never reached its variable

### Security

- **t**, **b**: a template, recipe or starting point that sits in a git
  repository with a remote — a store cloned with `t init`, which every
  `t sync` can change — no longer runs its `after:` hooks or `output` calls
  until you trust it. On a terminal it asks first, naming the command; anywhere
  else the command is refused with the `t trust`/`b trust` line that allows
  it. Trust covers the exact content, shared `.templates` included, so any
  later change asks again. Stores you keep yourself are not affected


## [3.1.0]

### Added

- **t**: `t apply` and `t diff` given no template cover every template the
  project records, so nothing has to be remembered about a project to bring it
  up to date — which makes `projekt folder exec -t work -- t diff` a drift
  report over every project at once. The project to work on moved to
  `--project`/`-C`, leaving the positional arguments for the templates
- **b**: create a project from its recipes again with `b apply` — `b new` now
  leaves the recipe, its answers and the template it rendered in the project's
  `.projekt/template.yaml`, so a recipe's own questions and commands are not
  lost the way they were when only the template was remembered
- **t**, **b**: tab completion for `--set` — the keys come from the template's
  manifest or from the `.Values` it reads, a `choice` and a `bool` complete
  their answers, and `--project` completes folders rather than files
- **t**: turn a project into a template with `t add --replace myapp=Name`,
  which rewrites the literals in the file contents *and* in the file names,
  longest first, leaves anything that is not text alone, and writes the
  `.vars.yaml` the `.Values` targets imply with the project's own values as
  the defaults
- **b**: validate a recipe, or the whole store, with `b check` — every problem
  at once rather than the first, the template it renders checked too, and the
  questions it asks compared with what that template actually reads
- **t**: the `after` commands of a template or a recipe are not run again by an
  apply unless `--hooks` says so; they ran when the project was created
- **t**: validate a template, or the whole store, with `t check` — every file
  and path segment parsed, every shared template resolved, the whole tree
  rendered with the manifest's own defaults, and what the manifest asks for
  compared with what the template reads. Exits non-zero on an error, so CI can
  gate on it
- **t**: keep the template store in a repository — `t init <repo>` clones one
  into it, `t sync` brings it up to date, fast-forward only, so the same
  templates reach your other machine without being copied by hand
- **t**: apply a template to a project again with `t apply`, and see what it
  would do with `t diff` — a unified diff, exiting non-zero when a project has
  drifted, so CI can gate on it. A project records what it was rendered from in
  `.projekt/template.yaml`, so the values are replayed without being given
  again, a file edited by hand is a conflict and kept unless `--force`, and one
  the template no longer writes is kept unless `--prune`
- **t**: run commands once the files are there with `after:` in the manifest —
  `go mod tidy`, `git init` — rendered first so a command can be conditional,
  printed before it runs, and skippable with `--no-hooks`. `b` runs a
  template's own commands too, before the recipe's
- **t**: give a template delimiters of its own with `delims: ["<%", "%>"]` in
  its `.vars.yaml`, so one that writes GitHub Actions, Helm charts or other Go
  templates stops escaping its own syntax

### Changed

- The module lives at `gitlab.com/dynamo-tools/projekt`, so import it — and
  `go install` it — under that path rather than the old
  `gitlab.com/dynamo.foss/projekt`

### Fixed

- **t**: a generated or hand-written `.vars.yaml` no longer round-trips with
  empty `prompt`, `type` and `choices` keys on every variable

- **t**: a path segment naming a value nobody set wrote a folder called
  `<no value>`; it is refused now, the way an empty or escaping segment
  already was

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
