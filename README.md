# Claude Session Browser

A terminal user interface (TUI) for browsing and resuming Claude Code sessions.

![GitHub release (latest by date)](https://img.shields.io/github/v/release/davidpp/claude-session-browser)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/davidpp/claude-session-browser/release.yml)
![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)

## Features

- 📁 Browse Claude Code sessions with a clean TUI
- ⚡ Fast and lightweight - parses sessions on demand
- 📋 Copy resume commands to clipboard with one keystroke
- 🕐 Relative timestamps (e.g., "2 hours ago")
- 📍 Auto-detect current project directory
- 💻 Cross-platform support (macOS, Linux, Windows)

## Installation

### Download Pre-built Binary (Recommended)

Download the latest release for your platform from the [Releases](https://github.com/davidpp/claude-session-browser/releases) page.

#### macOS

```bash
# For Apple Silicon (M1/M2/M3)
curl -L https://github.com/davidpp/claude-session-browser/releases/latest/download/claude-session-browser-darwin-arm64.tar.gz | tar xz
sudo mv claude-session-browser-darwin-arm64 /usr/local/bin/claude-session-browser

# For Intel Macs
curl -L https://github.com/davidpp/claude-session-browser/releases/latest/download/claude-session-browser-darwin-amd64.tar.gz | tar xz
sudo mv claude-session-browser-darwin-amd64 /usr/local/bin/claude-session-browser
```

#### Linux

```bash
# For AMD64
curl -L https://github.com/davidpp/claude-session-browser/releases/latest/download/claude-session-browser-linux-amd64.tar.gz | tar xz
sudo mv claude-session-browser-linux-amd64 /usr/local/bin/claude-session-browser

# For ARM64
curl -L https://github.com/davidpp/claude-session-browser/releases/latest/download/claude-session-browser-linux-arm64.tar.gz | tar xz
sudo mv claude-session-browser-linux-arm64 /usr/local/bin/claude-session-browser
```

#### Windows

Download the appropriate `.zip` file from the [Releases](https://github.com/davidpp/claude-session-browser/releases) page:
- `claude-session-browser-windows-amd64.zip` for 64-bit systems
- `claude-session-browser-windows-arm64.zip` for ARM64 systems

Extract and add to your PATH.

### Install from Source

If you have Go 1.24+ installed:

```bash
go install github.com/davidpp/claude-session-browser@latest
```

### Build from Source

```bash
git clone https://github.com/davidpp/claude-session-browser
cd claude-session-browser
go build -o claude-session-browser
```

## Usage

Simply run the command from anywhere:

```bash
claude-session-browser
```

The app will automatically find your Claude sessions in `~/.claude/projects/`. If you're in a directory with an active Claude project, it will open that project directly.

### Keyboard Shortcuts

- `↑↓` or `j/k` - Navigate through sessions
- `Enter` - Copy resume command to clipboard
- `r` - Refresh session list
- `q` - Quit
- `Ctrl+C` - Force quit

### Command Line Options

```bash
# Show help
claude-session-browser --help

# Use a custom Claude directory
claude-session-browser --claude-dir ~/my-claude-projects
```

## How It Works

1. The app reads JSONL session files from your Claude projects directory
2. Sessions are displayed with relative timestamps and truncated IDs
3. Select a session to see details including summary and full JSON data
4. Press Enter to copy the resume command to your clipboard
5. Paste the command in your terminal to resume the session

## Development

### Prerequisites

- Go 1.24 or higher
- Make (optional, for using Makefile)

### Building

```bash
# Development build
go build

# Production build with optimizations
go build -ldflags="-s -w" -o claude-session-browser

# Run tests
go test ./...
```

### Project Structure

```
claude-session-browser/
├── main.go                 # Entry point
├── internal/
│   ├── model/             # Data models
│   ├── parser/            # JSONL parser
│   ├── ui/                # TUI components
│   └── clipboard/         # Clipboard manager
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [atotto/clipboard](https://github.com/atotto/clipboard) - Cross-platform clipboard support