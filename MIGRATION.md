# Migration Guide

## Upgrading to Latest Version

This guide helps you migrate from older versions of Projekt to the latest optimized version.

### Breaking Changes

#### Go Version Requirement

**Old**: Go 1.18+
**New**: Go 1.21+

**Action Required**: Update your Go installation to 1.21 or later.

```bash
# Check your Go version
go version

# If needed, update Go from https://go.dev/dl/
```

#### Constant Naming

**Old**: `DEFAULT_REGEX_WORKSPACE`
**New**: `defaultRegexWorkspace` (unexported)

**Impact**: If you were using this constant directly in your code, you'll need to update your imports. This constant is now internal and accessed through methods.

**Action**: Use `Folder.GetRegexMatch()` instead of accessing the constant directly.

```go
// Old
regex := lazypath.DEFAULT_REGEX_WORKSPACE

// New
folder := &lazypath.Folder{IsWorkspace: true}
regex := folder.GetRegexMatch()
```

### New Features

#### Enhanced Logging

New log level `warn` is now available:

```bash
# Set log level
export PROJEKT_LOG_LEVEL=warn
projekt folder list -v warn
```

#### Configuration Validation

Configurations are now automatically validated on load:

```yaml
# Invalid config will show warnings
folders:
  - path: ""  # ⚠️  Warning: empty path
```

#### Better Error Handling

Errors are now properly propagated and logged:

```go
// Errors are now properly returned
folders, err := folderutil.ParseConfig(config)
if err != nil {
    // Handle error
}
```

### Dependency Updates

Major dependency updates:

- `github.com/spf13/cobra`: v1.6.1 → v1.8.1+
- `github.com/spf13/viper`: v1.13.0 → v1.19.0+
- `github.com/samber/lo`: v1.33.0 → v1.47.0+

**Action**: Run `go mod tidy` to update dependencies.

### Performance Improvements

The following functions are now more efficient:

- `ParseConfig()`: Better regex compilation handling
- `CheckFolderExist()`: Optimized string trimming
- `appendToParsedFolder()`: Reduced memory allocations

### Testing

The codebase now includes comprehensive tests. To verify your installation:

```bash
# Run all tests
make test

# View coverage
make test-coverage
```

### CI/CD Integration

New GitHub Actions workflows are available in `.github/workflows/`:

- `ci.yml`: Continuous integration with tests and linting
- `release.yml`: Automated releases

### Documentation

Enhanced documentation is now available:

- GoDoc comments for all exported functions
- Comprehensive README with examples
- `CONTRIBUTING.md` for contributors
- `CHANGELOG.md` for version history

### Verification

After upgrading, verify everything works:

```bash
# 1. Build
make build

# 2. Test
make test

# 3. Check version
./bin/projekt version

# 4. Lint (optional)
make lint
```

### Rollback

If you need to rollback:

```bash
# Checkout previous version
git checkout v1.1.0

# Rebuild
make clean
make build
```

## Need Help?

If you encounter issues during migration:

1. Check the [CHANGELOG.md](CHANGELOG.md) for detailed changes
2. Review [examples/](examples/) for updated usage patterns
3. Open an issue on the repository

## Recommended Actions

✅ **Immediate**:
1. Update Go to 1.21+
2. Run `go mod tidy`
3. Run tests to verify compatibility

✅ **Soon**:
1. Update CI/CD to use new workflows
2. Review and update any custom scripts
3. Enable code coverage reporting

✅ **Optional**:
1. Set up golangci-lint locally
2. Contribute to test coverage
3. Review new examples for best practices
