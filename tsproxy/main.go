package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"tailscale.com/tsnet"
)

func main() {
	name := flag.String("name", "", "Tailscale node name")
	ports := flag.String("ports", "", "Comma-separated list of ports, ranges, or mixed (e.g., '8000,8001' or '8000-8005' or '8000,8010-8015')")
	allowSelfOnly := flag.Bool("allow-self-only", true, "Only allow requests from the same Tailscale user")
	flag.Parse()

	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" || *name == "" || *ports == "" {
		log.Fatal("Usage: TS_AUTHKEY=<key> tsproxy -name=<name> -ports=<ports>")
	}

	portList, err := parsePorts(*ports)
	if err != nil {
		log.Fatalf("Failed to parse ports: %v", err)
	}

	srv := &tsnet.Server{
		Hostname: *name,
		AuthKey:  authKey,
		Logf:     log.Printf,
	}

	defer srv.Close()

	var selfLoginName string
	if *allowSelfOnly {
		log.Printf("Self-only mode enabled - will determine identity from first request")
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	for _, port := range portList {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if err := servePort(ctx, srv, p, *allowSelfOnly, selfLoginName); err != nil {
				log.Printf("Error serving port %d: %v", p, err)
			}
		}(port)
	}

	wg.Wait()
}

func parsePorts(portStr string) ([]int, error) {
	var ports []int
	seen := make(map[int]bool)

	// Split by comma first to handle mixed formats: "8000,8010-8015,8020"
	parts := strings.Split(portStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range format: "8000-8005"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %s: %v", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %s: %v", part, err)
			}
			for i := start; i <= end; i++ {
				if !seen[i] {
					ports = append(ports, i)
					seen[i] = true
				}
			}
		} else {
			// Single port
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %v", err)
			}
			if !seen[port] {
				ports = append(ports, port)
				seen[port] = true
			}
		}
	}

	return ports, nil
}

func servePort(ctx context.Context, srv *tsnet.Server, port int, allowSelfOnly bool, selfLoginName string) error {
	ln, err := srv.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", port, err)
	}
	defer ln.Close()

	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	lc, err := srv.LocalClient()
	if err != nil {
		return fmt.Errorf("failed to get local client: %v", err)
	}

	var allowedIdentity string
	var identityLock sync.Mutex

	// Wrap the proxy handler with logging and access control
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := r.RemoteAddr

		// Try to get Tailscale user info
		whois, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err == nil && whois != nil && whois.UserProfile != nil {
			if whois.UserProfile.LoginName != "" {
				identity = whois.UserProfile.LoginName
			}
		}

		// Check if access is restricted to self only
		if allowSelfOnly {
			identityLock.Lock()
			if allowedIdentity == "" {
				// First request sets the allowed identity
				allowedIdentity = identity
				log.Printf("[Port %d] Restricting access to: %s", port, allowedIdentity)
			}
			identityLock.Unlock()

			if identity != allowedIdentity {
				log.Printf("[Port %d] DENIED %s %s %s (from %s, expected %s)", port, r.Method, r.URL.Path, r.Proto, identity, allowedIdentity)
				http.Error(w, "Forbidden: Access restricted to owner only", http.StatusForbidden)
				return
			}
		}

		log.Printf("[Port %d] %s %s %s (from %s)", port, r.Method, r.URL.Path, r.Proto, identity)
		proxy.ServeHTTP(w, r)
	})

	log.Printf("Serving port %d on Tailscale network, proxying to localhost:%d", port, port)

	server := &http.Server{
		Handler: handler,
	}

	return server.Serve(ln)
}
