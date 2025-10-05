package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_handleNavigate(t *testing.T) {
	s := New(11111)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid body",
			method:         http.MethodPost,
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					body, err = json.Marshal(tt.body)
					if err != nil {
						t.Fatalf("failed to marshal body: %v", err)
					}
				}
			}

			req := httptest.NewRequest(tt.method, "/api/navigate", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			s.handleNavigate(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestServer_handleReadConsole(t *testing.T) {
	s := New(11111)
	s.consoleLogs = []string{"log1", "log2", "log3"}

	req := httptest.NewRequest(http.MethodGet, "/api/read_console", nil)
	rec := httptest.NewRecorder()

	s.handleReadConsole(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	logs, ok := response["logs"].([]interface{})
	if !ok {
		t.Fatalf("expected logs array in response")
	}

	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
}

func TestServer_handleClearConsole(t *testing.T) {
	s := New(11111)
	s.consoleLogs = []string{"log1", "log2"}

	req := httptest.NewRequest(http.MethodPost, "/api/clear_console", nil)
	rec := httptest.NewRecorder()

	s.handleClearConsole(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	s.consoleMutex.RLock()
	defer s.consoleMutex.RUnlock()
	if len(s.consoleLogs) != 0 {
		t.Errorf("expected console logs to be cleared, got %d logs", len(s.consoleLogs))
	}
}

func TestServer_jsonResponse(t *testing.T) {
	s := New(11111)

	rec := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	s.jsonResponse(rec, data)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["message"] != "test" {
		t.Errorf("expected message 'test', got '%s'", response["message"])
	}
}

func TestServer_jsonError(t *testing.T) {
	s := New(11111)

	rec := httptest.NewRecorder()
	s.jsonError(rec, "test error", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "test error" {
		t.Errorf("expected error 'test error', got '%s'", response["error"])
	}
}
