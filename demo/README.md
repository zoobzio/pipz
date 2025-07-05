# pipz Demo CLI

Interactive demonstrations of pipz capabilities with beautiful colored output.

## Building

```bash
cd demo
go mod tidy
go build -o pipz-demo
```

## Running

```bash
# Show help
./pipz-demo --help

# Run all demos
./pipz-demo all

# Run individual demos
./pipz-demo security    # Security audit pipeline
./pipz-demo transform   # Data transformation  
./pipz-demo universes   # Multi-tenant type universes
```

## Features

- 🎨 Colored terminal output
- 📊 Live code demonstrations
- 🔍 Interactive examples
- 📖 Syntax-highlighted code samples

## Troubleshooting

If colors don't appear:
- Make sure your terminal supports ANSI colors
- Try setting: `export FORCE_COLOR=1`
- On Windows, use Windows Terminal or similar

If formatting looks wrong:
- Ensure terminal width is at least 80 characters
- Use a monospace font in your terminal