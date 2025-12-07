#!/usr/bin/env python3
"""
Meta-server for ctr-agent: displays a dashboard of running container-agent containers.

Shows tiles for each container running /mnt/ctr-agent, with live tmux capture-pane
output. Tiles are sorted by last activity (most recently changed first).
"""

import argparse
import hashlib
import html
import json
import subprocess
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Dict, List, Optional
from dataclasses import dataclass, field


@dataclass
class ContainerState:
    """Tracks the state of a container for change detection."""
    container_id: str
    name: str
    pane_content: str = ""
    content_hash: str = ""
    last_change_time: float = field(default_factory=time.time)
    yatty_port: Optional[int] = None


# Global state tracking
container_states: Dict[str, ContainerState] = {}


def get_ctr_agent_containers() -> List[dict]:
    """Get all running containers that are running /mnt/ctr-agent."""
    try:
        # Get all running containers with their details
        result = subprocess.run(
            ["docker", "ps", "--format", "{{json .}}"],
            capture_output=True,
            text=True,
            timeout=10
        )
        if result.returncode != 0:
            return []

        containers = []
        for line in result.stdout.strip().split('\n'):
            if not line:
                continue
            try:
                container = json.loads(line)
                containers.append(container)
            except json.JSONDecodeError:
                continue

        # Filter to only containers running ctr-agent
        # Check if the container has /mnt/ctr-agent.py mounted
        ctr_agent_containers = []
        for container in containers:
            container_id = container.get('ID', '')
            # Check if this container is running ctr-agent by looking at the command
            # or checking if /mnt/ctr-agent.py exists
            try:
                check_result = subprocess.run(
                    ["docker", "exec", container_id, "test", "-f", "/mnt/ctr-agent.py"],
                    capture_output=True,
                    timeout=5
                )
                if check_result.returncode == 0:
                    ctr_agent_containers.append(container)
            except (subprocess.TimeoutExpired, Exception):
                continue

        return ctr_agent_containers
    except Exception as e:
        print(f"Error getting containers: {e}")
        return []


