# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive unit tests for core packages (70%+ coverage)
- CI/CD pipeline with GitHub Actions
- golangci-lint configuration
- Input validation for configuration
- GoDoc comments for all exported functions
- New log level: `warn`
- Environment helper methods: `IsDebug()`, `IsInfoOrAbove()`
- Configuration validation on load

### Changed
- **BREAKING**: Updated to Go 1.21 (from 1.18)
- Updated dependencies to latest versions:
  - `github.com/spf13/cobra` v1.6.1 → v1.8.1
  - `github.com/spf13/viper` v1.13.0 → v1.19.0
  - `github.com/samber/lo` v1.33.0 → v1.47.0
- Replaced deprecated `io/ioutil` with `os` package
- Improved error handling in `ParseConfig()`
- Optimized `appendToParsedFolder()` to avoid unnecessary map allocation
- Renamed constant `DEFAULT_REGEX_WORKSPACE` → `defaultRegexWorkspace` (Go convention)
- Removed unnecessary `else` statements
- Improved string trimming efficiency in `CheckFolderExist()`

### Fixed
- Error handling in `ParseConfig()` now properly returns errors
- Regex compilation errors are now caught and logged
- Symlink detection improved in folder parsing

### Improved
- Code documentation with comprehensive GoDoc comments
- README with detailed usage examples and configuration guide
- Performance optimizations in folder parsing
- Better error messages and warnings

## [1.1.0] - Previous Release

### Added
- Basic folder management functionality
- Template system
- Boilerplate generation
- Workspace support with regex matching

## [1.0.0] - Initial Release

### Added
- Initial release with basic project folder management
- Configuration file support
- Simple CLI interface

[Unreleased]: https://gitlab.com/dynamo.foss/projekt/compare/v1.1.0...HEAD
[1.1.0]: https://gitlab.com/dynamo.foss/projekt/compare/v1.0.0...v1.1.0
[1.0.0]: https://gitlab.com/dynamo.foss/projekt/releases/tag/v1.0.0
