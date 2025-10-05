# tsproxy

A minimal HTTP reverse proxy that exposes local ports on your Tailscale network.

## Features

- Expose multiple local ports on Tailscale with a single command
- Support for port ranges and comma-separated lists
- Identity-based access control (restrict to self by default)
- Request logging with Tailscale user identity

## Usage

```bash
# Single port
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000

# Multiple ports
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000,8001,8002

# Port range
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000-8005

# Mixed format
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000,8010-8015,8020

# Allow access from all Tailscale users (not just self)
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000 -allow-self-only=false
```

## Flags

- `-name` (required): Tailscale node name
- `-ports` (required): Comma-separated ports, ranges, or mixed (e.g., `8000,8010-8015`)
- `-allow-self-only` (default: `true`): Restrict access to the first user who connects

## Environment Variables

- `TS_AUTHKEY` (required): Tailscale auth key for authentication

## How It Works

For each port specified, tsproxy:
1. Creates a listener on the Tailscale network
2. Reverse proxies all HTTP requests to `localhost:<port>`
3. Logs each request with the Tailscale user's email/identity
4. Enforces access control if `-allow-self-only=true` (default)

## Prior Work

This implementation was inspired by [tsnsrv](https://github.com/boinkor-net/tsnsrv), which provides more features like funnel mode and path prefixing. `tsproxy` is a minimal implementation focused on simple port forwarding with identity-based access control.

## Building

```bash
go build -o tsproxy main.go
```
