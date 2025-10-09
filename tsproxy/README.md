# tsproxy

A minimal HTTP reverse proxy that exposes local ports on your Tailscale network.

See tsnsrv for a more featured thing.

# Warning

This was pretty much entirely vibe-coded.

## Usage

```bash
TS_AUTHKEY=tskey-... go run main.go -name=my-proxy -ports=8000,8010-8015,8020
```

## Building

```bash
go build -o tsproxy main.go
```
