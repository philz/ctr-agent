package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type Server struct {
	port          int
	ctx           context.Context
	cancel        context.CancelFunc
	consoleLogs   []string
	consoleMutex  sync.RWMutex
	browserWidth  int64
	browserHeight int64
}

func New(port int) *Server {
	return &Server{
		port:          port,
		consoleLogs:   make([]string, 0),
		browserWidth:  1920,
		browserHeight: 1080,
	}
}

func (s *Server) Start() error {
	// Create Chrome context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(int(s.browserWidth), int(s.browserHeight)),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	s.ctx, s.cancel = chromedp.NewContext(allocCtx)

	// Set up console log listener
	chromedp.ListenTarget(s.ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				s.consoleMutex.Lock()
				s.consoleLogs = append(s.consoleLogs, string(arg.Value))
				s.consoleMutex.Unlock()
			}
		}
	})

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWeb)
	mux.HandleFunc("/api/navigate", s.handleNavigate)
	mux.HandleFunc("/api/eval", s.handleEval)
	mux.HandleFunc("/api/screenshot", s.handleScreenshot)
	mux.HandleFunc("/api/resize", s.handleResize)
	mux.HandleFunc("/api/read_console", s.handleReadConsole)
	mux.HandleFunc("/api/clear_console", s.handleClearConsole)
	mux.HandleFunc("/api/stop", s.handleStop)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	return server.ListenAndServe()
}

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Headless Browser Control</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
        }
        h1 { color: #333; }
        .command {
            margin: 20px 0;
            padding: 15px;
            border: 1px solid #ddd;
            border-radius: 5px;
        }
        .command h3 {
            margin-top: 0;
            color: #0066cc;
        }
        input[type="text"], textarea {
            width: 100%;
            padding: 8px;
            margin: 5px 0;
            box-sizing: border-box;
        }
        input[type="number"] {
            width: 100px;
            padding: 8px;
            margin: 5px 5px 5px 0;
        }
        button {
            background-color: #0066cc;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        button:hover {
            background-color: #0052a3;
        }
        #output {
            margin-top: 20px;
            padding: 15px;
            background-color: #f5f5f5;
            border-radius: 5px;
            min-height: 100px;
        }
        .success { color: green; }
        .error { color: red; }
    </style>
