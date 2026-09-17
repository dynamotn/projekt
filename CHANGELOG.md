# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-09-17

### Added

- **Module loading on demand.** `init.sh` accepts a module set, either as a
  leading `--modules` argument or through the `DYBATPHO_MODULES` environment
  variable, and resolves the dependencies between modules for you. Use
  `--modules all` for the whole library.

  ```sh
  . dybatpho/init.sh --modules git semver
  DYBATPHO_MODULES="git semver" . dybatpho/init.sh
  ```

- `dybatpho::load` widens the module set after bootstrap, `dybatpho::module_loaded`
  answers whether a module is available, and `dybatpho::module_list` prints the
  `loaded`, `all`, `core`, or `optional` module names.
- `example/init_modules.sh`, showing how to request a module set and widen it at
  run time.
- `doc/init.md`, generated from `init.sh` like every module reference.

### Changed

- **BREAKING:** sourcing `init.sh` with no argument now loads only the core
  modules — `string`, `logging`, `helpers`, `process`, `file` and `secret`.
  Previously it loaded every module. A script that uses anything else has to ask
  for it:

  ```sh
  . dybatpho/init.sh               # before: everything; now: core only
  . dybatpho/init.sh --modules all # the previous behavior
  . dybatpho/init.sh --modules git # or name what the script actually uses
  ```

  Without the change, the script fails on the first call into an unloaded
  module. `dybatpho::module_list loaded` shows what a shell currently has.
- Project editor settings moved from `.neoconf.json` to `.vscode/settings.json`,
  which VS Code reads natively and
  [codesettings.nvim](https://github.com/mrjones2014/codesettings.nvim) feeds
  into Neovim's LSP configuration. The file pins the two bash-language-server
  settings without which no `dybatpho::` function resolves in an editor.

[2.0.0]: https://github.com/dynamotn/dybatpho/releases/tag/v2.0.0
