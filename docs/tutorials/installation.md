# Installation

## Requirements

- Go 1.21 or higher (for generics support)
- No external dependencies

## Install with Go Modules

Add pipz to your project:

```bash
go get github.com/zoobzio/pipz
```

## Import in Your Code

```go
import "github.com/zoobzio/pipz"
```

## Version Management

To use a specific version:

```bash
go get github.com/zoobzio/pipz@v0.6.0
```

To update to the latest version:

```bash
go get -u github.com/zoobzio/pipz
```

## Verify Installation

Create a simple test file to verify the installation:

```go
// test.go
package main

import (
    "context"
    "fmt"
    "github.com/zoobzio/pipz"
)

func main() {
    // Create a simple processor
    double := pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2
    })
    
    result, err := double.Process(context.Background(), 21)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Result: %d\n", result) // Output: Result: 42
}
```

Run it:

```bash
go run test.go
```

## Development Setup

If you want to contribute or explore the source:

```bash
# Clone the repository
git clone https://github.com/zoobzio/pipz.git
cd pipz

# Run tests
make test

# Run benchmarks
make bench

# Run linters
make lint
```

## Editor Support

pipz uses standard Go idioms and generics. Any editor with Go 1.21+ support will provide:
- Full autocomplete
- Type checking
- Inline documentation

### Recommended Extensions

**VS Code:**
- [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go)

**IntelliJ/GoLand:**
- Built-in Go support

**Vim/Neovim:**
- [vim-go](https://github.com/fatih/vim-go)
- [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig) with gopls

## Next Steps

- [Quick Start](./quickstart.md) - Build your first pipeline
- [Core Concepts](../learn/core-concepts.md) - Understand pipz fundamentals
- [Examples](../examples/) - Learn from real implementations