def get_yatty_port(container_id: str) -> Optional[int]:
    """Get the host port mapped to container port 8001 (yatty)."""
    try:
        result = subprocess.run(
            ["docker", "port", container_id, "8001"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0 and result.stdout.strip():
            # Output is like "0.0.0.0:32768" or "[::]:32768"
            port_mapping = result.stdout.strip().split('\n')[0]
            port = port_mapping.split(':')[-1]
            return int(port)
    except Exception:
        pass
    return None


def capture_tmux_pane(container_id: str) -> str:
    """Capture the current tmux pane content from a container."""
    try:
        result = subprocess.run(
            ["docker", "exec", container_id, "tmux", "capture-pane", "-p", "-t", "s:0"],
            capture_output=True,
            text=True,
            timeout=10
        )
        if result.returncode == 0:
            return result.stdout
    except Exception as e:
        return f"Error capturing pane: {e}"
    return ""


def update_container_states(containers: List[dict]) -> None:
    """Update the global container states with latest pane content."""
    global container_states

    current_ids = set()

    for container in containers:
        container_id = container.get('ID', '')
        name = container.get('Names', container_id)
        current_ids.add(container_id)

        # Capture pane content
        pane_content = capture_tmux_pane(container_id)
        content_hash = hashlib.md5(pane_content.encode()).hexdigest()

        # Get yatty port
        yatty_port = get_yatty_port(container_id)

        if container_id in container_states:
            state = container_states[container_id]
            state.name = name
            state.yatty_port = yatty_port
            # Check if content changed
            if state.content_hash != content_hash:
                state.pane_content = pane_content
                state.content_hash = content_hash
                state.last_change_time = time.time()
        else:
            # New container
            container_states[container_id] = ContainerState(
                container_id=container_id,
                name=name,
                pane_content=pane_content,
                content_hash=content_hash,
                last_change_time=time.time(),
                yatty_port=yatty_port
            )

    # Remove containers that no longer exist
    for container_id in list(container_states.keys()):
        if container_id not in current_ids:
            del container_states[container_id]


def format_time_ago(timestamp: float) -> str:
    """Format a timestamp as a human-readable 'time ago' string."""
    diff = time.time() - timestamp
    if diff < 60:
        return f"{int(diff)}s ago"
    elif diff < 3600:
        return f"{int(diff / 60)}m ago"
    elif diff < 86400:
        return f"{int(diff / 3600)}h ago"
    else:
        return f"{int(diff / 86400)}d ago"


def generate_html(magic_dns_suffix: Optional[str] = None) -> str:
    """Generate the HTML dashboard."""
    # Sort containers by last change time (most recent first)
    sorted_states = sorted(
        container_states.values(),
        key=lambda s: s.last_change_time,
        reverse=True
    )

    tiles_html = ""
    for state in sorted_states:
        # Build the yatty URL
        if magic_dns_suffix:
            # Use Tailscale hostname
            yatty_url = f"http://{state.name}.{magic_dns_suffix}:8001/"
        elif state.yatty_port:
            # Use localhost with mapped port
            yatty_url = f"http://localhost:{state.yatty_port}/"
        else:
            yatty_url = "#"

        # Escape HTML in pane content
        escaped_content = html.escape(state.pane_content)
        time_ago = format_time_ago(state.last_change_time)

        tiles_html += f'''
        <a href="{yatty_url}" class="tile" target="_blank">
            <div class="tile-header">
                <span class="tile-name">{html.escape(state.name)}</span>
                <span class="tile-time">{time_ago}</span>
            </div>
            <pre class="tile-content">{escaped_content}</pre>
        </a>
        '''

    if not tiles_html:
        tiles_html = '<div class="no-containers">No ctr-agent containers running</div>'

    return f'''<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>ctr-agent Dashboard</title>
    <meta http-equiv="refresh" content="20">
    <style>
        * {{
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }}

        body {{
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #1a1a2e;
            color: #eee;
            min-height: 100vh;
            padding: 10px;
        }}

        h1 {{
            text-align: center;
            margin-bottom: 15px;
            font-size: 1.5em;
            color: #7b68ee;
        }}

        .container {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 10px;
            width: 100%;
        }}

        .tile {{
            background: #16213e;
            border: 1px solid #0f3460;
            border-radius: 8px;
            padding: 10px;
            text-decoration: none;
            color: inherit;
            display: flex;
            flex-direction: column;
            min-height: 200px;
            max-height: 400px;
            overflow: hidden;
            transition: transform 0.2s, box-shadow 0.2s;
        }}

        .tile:hover {{
            transform: translateY(-2px);
            box-shadow: 0 4px 20px rgba(123, 104, 238, 0.3);
            border-color: #7b68ee;
        }}

        .tile-header {{
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
            padding-bottom: 8px;
            border-bottom: 1px solid #0f3460;
        }}

        .tile-name {{
            font-weight: bold;
            font-size: 1.1em;
            color: #7b68ee;
        }}

        .tile-time {{
            font-size: 0.8em;
            color: #888;
        }}

        .tile-content {{
            flex: 1;
            overflow: hidden;
            font-family: "SF Mono", "Monaco", "Inconsolata", "Fira Mono", "Droid Sans Mono", "Source Code Pro", monospace;
            font-size: 10px;
            line-height: 1.3;
            white-space: pre;
            background: #0a0a15;
            padding: 8px;
            border-radius: 4px;
            color: #aaa;
        }}

        .no-containers {{
            text-align: center;
            padding: 50px;
            color: #666;
            font-size: 1.2em;
        }}

        .refresh-indicator {{
            position: fixed;
            bottom: 10px;
            right: 10px;
            font-size: 0.8em;
            color: #666;
        }}
    </style>
</head>
<body>
    <h1>ctr-agent Dashboard</h1>
    <div class="container">
        {tiles_html}
    </div>
    <div class="refresh-indicator">Auto-refresh: 20s</div>
</body>
</html>'''


class DashboardHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the dashboard."""

    magic_dns_suffix = None

    def log_message(self, format, *args):
        # Suppress default logging
        pass

    def do_GET(self):
        if self.path == '/' or self.path == '/index.html':
            # Refresh container states
            containers = get_ctr_agent_containers()
            update_container_states(containers)

            # Generate and send HTML
            html_content = generate_html(self.magic_dns_suffix)

            self.send_response(200)
            self.send_header('Content-type', 'text/html; charset=utf-8')
            self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
            self.end_headers()
            self.wfile.write(html_content.encode('utf-8'))
        elif self.path == '/api/containers':
            # JSON API endpoint
            containers = get_ctr_agent_containers()
            update_container_states(containers)

            data = [
                {
                    'id': s.container_id,
                    'name': s.name,
                    'lastChange': s.last_change_time,
                    'yattyPort': s.yatty_port,
                    'content': s.pane_content[:500]  # Truncate for API
                }
                for s in sorted(
                    container_states.values(),
                    key=lambda s: s.last_change_time,
                    reverse=True
                )
            ]

            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Cache-Control', 'no-cache')
            self.end_headers()
            self.wfile.write(json.dumps(data).encode('utf-8'))
        else:
            self.send_response(404)
            self.end_headers()


def get_magic_dns_suffix() -> Optional[str]:
    """Get the Tailscale MagicDNS suffix."""
    import platform

    tailscale_paths = ["tailscale"]
    if platform.system() == "Darwin":
        tailscale_paths.append("/Applications/Tailscale.app/Contents/MacOS/Tailscale")

    for ts_path in tailscale_paths:
        try:
            result = subprocess.run(
                [ts_path, "status", "-json"],
                capture_output=True,
                text=True,
                timeout=5
            )
            if result.returncode == 0:
                ts_status = json.loads(result.stdout)
                suffix = ts_status.get("MagicDNSSuffix", "").rstrip(".")
                if suffix:
                    return suffix
        except Exception:
            continue
    return None


def main():
    parser = argparse.ArgumentParser(description="ctr-agent meta-server dashboard")
    parser.add_argument("--port", type=int, default=2000, help="Port to serve on (default: 2000)")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to (default: 0.0.0.0)")
    args = parser.parse_args()

    # Get MagicDNS suffix for Tailscale URLs
    magic_dns_suffix = get_magic_dns_suffix()
    if magic_dns_suffix:
        print(f"Using Tailscale MagicDNS suffix: {magic_dns_suffix}")
    else:
        print("Tailscale not detected, will use localhost port mappings")

    # Set the suffix on the handler class
    DashboardHandler.magic_dns_suffix = magic_dns_suffix

    server = HTTPServer((args.host, args.port), DashboardHandler)
    print(f"Starting meta-server on http://{args.host}:{args.port}/")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == "__main__":
    main()