</head>
<body>
    <h1>Headless Browser Control</h1>
    <p>Control your headless Chrome browser from this interface.</p>

    <div class="command">
        <h3>Navigate</h3>
        <input type="text" id="navUrl" placeholder="https://example.com">
        <button onclick="navigate()">Navigate</button>
    </div>

    <div class="command">
        <h3>Evaluate JavaScript</h3>
        <textarea id="evalJs" rows="3" placeholder="document.title"></textarea>
        <button onclick="evalJs()">Evaluate</button>
    </div>

    <div class="command">
        <h3>Screenshot</h3>
        <input type="text" id="screenshotPath" placeholder="/path/to/screenshot.png">
        <button onclick="screenshot()">Take Screenshot</button>
    </div>

    <div class="command">
        <h3>Resize Browser</h3>
        <input type="number" id="width" placeholder="Width" value="1920">
        <input type="number" id="height" placeholder="Height" value="1080">
        <button onclick="resize()">Resize</button>
    </div>

    <div class="command">
        <h3>Console</h3>
        <button onclick="readConsole()">Read Console</button>
        <button onclick="clearConsole()">Clear Console</button>
    </div>

    <div class="command">
        <h3>Server Control</h3>
        <button onclick="stop()">Stop Server</button>
    </div>

    <div id="output"></div>

    <script>
        function showOutput(message, isError = false) {
            const output = document.getElementById('output');
            output.innerHTML = '<div class="' + (isError ? 'error' : 'success') + '">' +
                message + '</div>';
        }

        async function navigate() {
            const url = document.getElementById('navUrl').value;
            try {
                const response = await fetch('/api/navigate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({url: url})
                });
                const data = await response.json();
                showOutput(data.message || 'Navigated successfully', !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function evalJs() {
            const js = document.getElementById('evalJs').value;
            try {
                const response = await fetch('/api/eval', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({js: js})
                });
                const data = await response.json();
                showOutput('Result: ' + JSON.stringify(data.result, null, 2), !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function screenshot() {
            const path = document.getElementById('screenshotPath').value;
            try {
                const response = await fetch('/api/screenshot', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({path: path})
                });
                const data = await response.json();
                showOutput(data.message || 'Screenshot saved', !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function resize() {
            const width = parseInt(document.getElementById('width').value);
            const height = parseInt(document.getElementById('height').value);
            try {
                const response = await fetch('/api/resize', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({width: width, height: height})
                });
                const data = await response.json();
                showOutput(data.message || 'Resized successfully', !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function readConsole() {
            try {
                const response = await fetch('/api/read_console');
                const data = await response.json();
                showOutput('Console logs:\n' + JSON.stringify(data.logs, null, 2), !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function clearConsole() {
            try {
                const response = await fetch('/api/clear_console', {method: 'POST'});
                const data = await response.json();
                showOutput(data.message || 'Console cleared', !response.ok);
            } catch (e) {
                showOutput('Error: ' + e.message, true);
            }
        }

        async function stop() {
            if (confirm('Are you sure you want to stop the server?')) {
                try {
                    await fetch('/api/stop', {method: 'POST'});
                    showOutput('Server stopping...');
                } catch (e) {
                    showOutput('Server stopped', false);
                }
            }
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

type navigateRequest struct {
	URL string `json:"url"`
}

func (s *Server) handleNavigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req navigateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := chromedp.Run(s.ctx, chromedp.Navigate(req.URL)); err != nil {
		s.jsonError(w, fmt.Sprintf("Navigation failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"message": "Navigated successfully"})
}

type evalRequest struct {
	JS string `json:"js"`
}

func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req evalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var result interface{}
	if err := chromedp.Run(s.ctx, chromedp.Evaluate(req.JS, &result)); err != nil {
		s.jsonError(w, fmt.Sprintf("Evaluation failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{"result": result})
}

type screenshotRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req screenshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		s.jsonError(w, fmt.Sprintf("Screenshot failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(req.Path, buf, 0644); err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to save screenshot: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"message": fmt.Sprintf("Screenshot saved to %s", req.Path)})
}

type resizeRequest struct {
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

func (s *Server) handleResize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req resizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.browserWidth = req.Width
	s.browserHeight = req.Height

	if err := chromedp.Run(s.ctx, chromedp.EmulateViewport(req.Width, req.Height)); err != nil {
		s.jsonError(w, fmt.Sprintf("Resize failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"message": fmt.Sprintf("Resized to %dx%d", req.Width, req.Height)})
}

func (s *Server) handleReadConsole(w http.ResponseWriter, r *http.Request) {
	s.consoleMutex.RLock()
	logs := make([]string, len(s.consoleLogs))
	copy(logs, s.consoleLogs)
	s.consoleMutex.RUnlock()

	s.jsonResponse(w, map[string]interface{}{"logs": logs})
}

func (s *Server) handleClearConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.consoleMutex.Lock()
	s.consoleLogs = make([]string, 0)
	s.consoleMutex.Unlock()

	s.jsonResponse(w, map[string]string{"message": "Console cleared"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.jsonResponse(w, map[string]string{"message": "Server stopping"})

	go func() {
		time.Sleep(100 * time.Millisecond)
		if s.cancel != nil {
			s.cancel()
		}
		os.Exit(0)
	}()
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// GetContext returns the browser context (for use by AI agent)
func (s *Server) GetContext() context.Context {
	return s.ctx
}

// ReadConsoleLogs returns current console logs
func (s *Server) ReadConsoleLogs() []string {
	s.consoleMutex.RLock()
	defer s.consoleMutex.RUnlock()
	logs := make([]string, len(s.consoleLogs))
	copy(logs, s.consoleLogs)
	return logs
}

// ClearConsoleLogs clears the console logs
func (s *Server) ClearConsoleLogs() {
	s.consoleMutex.Lock()
	defer s.consoleMutex.Unlock()
	s.consoleLogs = make([]string, 0)
}

// Navigate navigates to a URL
func (s *Server) Navigate(url string) error {
	return chromedp.Run(s.ctx, chromedp.Navigate(url))
}

// Evaluate evaluates JavaScript
func (s *Server) Evaluate(js string) (interface{}, error) {
	var result interface{}
	err := chromedp.Run(s.ctx, chromedp.Evaluate(js, &result))
	return result, err
}

// Screenshot takes a screenshot
func (s *Server) Screenshot(path string) error {
	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}
