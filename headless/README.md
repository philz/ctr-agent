# headless

A CLI tool for controlling a headless Chrome browser from the command line, with built-in AI agent capabilities.

## Installation

```bash
go install github.com/philz/headless/cmd/headless@latest
```

Or build from source:

```bash
git clone https://github.com/philz/headless
cd headless
go build -o headless ./cmd/headless
```

## Usage

### Starting the Server

Start a headless Chrome browser with an HTTP control interface (runs in background by default):

```bash
headless start [-port PORT] [--foreground]
```

The server runs in background by default. Use `--foreground` to run in the current terminal. Default port is 11111.

### Remote Commands

Once the server is running, control it from another terminal:

#### Navigate to a URL

```bash
headless navigate https://example.com
```

#### Evaluate JavaScript

```bash
headless eval "document.title"
```

#### Take a Screenshot

```bash
headless screenshot output.png
```

#### Resize Browser Window

```bash
headless resize 1280 720
```

#### Read Console Logs

```bash
headless read_console
```

#### Clear Console Logs

```bash
headless clear_console
```

#### Stop the Server

```bash
headless stop
```

### Web Interface

When the server is running, open `http://localhost:11111` in your browser to access a web-based control panel with forms for all available commands.

### AI Agent Mode

Run an AI agent that can autonomously browse and interact with web pages:

```bash
export ANTHROPIC_API_KEY=your-key-here
headless ai "What is the title of example.com?"
```

The AI agent will use the running headless server by default. To run with an independent browser instance:

```bash
headless ai --standalone "Search for information about Go programming"
```

The agent has access to these tools:
- **navigate**: Navigate to URLs
- **evaluate**: Execute JavaScript and read results
- **screenshot**: Capture page screenshots

## Examples

### Basic Browser Automation

```bash
# Start the server (runs in background)
headless start

# Control the browser
headless navigate https://example.com
headless eval "document.querySelector('h1').textContent"
headless screenshot example.png

# Stop the server when done
headless stop
```

### AI-Powered Browsing

```bash
# Start the server (runs in background)
headless start

# Ask the AI to browse
export ANTHROPIC_API_KEY=sk-ant-...
headless ai "Go to example.com and tell me what the main heading says"

# Stop when done
headless stop
```

### Standalone AI Agent

```bash
export ANTHROPIC_API_KEY=sk-ant-...
headless ai --standalone "Visit github.com and take a screenshot"
```

## Architecture

- **cmd/headless**: Main CLI application
- **internal/server**: HTTP server and Chrome automation via chromedp
- **internal/browser**: Client for sending commands to the server
- **internal/ai**: AI agent with tool-calling capabilities using Claude

## Requirements

- Go 1.19 or later
- Chrome/Chromium (automatically managed by chromedp)
- Anthropic API key (for AI features)

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build -o headless ./cmd/headless
```

## License

MIT
