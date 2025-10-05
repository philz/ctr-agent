package browser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		params         map[string]interface{}
		responseStatus int
		responseBody   interface{}
		wantErr        bool
	}{
		{
			name:           "successful command",
			command:        "navigate",
			params:         map[string]interface{}{"url": "https://example.com"},
			responseStatus: http.StatusOK,
			responseBody:   map[string]string{"message": "success"},
			wantErr:        false,
		},
		{
			name:           "server error",
			command:        "navigate",
			params:         map[string]interface{}{"url": "invalid"},
			responseStatus: http.StatusInternalServerError,
			responseBody:   map[string]string{"error": "navigation failed"},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/"+tt.command {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.responseStatus)
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			// Extract port from server URL
			var port int
			_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
			if err != nil {
				t.Fatalf("failed to extract port: %v", err)
			}

			err = SendCommand(port, tt.command, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSendCommandWithResponse(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		params         map[string]interface{}
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		wantContains   string
	}{
		{
			name:           "successful eval",
			command:        "eval",
			params:         map[string]interface{}{"js": "2+2"},
			responseStatus: http.StatusOK,
			responseBody:   map[string]interface{}{"result": 4},
			wantErr:        false,
			wantContains:   "result",
		},
		{
			name:           "read console",
			command:        "read_console",
			params:         nil,
			responseStatus: http.StatusOK,
			responseBody:   map[string]interface{}{"logs": []string{"log1", "log2"}},
			wantErr:        false,
			wantContains:   "logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			var port int
			_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
			if err != nil {
				t.Fatalf("failed to extract port: %v", err)
			}

			result, err := SendCommandWithResponse(port, tt.command, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendCommandWithResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantContains != "" {
				if !contains(result, tt.wantContains) {
					t.Errorf("SendCommandWithResponse() result doesn't contain %q, got %q", tt.wantContains, result)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
