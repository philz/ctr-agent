package browser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SendCommand sends a command to the headless server without expecting a complex response
func SendCommand(port int, command string, params map[string]interface{}) error {
	url := fmt.Sprintf("http://localhost:%d/api/%s", port, command)

	var body io.Reader
	if params != nil {
		jsonData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if params != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if msg, ok := errResp["error"]; ok {
				return fmt.Errorf("server error: %s", msg)
			}
		}
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// SendCommandWithResponse sends a command and returns the response
func SendCommandWithResponse(port int, command string, params map[string]interface{}) (string, error) {
	url := fmt.Sprintf("http://localhost:%d/api/%s", port, command)

	var body io.Reader
	method := http.MethodGet
	if params != nil {
		method = http.MethodPost
		jsonData, err := json.Marshal(params)
		if err != nil {
			return "", fmt.Errorf("failed to marshal params: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if params != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			if msg, ok := errResp["error"]; ok {
				return "", fmt.Errorf("server error: %s", msg)
			}
		}
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return string(bodyBytes), nil
}
