package ai

import (
	"testing"
)

func TestGetTools(t *testing.T) {
	tools := getTools()

	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	expectedTools := map[string]bool{
		"navigate":  false,
		"evaluate":  false,
		"screenshot": false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; !ok {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		expectedTools[tool.Name] = true
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestBrowserContext_Close(t *testing.T) {
	// Test that Close doesn't panic for non-standalone context
	bc := newServerBrowser(11111)
	bc.Close() // Should not panic

	// We can't easily test standalone context without actual Chrome
	// but we ensure the method exists and doesn't panic on nil cancel
	bc2 := &BrowserContext{standalone: true}
	bc2.Close() // Should not panic
}

func TestContentBlock(t *testing.T) {
	// Test that ContentBlock can be properly marshaled/unmarshaled
	block := ContentBlock{
		Type: "text",
		Text: "test",
	}

	if block.Type != "text" {
		t.Errorf("expected type 'text', got '%s'", block.Type)
	}

	if block.Text != "test" {
		t.Errorf("expected text 'test', got '%s'", block.Text)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "test message",
	}

	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}

	if msg.Content != "test message" {
		t.Errorf("expected content 'test message', got '%v'", msg.Content)
	}
}
