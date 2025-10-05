package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/philz/headless/internal/ai"
	"github.com/philz/headless/internal/browser"
	"github.com/philz/headless/internal/server"
)

const usage = `headless - control a headless Chrome browser from the CLI

Usage:
  headless start [-port PORT] [--foreground]
  headless [-port PORT] <command> [args...]

Server Command:
  start                 Start headless browser server (runs in background by default)
    -port PORT          Port to run on (default: 11111)
    --foreground        Run in foreground instead of background

Browser Commands:
  navigate <url>        Navigate to a URL
  eval <js>             Evaluate JavaScript and return result
  screenshot <path>     Take a screenshot and save to path
  resize <width> <h>    Resize browser window
  read_console          Read console logs
  clear_console         Clear console logs
  stop                  Stop the server

AI Command:
  ai [--standalone] <prompt>
                        Run AI agent to browse and answer prompt
    --standalone        Use independent browser (default: use running server)`

func startBackground(port int) error {
	// Get the path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare command arguments
	args := []string{"start", "-port", strconv.Itoa(port), "--foreground"}

	// Create the command
	cmd := exec.Command(executable, args...)

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Redirect output to /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer devNull.Close()

	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = nil

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background process: %w", err)
	}

	// Write PID file
	pidFile := fmt.Sprintf("/tmp/headless-%d.pid", port)
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Release the process so it can continue independently
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release process: %w", err)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(0)
	}

	port := 11111
	args := os.Args[1:]

	// Parse port flag
	if len(args) >= 2 && args[0] == "-port" {
		p, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid port: %v\n", err)
			os.Exit(1)
		}
		port = p
		args = args[2:]
	}

	if len(args) == 0 {
		fmt.Println(usage)
		os.Exit(0)
	}

	command := args[0]

	switch command {
	case "start":
		foreground := false
		startArgs := args[1:]

		// Parse start-specific flags
		for i := 0; i < len(startArgs); i++ {
			if startArgs[i] == "--foreground" {
				foreground = true
			} else if startArgs[i] == "-port" && i+1 < len(startArgs) {
				p, err := strconv.Atoi(startArgs[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid port: %v\n", err)
					os.Exit(1)
				}
				port = p
				i++ // Skip next arg since it's the port value
			}
		}

		if foreground {
			fmt.Printf("Starting headless server on port %d\n", port)
			srv := server.New(port)
			if err := srv.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Run in background
			if err := startBackground(port); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Started headless server in background on port %d\n", port)
			fmt.Printf("PID file: /tmp/headless-%d.pid\n", port)
		}

	case "navigate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: headless navigate <url>")
			os.Exit(1)
		}
		if err := browser.SendCommand(port, "navigate", map[string]interface{}{"url": args[1]}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Navigated successfully")

	case "eval":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: headless eval <js>")
			os.Exit(1)
		}
		result, err := browser.SendCommandWithResponse(port, "eval", map[string]interface{}{"js": args[1]})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)

	case "screenshot":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: headless screenshot <output path>")
			os.Exit(1)
		}
		if err := browser.SendCommand(port, "screenshot", map[string]interface{}{"path": args[1]}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Screenshot saved to %s\n", args[1])

	case "resize":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: headless resize <width> <height>")
			os.Exit(1)
		}
		width, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid width: %v\n", err)
			os.Exit(1)
		}
		height, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid height: %v\n", err)
			os.Exit(1)
		}
		if err := browser.SendCommand(port, "resize", map[string]interface{}{"width": width, "height": height}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Resized to %dx%d\n", width, height)

	case "read_console":
		result, err := browser.SendCommandWithResponse(port, "read_console", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)

	case "clear_console":
		if err := browser.SendCommand(port, "clear_console", nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Console cleared")

	case "stop":
		if err := browser.SendCommand(port, "stop", nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Server stopped")

	case "ai":
		standalone := false
		prompt := ""

		for i := 1; i < len(args); i++ {
			if args[i] == "--standalone=true" || args[i] == "--standalone" {
				standalone = true
			} else if args[i] == "--standalone=false" {
				standalone = false
			} else {
				// Rest is prompt
				prompt = args[i]
				if i+1 < len(args) {
					for j := i + 1; j < len(args); j++ {
						prompt += " " + args[j]
					}
				}
				break
			}
		}

		if prompt == "" {
			fmt.Fprintln(os.Stderr, "Usage: headless ai [--standalone=false] <prompt>")
			os.Exit(1)
		}

		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY environment variable not set")
			os.Exit(1)
		}

		result, err := ai.RunAgenticLoop(apiKey, prompt, port, standalone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Println(usage)
		os.Exit(1)
	}
}
