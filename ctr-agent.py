#!/usr/bin/env python3
import argparse
import getpass
import json
import os
import random
import secrets
import subprocess
import sys
from pathlib import Path


ADJECTIVES = [
    "happy", "clever", "brave", "calm", "eager", "gentle", "jolly", "kind",
    "lively", "proud", "swift", "wise", "bright", "cool", "fair", "keen",
    "noble", "quick", "sharp", "warm", "bold", "daring", "fuzzy", "silly"
]

ANIMALS = [
    "ant", "bear", "cat", "dog", "eagle", "fox", "goat", "hawk", "ibex",
    "jay", "koala", "lion", "mouse", "newt", "owl", "panda", "quail", "rabbit",
    "seal", "tiger", "urchin", "viper", "wolf", "yak", "zebra", "otter", "penguin"
]


def load_config():
    """Load configuration from JSON file."""
    # Allow environment variable to override config path
    config_path_str = os.environ.get("CTR_AGENT_CONFIG")
    if config_path_str:
        config_path = Path(config_path_str).expanduser()
    else:
        config_path = Path.home() / ".config" / "ctr-agent" / "config.json"

    # Always ensure config directories exist
    ctr_agent_dir = Path.home() / ".config" / "ctr-agent"
    (ctr_agent_dir / "codex").mkdir(parents=True, exist_ok=True)
    (ctr_agent_dir / "claude").mkdir(parents=True, exist_ok=True)

    if not config_path.exists():
        config = get_default_config()
        # Write out default config
        config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(config_path, "w") as f:
            json.dump(config, f, indent=2)

        print(f"Created default config at: {config_path}")
        print(f"Edit this file to customize your configuration")
        return config

    with open(config_path, "r") as f:
        return json.load(f)


def get_default_config():
    """Return default configuration."""
    return {
        "image": "container-agent:dev",
        "docker_options": ["-p", "0:9000"],
        "env_vars": {
            "OPENAI_API_KEY": None,
            "ANTHROPIC_API_KEY": None,
            "TS_AUTHKEY": None,
        },
        "mounts": [
            {"host": "/var/run/docker.sock", "container": "/var/run/docker.sock"},
            {"host": "{HOME}/.config/ctr-agent/codex", "container": "/home/agent/.codex"},
            {"host": "{HOME}/.config/ctr-agent/claude", "container": "/home/agent/.claude"},
        ],
        "agents": {
            "codex": {
                "command": "codex -s danger-full-access",
            },
            "claude": {
                "command": "claude --dangerously-skip-permissions",
            },
            "bash": {
                "command": "bash",
            },
        },
        "additional_panes": [
            {
                "name": "tsproxy",
                "command": "if [ -n \"$TS_AUTHKEY\" ]; then /go/bin/tsproxy -name {slug} -ports 8000-9999,11111; else sleep infinity; fi",
                # Alternative: use tsnsrv instead
                # "command": "if [ -n \"$TS_AUTHKEY\" ]; then /go/bin/tsnsrv -name {slug} -listenAddr :9000 -plaintext=true http://0.0.0.0:9000/; else sleep infinity; fi",
            },
            {
                "name": "gotty",
                "command": "/go/bin/gotty -w -p 8001 --title-format 'Terminal - {slug}' --ws-query-args 'preferences='$(echo '{{\"theme\":{{\"background\":\"#ffffff\",\"foreground\":\"#000000\",\"cursor\":\"#000000\",\"cursorAccent\":\"#ffffff\",\"selection\":\"rgba(0,0,0,0.3)\"}}}}' | base64 -w0) tmux attach",
            },
            {
                "name": "headless",
                "command": "/go/bin/headless start --foreground",
            },
        ],
    }


