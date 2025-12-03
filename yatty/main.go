package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed index.html
var indexHTML []byte

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message types for websocket communication
const (
	msgInput  = '0' // Input from browser to PTY
	msgOutput = '1' // Output from PTY to browser
	msgResize = '2' // Resize terminal
)

// ResizeMessage represents a terminal resize request
type ResizeMessage struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: yatty [--port PORT] COMMAND [ARGS...]")
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, command, cmdArgs)
	})
	http.HandleFunc("/static/", handleStatic)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting yatty on http://localhost%s", addr)
	log.Printf("Running command: %s %v", command, cmdArgs)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down...")
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	// Strip /static/ prefix and serve from embedded filesystem
	path := r.URL.Path[1:] // Remove leading /
	data, err := staticFiles.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on extension
	switch {
	case len(path) > 3 && path[len(path)-3:] == ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case len(path) > 4 && path[len(path)-4:] == ".css":
		w.Header().Set("Content-Type", "text/css")
	}

	w.Write(data)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, command string, args []string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Start the command with a PTY
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("PTY start error: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error starting command: "+err.Error()))
		return
	}
	defer ptmx.Close()

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Read from PTY and send to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
				n, err := ptmx.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Printf("PTY read error: %v", err)
					}
					return
				}
				if n > 0 {
					// Prepend message type
					msg := append([]byte{msgOutput}, buf[:n]...)
					if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
						log.Printf("WebSocket write error: %v", err)
						return
					}
				}
			}
		}
	}()

	// Read from WebSocket and write to PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}

			if len(message) == 0 {
				continue
			}

			msgType := message[0]
			payload := message[1:]

			switch msgType {
			case msgInput:
				// Write input to PTY
				if _, err := ptmx.Write(payload); err != nil {
					log.Printf("PTY write error: %v", err)
					return
				}
			case msgResize:
				// Resize PTY
				var resize ResizeMessage
				if err := json.Unmarshal(payload, &resize); err != nil {
					log.Printf("Resize parse error: %v", err)
					continue
				}
				if err := pty.Setsize(ptmx, &pty.Winsize{
					Rows: resize.Rows,
					Cols: resize.Cols,
				}); err != nil {
					log.Printf("PTY resize error: %v", err)
				}
			}
		}
	}()

	// Wait for command to finish
	cmd.Wait()
	wg.Wait()
}
