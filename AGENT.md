# AGENT.md

## Purpose

`dybatpho` is a Bash utility library. Its primary development model is
**spec-driven**: define behavior in source/spec functions, then reuse the same
metadata to generate the parser, help, completions, JSON schema, and man page.

This document describes the repository workflow and conventions to preserve.

## Repository structure

- `src/` — Bash modules, each covering one functional area.
- `init.sh` — required entrypoint; source it before using the library.
- `test/` — Bats tests for each module, such as `test/cli.bats`.
- `example/` — complete, runnable usage examples for every public module.
- `doc/` — API documentation generated from source comments.
- `doc/spec/` — Spec Kit-style feature specifications.
- `scripts/test.sh` — full test and coverage runner.
- `.mise.toml` — standard tasks such as `mise run test` and `mise run doc`.

## Module scope

Every module in `src/` must have a clear responsibility and expose public
functions under the `dybatpho::` namespace. Every public module must have a
corresponding complete example in `example/` (normally
`example/<module>_ops.sh`) **and a specification in `doc/spec/<module>.md`**.
When changing a module, read its source, tests, API documentation, matching
specification, and example before editing.

### Specification requirements

**Every module in `src/` MUST have `doc/spec/<module>.md`. A change that adds a
module or public behavior without its spec is incomplete and must not be
reported as done.** This is not conditional on the size of the change.

Generate or update the spec in the same change that touches the module:

- **New module** — write `doc/spec/<module>.md` before the work is considered
  complete, and register it in both the Spec Files and Source Mapping lists of
  `doc/spec/README.md`.
- **New or changed public function, argument, rule, output, or exit code** —
  add or update the matching user story, functional requirement (`FR-xxx`),
  edge case, and integration test (`IT-xxx`) entries.
- **Removed behavior** — delete the requirements it covered rather than leaving
  them describing code that no longer exists.

Every spec follows the same section order, taken from the existing files:

1. Header (`# Feature Specification: <title>`, `Feature Branch`, `Status`,
   `Input` naming the source, tests, docs, and example it was derived from)
2. `## Problem Statement *(mandatory)*`
3. `## Business Value *(mandatory)*`
4. `## User Scenarios & Testing *(mandatory)*` — numbered, prioritized user
   stories, each with an **Independent Test** and numbered
   **Acceptance Scenarios** in Given/When/Then form, plus an example workflow
5. `## Edge Cases`
6. `## Requirements *(mandatory)*` — `FR-001`-style functional requirements
   written with MUST, and `### Key Entities` when the feature owns data
7. `## Success Criteria *(mandatory)*` — `SC-001`-style measurable outcomes
8. `## Integration Tests *(mandatory)*` — `IT-001`-style entries that map to
   real cases in `test/<module>.bats`
9. `## Acceptance Criteria *(mandatory)*`

Specs describe observable behavior and contracts, not implementation lines.
Requirements must be traceable: every `FR-xxx` should be enforced by code and
every `IT-xxx` should correspond to a test that exists.

Verify no module is missing a spec before finishing:

```bash
comm -23 \
  <(ls src/*.sh | xargs -n1 basename | sed 's/\.sh$/.md/' | sort) \
  <(ls doc/spec/*.md | xargs -n1 basename | sort)
```

The command must print nothing. Any output is a missing spec that must be
written before the change is complete.

### Complete example requirements

A complete example is a user-facing, executable workflow rather than a list of
isolated function calls. It must:

- source `init.sh` and run successfully with `bash example/<name>.sh`;
- demonstrate the module's primary public APIs, including newly added options
  or behavior;
- show realistic input, expected output, and relevant success/failure paths;
- run non-interactively without credentials, network access, or machine-local
  assumptions; use temporary fixtures, dry-run mode, or command mocks where
  needed;
- clean up temporary files and avoid modifying the repository or user data.

When adding or changing a public API, update the corresponding example in the
same change. If a module has no example, add one before considering the work
complete. Validate every example with `bash -n example/*.sh` and execute new or
changed examples.

