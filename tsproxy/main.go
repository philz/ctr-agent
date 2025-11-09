package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tailscale.com/tsnet"
)

func main() {
	name := flag.String("name", "", "Tailscale node name")
	ports := flag.String("ports", "", "Comma-separated list of ports, ranges, or mixed (e.g., '8000,8001' or '8000-8005' or '8000,8010-8015')")
	allowSelfOnly := flag.Bool("allow-self-only", true, "Only allow requests from the same Tailscale user")
	magicDNSSuffix := flag.String("magic-dns-suffix", "", "Tailscale MagicDNS suffix for health checking (e.g., 'example.ts.net')")
	checkInterval := flag.Duration("check-interval", 30*time.Second, "Interval for DNS health checks")
	flag.Parse()

	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" || *name == "" || *ports == "" {
		log.Fatal("Usage: TS_AUTHKEY=<key> tsproxy -name=<name> -ports=<ports>")
	}

	portList, err := parsePorts(*ports)
	if err != nil {
		log.Fatalf("Failed to parse ports: %v", err)
	}

	// Only enable verbose backend logs if TSPROXY_DEBUG is set
	var logf func(string, ...any)
	if os.Getenv("TSPROXY_DEBUG") != "" {
		logf = log.Printf
	}

	for {
		srv := &tsnet.Server{
			Hostname: *name,
			AuthKey:  authKey,
			Logf:     logf,
		}

		var selfLoginName string
		if *allowSelfOnly {
			log.Printf("Self-only mode enabled - will determine identity from first request")
		}

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup

		// Start DNS health checker if magicDNSSuffix is provided
		if *magicDNSSuffix != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				monitorDNS(ctx, *name, *magicDNSSuffix, *checkInterval, cancel)
			}()
		}

		// Start port 80 status server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := serveStatusPage(ctx, srv, *allowSelfOnly, *name, *magicDNSSuffix); err != nil {
				log.Printf("Error serving status page on port 80: %v", err)
			}
		}()

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

		// Clean up server before restart
		srv.Close()

		// If context wasn't cancelled (no DNS failure), exit normally
		select {
		case <-ctx.Done():
			log.Printf("Restarting tsproxy due to DNS failure...")
			time.Sleep(5 * time.Second) // Brief delay before restart
		default:
			log.Printf("tsproxy shutting down normally")
			return
		}
	}
}

func monitorDNS(ctx context.Context, hostname, magicDNSSuffix string, checkInterval time.Duration, cancelFunc context.CancelFunc) {
	fullHostname := fmt.Sprintf("%s.%s", hostname, magicDNSSuffix)
	log.Printf("Starting DNS health monitoring for %s (checking every %v)", fullHostname, checkInterval)

	// Use Tailscale's DNS resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 5 * time.Second,
			}
			return d.DialContext(ctx, network, "100.100.100.100:53")
		},
	}

	consecutiveFailures := 0
	const maxFailures = 3
	successfulChecks := 0

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
			addrs, err := resolver.LookupHost(checkCtx, fullHostname)
			checkCancel()

			if err != nil {
				consecutiveFailures++
				log.Printf("DNS check failed (%d/%d): %v", consecutiveFailures, maxFailures, err)

				if consecutiveFailures >= maxFailures {
					log.Printf("DNS has failed %d consecutive times, triggering restart", maxFailures)
					cancelFunc()
					return
				}
			} else {
				if consecutiveFailures > 0 {
					log.Printf("DNS check recovered: %s resolves to %v", fullHostname, addrs)
				}
				consecutiveFailures = 0
				successfulChecks++

				// Log every 10 successful checks to show DNS monitoring is working
				if successfulChecks%10 == 0 {
					log.Printf("DNS health check: %s resolves to %v (%d checks)", fullHostname, addrs, successfulChecks)
				}
			}
		}
	}
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

	// Shutdown server when context is cancelled
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	err = server.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func serveStatusPage(ctx context.Context, srv *tsnet.Server, allowSelfOnly bool, hostname string, magicDNSSuffix string) error {
	ln, err := srv.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("failed to listen on port 80: %v", err)
	}
	defer ln.Close()

	lc, err := srv.LocalClient()
	if err != nil {
		return fmt.Errorf("failed to get local client: %v", err)
	}

	var allowedIdentity string
	var identityLock sync.Mutex

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
				allowedIdentity = identity
				log.Printf("[Port 80] Restricting access to: %s", allowedIdentity)
			}
			identityLock.Unlock()

			if identity != allowedIdentity {
				log.Printf("[Port 80] DENIED %s %s %s (from %s, expected %s)", r.Method, r.URL.Path, r.Proto, identity, allowedIdentity)
				http.Error(w, "Forbidden: Access restricted to owner only", http.StatusForbidden)
				return
			}
		}

		log.Printf("[Port 80] %s %s %s (from %s)", r.Method, r.URL.Path, r.Proto, identity)

		// Get listening ports
		ports, err := getListeningPorts()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting listening ports: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<html><head><title>%s - Listening Ports</title></head><body>", hostname)
		fmt.Fprintf(w, "<h1>%s</h1>", hostname)
		fmt.Fprintf(w, "<h2>Listening Ports</h2>")
		fmt.Fprintf(w, "<table border='1' cellpadding='5' cellspacing='0'>")
		fmt.Fprintf(w, "<tr><th>Port</th><th>Process</th><th>PID</th><th>Command</th></tr>")
		for _, p := range ports {
			// Build full hostname for links
			fullHostname := hostname
			if magicDNSSuffix != "" {
				fullHostname = fmt.Sprintf("%s.%s", hostname, magicDNSSuffix)
			}
			portLink := fmt.Sprintf("http://%s:%d/", fullHostname, p.Port)
			fmt.Fprintf(w, "<tr><td><a href='%s' target='_blank'>%d</a></td><td>%s</td><td>%s</td><td>%s</td></tr>",
				portLink, p.Port, p.Process, p.PID, p.Command)
		}
		fmt.Fprintf(w, "</table></body></html>")
	})

	log.Printf("Serving status page on port 80")

	server := &http.Server{
		Handler: handler,
	}

	// Shutdown server when context is cancelled
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	err = server.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type PortInfo struct {
	Port    int
	Process string
	PID     string
	Command string
}

