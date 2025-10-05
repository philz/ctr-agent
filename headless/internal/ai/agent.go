package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"
const modelName = "claude-3-5-sonnet-20241022"

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ContentBlock struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	ID      string                 `json:"id,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Input   map[string]interface{} `json:"input,omitempty"`
	Content interface{}            `json:"content,omitempty"` // For tool_result
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type AnthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
}

type AnthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	Usage        interface{}    `json:"usage"`
	Error        *APIError      `json:"error,omitempty"`
}

type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// BrowserContext wraps either a standalone browser or a connection to the server
type BrowserContext struct {
	ctx        context.Context
	cancel     context.CancelFunc
	standalone bool
	serverPort int
}

func newStandaloneBrowser() (*BrowserContext, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel2 := chromedp.NewContext(allocCtx)

	return &BrowserContext{
		ctx:        ctx,
		cancel:     func() { cancel2(); cancel() },
		standalone: true,
	}, nil
}

func newServerBrowser(port int) *BrowserContext {
	return &BrowserContext{
		serverPort: port,
		standalone: false,
	}
}

func (bc *BrowserContext) Close() {
	if bc.standalone && bc.cancel != nil {
		bc.cancel()
	}
}

func (bc *BrowserContext) Navigate(url string) error {
	if bc.standalone {
		return chromedp.Run(bc.ctx, chromedp.Navigate(url))
	}
	// Use HTTP API
	return sendHTTPCommand(bc.serverPort, "navigate", map[string]interface{}{"url": url})
}

func (bc *BrowserContext) Evaluate(js string) (string, error) {
	if bc.standalone {
		var result interface{}
		if err := chromedp.Run(bc.ctx, chromedp.Evaluate(js, &result)); err != nil {
			return "", err
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}
	// Use HTTP API
	return sendHTTPCommandWithResponse(bc.serverPort, "eval", map[string]interface{}{"js": js})
}

func (bc *BrowserContext) Screenshot(path string) error {
	if bc.standalone {
		var buf []byte
		if err := chromedp.Run(bc.ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return err
		}
		return os.WriteFile(path, buf, 0644)
	}
	// Use HTTP API
	return sendHTTPCommand(bc.serverPort, "screenshot", map[string]interface{}{"path": path})
}

func sendHTTPCommand(port int, command string, params map[string]interface{}) error {
	url := fmt.Sprintf("http://localhost:%d/api/%s", port, command)
	jsonData, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func sendHTTPCommandWithResponse(port int, command string, params map[string]interface{}) (string, error) {
	url := fmt.Sprintf("http://localhost:%d/api/%s", port, command)
	jsonData, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return string(body), nil
}

func getTools() []Tool {
	return []Tool{
		{
			Name:        "navigate",
			Description: "Navigate the browser to a URL",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to navigate to",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "evaluate",
			Description: "Evaluate JavaScript in the browser and return the result",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"js": map[string]interface{}{
						"type":        "string",
						"description": "The JavaScript code to evaluate",
					},
				},
				"required": []string{"js"},
			},
		},
		{
			Name:        "screenshot",
			Description: "Take a screenshot of the current page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The file path where to save the screenshot",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

func RunAgenticLoop(apiKey, prompt string, serverPort int, standalone bool) (string, error) {
	var bc *BrowserContext
	var err error

	if standalone {
		bc, err = newStandaloneBrowser()
		if err != nil {
			return "", fmt.Errorf("failed to create standalone browser: %w", err)
		}
		defer bc.Close()
	} else {
		bc = newServerBrowser(serverPort)
	}

	messages := []Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	tools := getTools()
	maxIterations := 10

	for i := 0; i < maxIterations; i++ {
		response, err := callClaude(apiKey, messages, tools)
		if err != nil {
			return "", fmt.Errorf("Claude API error: %w", err)
		}

		if response.Error != nil {
			return "", fmt.Errorf("Claude API error: %s - %s", response.Error.Type, response.Error.Message)
		}

		// Check if we have a final answer
		if response.StopReason == "end_turn" {
			// Extract text response
			var finalText string
			for _, block := range response.Content {
				if block.Type == "text" {
					finalText += block.Text
				}
			}
			return finalText, nil
		}

		// Process tool uses
		if response.StopReason == "tool_use" {
			var toolResults []ContentBlock

			for _, block := range response.Content {
				if block.Type == "tool_use" {
					result, err := executeTool(bc, block.Name, block.Input)
					if err != nil {
						toolResults = append(toolResults, ContentBlock{
							Type:    "tool_result",
							ID:      block.ID,
							Content: fmt.Sprintf("Error: %v", err),
						})
					} else {
						toolResults = append(toolResults, ContentBlock{
							Type:    "tool_result",
							ID:      block.ID,
							Content: result,
						})
					}
				}
			}

			// Add assistant message with tool uses
			messages = append(messages, Message{
				Role:    "assistant",
				Content: response.Content,
			})

			// Add user message with tool results
			messages = append(messages, Message{
				Role:    "user",
				Content: toolResults,
			})

			continue
		}

		return "", fmt.Errorf("unexpected stop reason: %s", response.StopReason)
	}

	return "", fmt.Errorf("max iterations reached")
}

func executeTool(bc *BrowserContext, toolName string, input map[string]interface{}) (string, error) {
	switch toolName {
	case "navigate":
		url, ok := input["url"].(string)
		if !ok {
			return "", fmt.Errorf("invalid url parameter")
		}
		if err := bc.Navigate(url); err != nil {
			return "", err
		}
		return fmt.Sprintf("Navigated to %s", url), nil

	case "evaluate":
		js, ok := input["js"].(string)
		if !ok {
			return "", fmt.Errorf("invalid js parameter")
		}
		result, err := bc.Evaluate(js)
		if err != nil {
			return "", err
		}
		return result, nil

	case "screenshot":
		path, ok := input["path"].(string)
		if !ok {
			return "", fmt.Errorf("invalid path parameter")
		}
		if err := bc.Screenshot(path); err != nil {
			return "", err
		}
		return fmt.Sprintf("Screenshot saved to %s", path), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func callClaude(apiKey string, messages []Message, tools []Tool) (*AnthropicResponse, error) {
	reqBody := AnthropicRequest{
		Model:     modelName,
		MaxTokens: 4096,
		Messages:  messages,
		Tools:     tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, anthropicAPIURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response AnthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w (body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if response.Error != nil {
			return &response, nil
		}
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return &response, nil
}