| Module | Primary responsibility | Tests / documentation |
| --- | --- | --- |
| `array.sh` | Create, read, join, filter, and manipulate Bash arrays | `test/array.bats`, `doc/array.md`, `doc/spec/array.md` |
| `archive.sh` | Create, extract, and inspect archives | `test/archive.bats`, `doc/archive.md`, `doc/spec/archive.md` |
| `cli.sh` | Declarative parser, help, subcommands, completions, and CLI artifacts | `test/cli.bats`, `doc/cli.md`, `doc/spec/cli.md` |
| `config.sh` | Load dotenv, JSON/YAML configuration, precedence, typed schema validation, and configuration docs | `test/config.bats`, `doc/config.md`, `doc/spec/config.md` |
| `date.sh` | Portable date/time parsing, formatting, and calculations | `test/date.bats`, `doc/date.md`, `doc/spec/date.md` |
| `file.sh` | Path, file, directory, and temporary-file helpers | `test/file.bats`, `doc/file.md`, `doc/spec/file.md` |
| `git.sh` | Safe repository, branch, commit, and Git operations | `test/git.bats`, `doc/git.md`, `doc/spec/git.md` |
| `helpers.sh` | Argument validation, command lookup, retry, and common helpers | `test/helpers.bats`, `doc/helpers.md`, `doc/spec/helpers.md` |
| `json.sh` | Query, validate, pretty-print, and convert JSON/YAML | `test/json.bats`, `doc/json.md`, `doc/spec/json.md` |
| `lock.sh` | Portable `mkdir`-based process locks, waiting, stale reclaim, and `with_lock` | `test/lock.bats`, `doc/lock.md`, `doc/spec/lock.md` |
| `logging.sh` | Log levels, text/JSON logging, banners, and Bash tracing | `test/logging.bats`, `doc/logging.md`, `doc/spec/logging.md` |
| `network.sh` | Curl wrappers, retries, JSON requests, and HTTP metadata | `test/network.bats`, `doc/network.md`, `doc/spec/network.md` |
| `notification.sh` | Webhook notifications and JSON payloads | `test/notification.bats`, `doc/notification.md`, `doc/spec/notification.md` |
| `os.sh` | OS, architecture, and environment detection | `test/os.bats`, `doc/os.md`, `doc/spec/os.md` |
| `process.sh` | Traps, cleanup, dry-run, and process lifecycle | `test/process.bats`, `doc/process.md`, `doc/spec/process.md` |
| `secret.sh` | Read, mask, and store secrets safely | `test/secret.bats`, `doc/secret.md`, `doc/spec/secret.md` |
| `semver.sh` | Semantic version parsing, comparison, and validation | `test/semver.bats`, `doc/semver.md`, `doc/spec/semver.md` |
| `string.sh` | Case conversion, matching, splitting, trimming, and predicates | `test/string.bats`, `doc/string.md`, `doc/spec/string.md` |
| `table.sh` | Plain-text and Markdown table rendering | `test/table.bats`, `doc/table.md`, `doc/spec/table.md` |
| `text.sh` | Multiline text processing, indentation, wrapping, and formatting | `test/text.bats`, `doc/text.md`, `doc/spec/text.md` |

`init.sh` loads modules according to their dependencies. Do not source a module
in isolation when it uses helpers from another module unless the source header
documents the dependency and isolated tests provide the required setup.

### Principles by module group

- **Data primitives** (`array`, `string`, `text`, `table`): keep functions
  predictable and free of unexpected file writes or logging.
- **System primitives** (`date`, `file`, `os`, `process`, `helpers`): prioritize
  GNU/BSD/BusyBox portability and return clear errors for invalid input.
- **Security primitives** (`secret`): never log or export secret values, keep
  the masking registry process-local, and validate file permissions before use.
- **Integration modules** (`git`, `network`, `notification`, `archive`, `config`,
  `json`): validate dependencies, quote paths/URLs/payloads, and do not hide
  external command failures.
- **Presentation modules** (`logging`, `cli`): reserve stdout for pipeable or
  capturable data and stderr for diagnostics; preserve text output when adding
  machine-readable output.
- **Coordination modules** (`lock`): keep operations atomic and portable
  without `flock`, always report the holder on failure, and never leave a lock
  behind on the failure path.

## Bash conventions

- Use Bash 4 or newer and always source `init.sh`.
- Keep compatibility with strict mode: `set -euo pipefail`.
- Use two-space indentation, LF line endings, and a final newline.
- Put public functions under the `dybatpho::` namespace.
- Use non-public names for internal functions, usually beginning with `__`.
- Use `printf` instead of `echo` when output must be stable or contains user data.
- Quote variables that may contain whitespace or special characters.
- Do not silently swallow errors; return clear failures following repository
  conventions.
- Validate and quote every option or variable inserted into generated shell code.

## CLI change workflow

### 1. Update the CLI spec

Define commands in shell functions using the `dybatpho::opts::*` DSL:

```bash
_spec_root() {
  dybatpho::opts::setup "Tool description" ROOT_ARGS action:"_run_root"
  dybatpho::opts::flag "Verbose output" VERBOSE --verbose alias:-v
  dybatpho::opts::param "API token" TOKEN --token env:API_TOKEN required:true
  dybatpho::opts::param "Component" COMPONENT --component \
    choices:api,worker,frontend multiple:true prompt:"Choose components"
  dybatpho::opts::cmd deploy _spec_deploy
}
```

Common attributes:

| Attribute | Meaning |
| --- | --- |
| `alias:` / `aliases:` | Alias for an option or command |
| `required:true` | Require the option to be supplied |
| `env:NAME` | Use an environment variable as the initial value |
| `prompt:"..."` | Prompt for a missing value |
| `choices:a,b` | Restrict values to a choice list |
| `multiple:true` | Accumulate values; prompts support `1,3` and `1-3` |
| `persistent:true` | Inherit an option in subcommands |
| `hidden:true` | Hide the item from public help/artifacts |
| `deprecated:"..."` | Warn on use and annotate help |
| `args:<rule>` | Validate positional arguments |