func getListeningPorts() ([]PortInfo, error) {
	// Check if /proc exists (Linux)
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		// Non-Linux system, return empty list
		return []PortInfo{}, nil
	}

	var ports []PortInfo
	seen := make(map[int]bool)

	// Parse /proc/net/tcp for IPv4
	if tcpPorts, err := parseProcNetTCP("/proc/net/tcp"); err == nil {
		for _, p := range tcpPorts {
			if !seen[p.Port] {
				ports = append(ports, p)
				seen[p.Port] = true
			}
		}
	}

	// Parse /proc/net/tcp6 for IPv6
	if tcp6Ports, err := parseProcNetTCP("/proc/net/tcp6"); err == nil {
		for _, p := range tcp6Ports {
			if !seen[p.Port] {
				ports = append(ports, p)
				seen[p.Port] = true
			}
		}
	}

	// Sort ports by port number
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	return ports, nil
}

func parseProcNetTCP(path string) ([]PortInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ports []PortInfo
	scanner := bufio.NewScanner(file)

	// Skip header line
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// Field 1 is local_address (format: hex_ip:hex_port)
		// Field 3 is st (state) - 0A = listening
		// Field 7 is uid
		// Field 9 is inode

		if fields[3] != "0A" {
			// Not in listening state
			continue
		}

		// Parse port from local_address
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		port, err := strconv.ParseInt(portHex, 16, 64)
		if err != nil {
			continue
		}

		inode := fields[9]

		// Find process info by inode
		process, pid, command := findProcessByInode(inode)

		ports = append(ports, PortInfo{
			Port:    int(port),
			Process: process,
			PID:     pid,
			Command: command,
		})
	}

	return ports, scanner.Err()
}

func findProcessByInode(inode string) (string, string, string) {
	// Scan /proc/*/fd/* to find which process has this socket
	procDirs, err := filepath.Glob("/proc/[0-9]*/fd/*")
	if err != nil {
		return "unknown", "", ""
	}

	searchStr := fmt.Sprintf("socket:[%s]", inode)

	for _, fdPath := range procDirs {
		linkTarget, err := os.Readlink(fdPath)
		if err != nil {
			continue
		}

		if linkTarget == searchStr {
			// Extract PID from path
			parts := strings.Split(fdPath, "/")
			if len(parts) < 3 {
				continue
			}
			pid := parts[2]

			// Read process name from /proc/PID/comm
			commPath := filepath.Join("/proc", pid, "comm")
			commData, err := os.ReadFile(commPath)
			if err != nil {
				return "unknown", pid, ""
			}
			process := strings.TrimSpace(string(commData))

			// Read command line from /proc/PID/cmdline
			cmdlinePath := filepath.Join("/proc", pid, "cmdline")
			cmdlineData, err := os.ReadFile(cmdlinePath)
			if err != nil {
				return process, pid, ""
			}
			// Replace null bytes with spaces
			command := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
			command = strings.TrimSpace(command)

			return process, pid, command
		}
	}

	return "unknown", "", ""
}
