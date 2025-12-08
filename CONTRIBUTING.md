# Contributing to Projekt

First off, thank you for considering contributing to Projekt! It's people like you that make Projekt such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our commitment to providing a welcoming and inspiring community for all. Please be respectful and constructive.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* **Use a clear and descriptive title**
* **Describe the exact steps to reproduce the problem**
* **Provide specific examples to demonstrate the steps**
* **Describe the behavior you observed after following the steps**
* **Explain which behavior you expected to see instead and why**
* **Include logs and error messages**
* **Include your environment details** (OS, Go version, etc.)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

* **Use a clear and descriptive title**
* **Provide a step-by-step description of the suggested enhancement**
* **Provide specific examples to demonstrate the steps**
* **Describe the current behavior and explain which behavior you expected to see instead**
* **Explain why this enhancement would be useful**

### Pull Requests

* Fill in the required template
* Do not include issue numbers in the PR title
* Follow the Go coding style
* Include thoughtfully-worded, well-structured tests
* Document new code based on the Documentation Styleguide
* End all files with a newline

## Development Setup

1. **Fork and clone the repository**

```bash
git clone https://gitlab.com/dynamo.foss/projekt.git
cd projekt
```

2. **Install dependencies**

```bash
go mod download
```

3. **Build the project**

```bash
make all
```

4. **Run tests**

```bash
go test ./...
```

## Development Process

1. **Create a branch**

```bash
git checkout -b feature/my-new-feature
# or
git checkout -b fix/my-bug-fix
```

2. **Make your changes**
   - Write or update tests
   - Update documentation if needed
   - Follow the coding style

3. **Run tests and linters**

```bash
# Run tests
go test -v -race ./...

# Run linters
golangci-lint run

# Check formatting
gofmt -l .
```

4. **Commit your changes**

```bash
git add .
git commit -m "feat: add new feature"
```

Follow conventional commits:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `style:` - Code style changes (formatting, etc.)
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

5. **Push to your fork**

```bash
git push origin feature/my-new-feature
```

6. **Create a Pull Request**

## Coding Style

### Go Code Style

* Follow standard Go conventions
* Use `gofmt` to format your code
* Use meaningful variable and function names
* Keep functions small and focused
* Add comments for exported functions (GoDoc format)
* Handle errors explicitly
* Use early returns to reduce nesting

### Example

```go
// ParseConfig parses the configuration and returns a list of parsed folders
func ParseConfig(c lazypath.Config) ([]ParsedFolder, error) {
    if len(c.Folders) == 0 {
        return nil, nil
    }
    
    var result []ParsedFolder
    for _, folder := range c.Folders {
        // Process folder...
    }
    
    return result, nil
}
```

### Testing

* Write table-driven tests when possible
* Test both success and error cases
* Use descriptive test names
* Aim for at least 70% code coverage

### Example Test

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        config  lazypath.Config
        want    []ParsedFolder
        wantErr bool
    }{
        {
            name: "empty config",
            config: lazypath.Config{Folders: []lazypath.Folder{}},
            want: nil,
            wantErr: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.config)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
            }
            // More assertions...
        })
    }
}
```

## Documentation Style

* Use clear, concise language
* Provide examples where applicable
* Update README.md if you change functionality
* Add GoDoc comments for exported functions
* Update CHANGELOG.md

## Questions?

Feel free to open an issue with your question, or reach out to the maintainers.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (MIT License).

## Recognition

Contributors will be recognized in the project README and release notes.

Thank you for contributing! 🎉
