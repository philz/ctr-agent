# Usage Examples

## Basic Usage

### Starting the Server

```bash
# Start on default port (11111) - runs in background by default
headless start

# Start on custom port
headless start -port 8080

# Start in foreground (stays attached to terminal)
headless start --foreground

# Start on custom port in foreground
headless start -port 8080 --foreground
```

### Browser Control Commands

#### Navigate to a Website

```bash
# In another terminal
headless navigate https://example.com
```

#### Execute JavaScript

```bash
# Get page title
headless eval "document.title"

# Get all links
headless eval "Array.from(document.querySelectorAll('a')).map(a => a.href)"

# Get page text content
headless eval "document.body.innerText"
```

#### Take Screenshots

```bash
# Take a screenshot
headless screenshot page.png

# Navigate and screenshot
headless navigate https://github.com
sleep 2
headless screenshot github.png
```

#### Resize Browser Window

```bash
# Resize to mobile size
headless resize 375 667

# Resize to desktop size
headless resize 1920 1080
```

#### Console Management

```bash
# Read console logs
headless read_console

# Clear console logs
headless clear_console
```

## Advanced Usage with AI Agent

### Prerequisites

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

### Using Existing Server

```bash
# Start the server (runs in background)
headless start

# Use AI to browse
headless ai "Go to example.com and tell me what the main heading says"

# Stop when done
headless stop
```

### Standalone Mode

The AI agent can run its own independent browser:

```bash
headless ai --standalone "Visit github.com/chromedp/chromedp and summarize what this project does"
```

### Complex AI Tasks

```bash
# Multi-step browsing task
headless ai "Navigate to example.com, take a screenshot, then tell me the page title"

# Information extraction
headless ai --standalone "Go to news.ycombinator.com and tell me the title of the top story"

# Web scraping
headless ai "Visit golang.org and extract the main features listed on the homepage"
```

## Web Interface

When the server is running, visit `http://localhost:11111` in your web browser to access the control panel.

The web interface provides:
- Forms for all commands
- Real-time feedback
- Easy testing of browser automation

## Integration with Other Tools

### Using with curl

```bash
# Navigate
curl -X POST http://localhost:11111/api/navigate \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'

# Evaluate JavaScript
curl -X POST http://localhost:11111/api/eval \
  -H "Content-Type: application/json" \
  -d '{"js":"document.title"}'

# Take screenshot
curl -X POST http://localhost:11111/api/screenshot \
  -H "Content-Type: application/json" \
  -d '{"path":"output.png"}'

# Read console
curl http://localhost:11111/api/read_console
```

### Scripting Example

```bash
#!/bin/bash

# Start server (runs in background automatically)
headless start -port 9999

# Wait for server to start
sleep 2

# Perform automation tasks
headless -port 9999 navigate https://example.com
headless -port 9999 eval "document.title"
headless -port 9999 screenshot example.png

# Cleanup
headless -port 9999 stop
```

## Tips and Best Practices

1. **Wait Between Commands**: Give the browser time to load pages between commands
2. **Check Console Logs**: Use `read_console` to debug JavaScript execution
3. **Screenshot for Verification**: Take screenshots to verify page state
4. **Use Resize for Responsive Testing**: Test different viewport sizes
5. **AI Agent Limitations**: The AI agent uses Claude's API and has token limits
6. **Standalone vs Server Mode**: Use standalone for one-off tasks, server mode for interactive sessions

## Troubleshooting

### Connection Refused

If you get "connection refused" errors:
- Verify the server is running
- Check you're using the correct port
- Ensure no firewall is blocking the connection

### Chrome Not Found

If Chrome/Chromium is not found:
- The chromedp library will try to download Chrome automatically
- You can install Chrome manually if needed

### AI Agent Errors

If the AI agent fails:
- Verify `ANTHROPIC_API_KEY` is set
- Check your API key has sufficient credits
- Ensure you have network connectivity to Anthropic's API
