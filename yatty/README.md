# yatty

Yet Another TTY - A simple web-based terminal using xterm.js and Go.

## Usage

```
yatty [--port PORT] COMMAND [ARGS...]
```

### Examples

```bash
# Run bash on port 8080 (default)
yatty /bin/bash

# Run bash on a specific port
yatty --port 8001 /bin/bash

# Run tmux attach
yatty --port 8001 tmux attach
```

## Features

- Single binary with all dependencies embedded
- Light theme by default (dark text on white background)
- WebSocket-based communication for low latency
- Automatic terminal resizing
- Uses xterm.js for terminal emulation

## Building

```bash
cd yatty
go build -o yatty .
```