def inside_mode(args, config):
    """Run inside the container - setup worktree and start agent."""

    # Fix ownership
    current_user = getpass.getuser()
    subprocess.run(["sudo", "chown", "-R", current_user, os.getcwd()], check=True)

    # Create unique work directory with random suffix
    random_suffix = secrets.token_hex(8)
    unique_work_dir = f"/home/agent/work-{random_suffix}"
    os.mkdir(unique_work_dir)

    # Add worktree to the unique directory
    # subprocess.run("bash")
    subprocess.run(
        ["git", "worktree", "add", unique_work_dir, "-b", args.slug, args.committish],
        # I don't know why this is necessary:
        cwd=args.git_dir + "/.git",
        check=True
    )

    # Rename the unique directory to /home/agent/work
    os.rename(unique_work_dir, "/home/agent/work")
    Path(unique_work_dir).symlink_to("/home/agent/work")

    # Change to work directory and then to prefix directory
    os.chdir("/home/agent/work")
    os.chdir(args.prefix)

    # Create symlink for .claude.json to work around directory-only mount limitation
    claude_json_symlink = Path("/home/agent/.claude.json")
    if not claude_json_symlink.exists():
        claude_json_symlink.symlink_to("/home/agent/.claude/claude.json")

    # Get agent configuration
    agent_config = config["agents"].get(args.agent)
    if not agent_config:
        raise ValueError(f"Unknown agent: {args.agent}")

    # Get agent command
    agent_cmd = agent_config["command"]

    # Get additional panes
    additional_panes = config.get("additional_panes", [])

    # Create tmux session with additional panes if configured
    session_name = "s"

    # TODO: Set tmux status bar to yellow?
    #subprocess.run(["tmux", "set-option", "-g", "status-style", "bg=yellow,fg=black"], check=False)

    if additional_panes:
        # Create detached tmux session
        subprocess.run(["tmux", "new-session", "-d", "-s", session_name], check=True)

        # Create additional panes
        for pane in additional_panes:
            pane_name = pane.get("name", "pane")
            pane_cmd = pane["command"].format(slug=args.slug)
            subprocess.run(
                ["tmux", "new-window", "-t", session_name, "-n", pane_name, pane_cmd],
                check=True
            )
            print(f"Started {pane_name} in tmux pane")

        # Run agent in main window and select it
        subprocess.run(
            ["tmux", "send-keys", "-t", f"{session_name}:0", agent_cmd, "Enter"],
            check=True
        )
        # Switch to first window (main agent window)
        subprocess.run(
            ["tmux", "select-window", "-t", f"{session_name}:0"],
            check=True
        )
        # Attach to session
        cmd = ["tmux", "attach-session", "-t", session_name]
    else:
        # No additional panes, just create new session with agent
        cmd = ["tmux", "new-session", "-s", session_name, agent_cmd]

    subprocess.run(cmd, check=False)

    # After exit, print slug and clean up worktree if branch hasn't moved
    print(f"\nExited container: {args.slug}")

    # Check if workspace is dirty and commit if so
    git_status = subprocess.run(
        ["git", "status", "--porcelain"],
        capture_output=True, text=True, check=False
    )

    if git_status.returncode == 0 and git_status.stdout.strip():
        print(f"Workspace is dirty, creating commit...")
        # Add all changes
        subprocess.run(["git", "add", "-A"], check=False)
        # Create commit
        commit_msg = f"Auto-commit by ctragent on exit\n\nAgent: {args.agent}\nBranch: {args.slug}"
        subprocess.run(
            ["git", "commit", "-m", commit_msg],
            check=False
        )
        print(f"Created commit for dirty workspace")

    # Check if branch still points to original commit
    current_commit = subprocess.run(
        ["git", "rev-parse", args.slug],
        capture_output=True, text=True, check=False,
        cwd=args.git_dir
    )

    if current_commit.returncode == 0 and current_commit.stdout.strip() == args.committish:
        print(f"Branch {args.slug} unchanged, cleaning up...")
        subprocess.run(
            ["git", "worktree", "remove", "--force", unique_work_dir],
            cwd=args.git_dir,
            check=False
        )
        subprocess.run(
            ["git", "branch", "-D", args.slug],
            cwd=args.git_dir,
            check=False
        )
    else:
        print(f"Branch {args.slug} has moved, keeping worktree and branch")