Command-line values must take precedence over `env:NAME`. Prompts run only when
the destination variable is still empty after initialization and parsing.

### 2. Generate all output from the same spec

Use the same function for every artifact:

```bash
dybatpho::generate_from_spec _spec_root "$@"
dybatpho::generate_help _spec_root
dybatpho::generate_completion _spec_root bash mytool
dybatpho::generate_completion _spec_root zsh mytool
dybatpho::generate_completion _spec_root fish mytool
dybatpho::generate_schema _spec_root mytool
dybatpho::generate_man _spec_root mytool
```

Do not maintain separate option or subcommand lists for completions, schema, or
man pages. Metadata must flow through the same DSL to prevent drift between
runtime behavior and documentation.

When checking subcommand artifacts, verify root and child options, aliases,
environment/choice/prompt/multiple/required metadata, hidden/deprecated behavior,
and that a man page contains one `.TH` with appropriate child sections.

### 3. Keep actions and lifecycle explicit

- `setup` contains the description, positional rule, and lifecycle hooks.
- `prerun` runs after validation and before the action.
- `action` contains the main logic.
- `postrun` runs after the action when the action does not exit the process.
- Successful actions should intentionally use `return 0` or `exit 0`.
- Use display options (`disp`) for `--help`, `--version`, `--schema`, `--man`,
  and other actions that do not take a value.

## Tests required for CLI changes

Add behavior-focused tests to `test/cli.bats`. Cover:

- parser, automatic/custom help, and subcommand dispatch;
- prompt input, defaults, and EOF;
- valid and invalid choices;
- `multiple:true` with names, commas, and ranges such as `1-3`;
- environment fallback and CLI precedence;
- Bash, Zsh, and Fish completions;
- valid JSON schema and nested metadata;
- root/child man page output, aliases, and hidden options.

Quick checks:

```bash
test/lib/core/bin/bats --print-output-on-failure test/cli.bats
bash -n src/cli.sh
bash -n example/cli_ux.sh
git diff --check
```

Run the complete test and coverage workflow for broad changes:

```bash
mise run test
```

## Example reference

`example/cli_ux.sh` is the reference example for advanced CLI UX:

```bash
bash example/cli_ux.sh --help
bash example/cli_ux.sh --completion bash
bash example/cli_ux.sh --schema
bash example/cli_ux.sh --man
bash example/cli_ux.sh deploy
```

When adding CLI behavior, update these together:

1. `src/cli.sh` and embedded API/usage comments;
2. `test/cli.bats`;
3. `example/` when the behavior is useful to demonstrate;
4. `doc/cli.md`;
5. `doc/spec/cli.md` when the contract, acceptance scenario, or requirement changes.

## Workflow for other module changes

1. Identify the module's public API, side effects, dependencies, and portable
   behavior.
2. Find its tests in `test/<module>.bats`; extend tests near the changed behavior.
3. Add API comments for new public functions so `scripts/doc.sh` can generate
   the reference documentation.
4. Create or update `doc/spec/<module>.md`. This step is mandatory for any
   contract or capability change, and for every new module; do not defer it.
5. Add or update the module's complete example in `example/`; do not defer
   examples for public behavior.
6. Run `bash -n` on the source, the module test, and tests for affected
   dependencies.

Keep changes composable: functions should work in command substitution,
pipelines, and conditionals without output outside their documented contract.
Functions with side effects must document them and propagate errors according
to the module convention.

## Checks by change type

| Change | Minimum checks |
| --- | --- |
| String/array/text/table | Empty input, whitespace, special characters, and module tests |
| Path/file/archive | `BATS_TEST_TMPDIR`, paths with spaces, cleanup, and permission errors |
| Date/OS/network | GNU/BSD/BusyBox fallback or the relevant command mock |
| Git/process/config | Temporary repository/config, failure paths, traps, and cleanup |
| Logging/CLI | Separate stdout/stderr, filtering, strict mode, and machine-readable output |
| JSON/YAML/notification | Escaping, malformed input, and unavailable dependencies |
| Locking/coordination | Atomic acquire, stale reclaim, release on failure paths, and `DYBATPHO_LOCK_DIR` isolation in tests |
| New module | `doc/spec/<module>.md`, `doc/spec/README.md` entries, `test/<module>.bats`, and `example/<module>_ops.sh` |
| Documentation/spec | Correct links/references and `git diff --check` |

## Completion checklist

1. Inspect the worktree first and preserve existing changes.
2. Read related source, tests, documentation, and specification.
3. Make a focused, backward-compatible change unless the contract requires otherwise.
4. Add regression tests for new or fixed behavior.
5. Add or update a complete example for every changed public module.
6. Add or update `doc/spec/<module>.md` for every changed public module, and
   confirm the missing-spec check above prints nothing.
7. Run targeted tests, `bash -n example/*.sh`, changed examples, and
   `git diff --check`.
8. Review the final diff and remove temporary artifacts.
