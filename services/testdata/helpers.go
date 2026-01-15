package testdata

import (
	"fmt"
)

// Provider represents a test provider configuration
type Provider struct {
	ID              int
	Name            string
	BaseURL         string
	APIKey          string
	Level           int
	Priority        int
	SupportedModels map[string]bool
	ModelMapping    map[string]string
}

// MockProviderList returns a list of test providers
func MockProviderList() []Provider {
	return []Provider{
		{
			ID:       1,
			Name:     "Anthropic-Primary",
			BaseURL:  "https://api.anthropic.com",
			APIKey:   "sk-test-key-1",
			Level:    1,
			Priority: 1,
			SupportedModels: map[string]bool{
				"claude-sonnet-4": true,
				"claude-opus-4":   true,
				"claude-*":        true,
			},
		},
		{
			ID:       2,
			Name:     "OpenAI-Fallback",
			BaseURL:  "https://api.openai.com/v1",
			APIKey:   "sk-test-key-2",
			Level:    2,
			Priority: 1,
			SupportedModels: map[string]bool{
				"gpt-4":   true,
				"gpt-4-*": true,
			},
		},
		{
			ID:       3,
			Name:     "Gemini-Fallback",
			BaseURL:  "https://generativelanguage.googleapis.com",
			APIKey:   "sk-test-key-3",
			Level:    3,
			Priority: 1,
			SupportedModels: map[string]bool{
				"gemini-*": true,
			},
		},
	}
}

// MockClaudeRequest creates a mock Claude API request
func MockClaudeRequest(model string, content string) []byte {
	return []byte(fmt.Sprintf(`{
		"model": "%s",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": "%s"}
		]
	}`, model, content))
}

// MockClaudeStreamRequest creates a mock Claude streaming request
func MockClaudeStreamRequest(model string, content string) []byte {
	return []byte(fmt.Sprintf(`{
		"model": "%s",
		"max_tokens": 1024,
		"stream": true,
		"messages": [
			{"role": "user", "content": "%s"}
		]
	}`, model, content))
}

// MockCodexRequest creates a mock Codex/OpenAI request
func MockCodexRequest(model string, content string) []byte {
	return []byte(fmt.Sprintf(`{
		"model": "%s",
		"messages": [
			{"role": "user", "content": "%s"}
		]
	}`, model, content))
}

// MockClaudeResponse creates a mock Claude API response
func MockClaudeResponse(messageID string, content string, inputTokens int, outputTokens int) []byte {
	return []byte(fmt.Sprintf(`{
		"id": "%s",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "%s"}
		],
		"model": "claude-sonnet-4",
		"usage": {
			"input_tokens": %d,
			"output_tokens": %d
		},
		"stop_reason": "end_turn"
	}`, messageID, content, inputTokens, outputTokens))
}

// MockOpenAIResponse creates a mock OpenAI API response
func MockOpenAIResponse(messageID string, content string, inputTokens int, outputTokens int) []byte {
	return []byte(fmt.Sprintf(`{
		"id": "%s",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "gpt-4",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "%s"
				},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": %d,
			"completion_tokens": %d,
			"total_tokens": %d
		}
	}`, messageID, content, inputTokens, outputTokens, inputTokens+outputTokens))
}

// MockSSEEvent creates a mock Server-Sent Event
func MockSSEEvent(eventType string, data string) string {
	if eventType != "" {
		return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	}
	return fmt.Sprintf("data: %s\n\n", data)
}

// MockClaudeStreamEvent creates a mock Claude streaming event
func MockClaudeStreamEvent(eventType string, delta string) string {
	var data string
	switch eventType {
	case "content_block_start":
		data = `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
	case "content_block_delta":
		data = fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"%s"}}`, delta)
	case "content_block_stop":
		data = `{"type":"content_block_stop","index":0}`
	case "message_start":
		data = `{"type":"message_start","message":{"id":"msg-test","type":"message","role":"assistant"}}`
	case "message_delta":
		data = `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`
	case "message_stop":
		data = `{"type":"message_stop"}`
	default:
		data = fmt.Sprintf(`{"type":"%s"}`, eventType)
	}
	return MockSSEEvent("", data)
}