def generate_random_slug():
    """Generate a random two-word hyphenated slug."""
    adjective = random.choice(ADJECTIVES)
    animal = random.choice(ANIMALS)
    return f"{adjective}-{animal}"


def outside_mode(args, config):
    """Run outside the container - setup and launch docker."""
    # Generate slug
    args.slug = generate_random_slug()
    print(f"Generated slug: {args.slug}")

    # Handle --open flag to open browser to gotty
    open_browser = getattr(args, 'open', True)  # default True

    # Get git information
    git_dir = subprocess.run(
        ["git", "rev-parse", "--path-format=absolute", "--git-common-dir"],
        capture_output=True, text=True, check=True
    ).stdout.strip()

    # I don't really know why we need this, but we seem to, otherwise worktrees
    # have a dirty "status" when you create them
    if git_dir.endswith(".git"):
        git_dir = os.path.dirname(git_dir)

    committish = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        capture_output=True, text=True, check=True
    ).stdout.strip()

    prefix = subprocess.run(
        ["git", "rev-parse", "--show-prefix"],
        capture_output=True, text=True, check=True
    ).stdout.strip()

    if not prefix:
        prefix = "."

    workdir = f"/home/agent"

    print(f"Git dir: {git_dir}")
    print(f"Workdir:   {workdir}")
    print(f"Committish: {committish}")

    # Get script path
    script_path = Path(__file__).resolve()

    # Get image from config
    image_tag = config.get("image", "container-agent:dev")

    # Build docker command
    # If --open is true, run detached; otherwise run interactive
    if open_browser:
        docker_cmd = [
            "docker", "run", "--rm", "-d",
            "--hostname", args.slug,
            "--name", args.slug,
        ]
    else:
        docker_cmd = [
            "docker", "run", "--rm", "-it",
            "--hostname", args.slug,
            "--name", args.slug,
        ]

    # Add docker options from config
    docker_options = config.get("docker_options", [])
    docker_cmd.extend(docker_options)

    # Add environment variables from config
    env_vars = config.get("env_vars", {})
    for key, value in env_vars.items():
        if value is None:
            # Pass through from host environment
            docker_cmd.extend(["-e", f"{key}={os.environ.get(key, '')}"])
        else:
            # Use configured value
            docker_cmd.extend(["-e", f"{key}={value}"])

    # Always add COMMITTISH
    docker_cmd.extend(["-e", f"COMMITTISH={committish}"])

    # Add mounts from config
    mounts = config.get("mounts", [])
    for mount in mounts:
        # Expand variables in mount paths
        host_path = mount["host"].format(
            HOME=os.environ.get("HOME", ""),
            git_dir=git_dir,
        )
        container_path = mount["container"]
        docker_cmd.extend(["-v", f"{host_path}:{container_path}"])

    # Add git_dir mount (dynamic)
    docker_cmd.extend(["-v", f"{git_dir}:{git_dir}"])

    # Add script mount (dynamic)
    docker_cmd.extend(["-v", f"{script_path}:/mnt/ctr-agent.py"])

    # Add working directory and image
    docker_cmd.extend(["-w", workdir, image_tag])

    # Add command to run inside container
    docker_cmd.extend([
        "python3", "/mnt/ctr-agent.py", "inside",
        "--slug", args.slug,
        "--git-dir", git_dir,
        "--committish", committish,
        "--prefix", prefix,
        "--agent", args.agent,
    ])

    # Open browser if --open is True
    redirect_server = None
    if open_browser:
        import socket
        import threading
        from http.server import HTTPServer, BaseHTTPRequestHandler

        # Create a redirect handler that waits for hostname to resolve
        class RedirectHandler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                pass  # Suppress logging

            def do_GET(self):
                import socket
                import time

                target_url = f"http://{args.slug}:8001/"
                timeout = 20
                start_time = time.time()

                # Try to resolve the hostname with timeout
                resolved = False
                while time.time() - start_time < timeout:
                    try:
                        socket.getaddrinfo(args.slug, 8001, socket.AF_UNSPEC, socket.SOCK_STREAM)
                        resolved = True
                        break
                    except socket.gaierror:
                        time.sleep(0.5)

                if resolved:
                    self.send_response(302)
                    self.send_header('Location', target_url)
                    self.end_headers()
                else:
                    self.send_response(504)
                    self.send_header('Content-type', 'text/html')
                    self.end_headers()
                    self.wfile.write(f"<html><body><h1>Timeout</h1><p>Could not resolve {args.slug} after {timeout} seconds</p></body></html>".encode())

        # Start server on port 0 (random available port)
        server = HTTPServer(('localhost', 0), RedirectHandler)
        port = server.server_port

        def run_server():
            server.handle_request()  # Handle one request then stop

        server_thread = threading.Thread(target=run_server, daemon=True)
        server_thread.start()

        # Open browser to the local redirect server
        redirect_url = f"http://localhost:{port}/"
        print(f"Opening browser to: {redirect_url}")
        print(f"Will redirect to: http://{args.slug}:8001/ once hostname resolves")

        # Detect platform and use appropriate command
        import platform
        try:
            if platform.system() == "Darwin":  # macOS
                subprocess.run(["open", redirect_url], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            elif platform.system() == "Windows":
                subprocess.run(["start", redirect_url], shell=True, check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            else:  # Linux and others
                subprocess.run(["xdg-open", redirect_url], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except Exception as e:
            print(f"Failed to open browser: {e}")

    if open_browser:
        # Run container in detached mode
        result = subprocess.run(docker_cmd, capture_output=True, text=True, check=False)
        container_id = result.stdout.strip()

        if result.returncode == 0:
            print(f"\nContainer started: {args.slug}")
            print(f"Container ID: {container_id}")
            print(f"\nGotty URL: http://{args.slug}:8001/")
            print(f"\nTo attach a terminal, run:")
            print(f"  docker exec -it {args.slug} tmux attach")
            print(f"\nWaiting for container to exit (press Ctrl+C to detach)...")

            # Wait for the container to exit
            try:
                subprocess.run(["docker", "wait", args.slug], check=False)
            except KeyboardInterrupt:
                print(f"\n\nDetached from container. Container is still running.")
                print(f"To reattach: docker exec -it {args.slug} tmux attach")
                print(f"To stop: docker stop {args.slug}")
        else:
            print(f"Failed to start container: {result.stderr}")
    else:
        # Run container in interactive mode (original behavior)
        subprocess.run(docker_cmd, check=False)

    print(f"\nExited container: {args.slug}")


def main():
    # Load configuration
    config = load_config()

    # Check if running in inside mode
    if len(sys.argv) > 1 and sys.argv[1] == "inside":
        # Inside mode parser
        parser = argparse.ArgumentParser(description="Run inside container")
        parser.add_argument("mode", help="Must be 'inside'")
        parser.add_argument("--git-dir", required=True, help="Git directory path")
        parser.add_argument("--committish", required=True, help="Git commit hash")
        parser.add_argument("--prefix", required=True, help="Working directory prefix")
        parser.add_argument("--agent", required=True, help="Agent to run")
        parser.add_argument("--slug", help="slug")
        args = parser.parse_args()
        inside_mode(args, config)
    else:
        # Outside mode parser (default, user-facing)
        parser = argparse.ArgumentParser(description="Run agent in container")
        parser.add_argument("agent", help="Agent to run")
        parser.add_argument("--open", type=lambda x: x.lower() != 'false', default=True,
                          help="Open browser to gotty session (default: true, disable with --open=false)")
        args = parser.parse_args()
        outside_mode(args, config)


if __name__ == "__main__":
    main()
