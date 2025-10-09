# headless

A CLI tool for controlling a headless Chrome browser from the command line, with built-in AI agent capabilities.

Uses the excellent `chromedp` library for Go to do the Chrome stuff, and loosely
based on some experience I had building sketch.dev's browser tool.

# Warning

This was almost entirely vibe-coded.

## Installation

```bash
go install github.com/philz/ctr-agent/headless/cmd/headless@latest
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

## License

MIT
