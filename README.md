# Projekt

[![CI](https://github.com/YOUR_USERNAME/projekt/workflows/CI/badge.svg)](https://github.com/YOUR_USERNAME/projekt/actions)
[![Go Report Card](https://goreportcard.com/badge/gitlab.com/dynamo.foss/projekt)](https://goreportcard.com/report/gitlab.com/dynamo.foss/projekt)
[![GoDoc](https://godoc.org/gitlab.com/dynamo.foss/projekt?status.svg)](https://godoc.org/gitlab.com/dynamo.foss/projekt)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A smart command-line tool to manage and work with your project folders efficiently.

## Features

Projekt provides a complete solution for your terminal to:

- **Folder Management**: Add/update/remove Git project folders with intelligent tracking
- **Quick Navigation**: Switch between project folders instantly using `pj` or `projekt folder go`
- **Template System**: Create template files in your project folders
- **Boilerplate Generation**: Create new projects from various boilerplate sources
- **Workspace Support**: Organize multiple projects within workspaces
- **Regex Matching**: Filter and match folders using custom regex patterns

## Installing

### From Source (Linux, macOS)

You must have a working Go 1.21+ environment:

```bash
# Clone the repository
git clone https://gitlab.com/dynamo.foss/projekt.git
cd projekt

# Build and install
sudo make all
```

This will install three binaries:
- `projekt` - Main CLI tool
- `pj` - Alias for quick folder navigation
- `t` - Template command
- `b` - Boilerplate command

### Using Go Install

```bash
go install gitlab.com/dynamo.foss/projekt/cmd/projekt@latest
```

## Quick Start

### 1. Initialize Configuration

```bash
projekt init
```

### 2. Add Your Project Folders

```bash
# Add a single project folder
projekt folder add /path/to/your/project

# Add a workspace (folder containing multiple projects)
projekt folder add /path/to/workspace --workspace --regex "^project-.*"
```

### 3. List Your Folders

```bash
# List all folders with short names
projekt folder list

# List with full details
projekt folder list --plain

# Get a specific folder path
projekt folder get my-project
```

### 4. Navigate to Folders

```bash
# Use with cd command
cd $(pj my-project)

# Or create a shell alias
alias pj='cd $(projekt folder get $1)'
```

## Usage

### Folder Management

```bash
# Add a folder
projekt folder add /path/to/project --prefix myprefix

# Add a workspace with regex matching
projekt folder add ~/workspace --workspace --regex "^(app|lib)-.*"

# List folders
projekt folder list
projekt folder list --short-only
projekt folder list --plain --no-headers

# Remove a folder
projekt folder remove /path/to/project

# Get folder path by short name
projekt folder get myprefix-project
```

### Templates

```bash
# Create a template file
projekt template <template-name> [flags]
```

### Boilerplate

```bash
# Create a new project from boilerplate
projekt boilerplate <source> [flags]
```

## Configuration

Projekt stores its configuration in `~/.config/projekt/config.yaml` (or `$XDG_CONFIG_HOME/projekt/config.yaml`).

Example configuration:

```yaml
folders:
  - path: /home/user/projects/myapp
    prefix: app
    is_workspace: false
    priority: 0
  
  - path: /home/user/workspace
    prefix: ws
    is_workspace: true
    regex: "^(frontend|backend)-.*"
    priority: 1
```

### Configuration Options

- `path`: Absolute path to the folder
- `prefix`: Prefix for the short name (optional)
- `is_workspace`: Whether this is a workspace containing multiple projects
- `regex`: Regular expression to match subfolder names (workspace only)
- `priority`: Priority for ordering (higher priority first)

## Environment Variables

- `PROJEKT_LOG_LEVEL`: Set log level (`debug`, `info`, `warn`, `error`)

## Examples

### Example 1: Organizing Multiple Projects

```bash
# Add your workspace
projekt folder add ~/go/src/github.com/myorg --workspace --prefix gh --regex ".*"

# This creates short names like:
# gh-project1
# gh-project2
# gh-library
```

### Example 2: Quick Navigation

```bash
# Add to your .bashrc or .zshrc
pj() {
  cd "$(projekt folder get $1)"
}

# Now you can:
pj gh-project1  # Jumps to ~/go/src/github.com/myorg/project1
```

### Example 3: Filter Projects by Pattern

```bash
# Only match folders starting with "app-"
projekt folder add ~/projects --workspace --regex "^app-.*"

# Results in short names like:
# app-frontend
# app-backend
# app-mobile
```

## Development

### Running Tests

```bash
go test ./...

# With coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Running Linters

```bash
golangci-lint run
```

### Building

```bash
# Build all binaries
make all

# Build individual binaries
make projekt
make t
make b
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Documentation

For more detailed documentation, see the `doc` folder or run:

```bash
projekt --help
projekt folder --help
```

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [lo](https://github.com/samber/lo) - Utility functions

