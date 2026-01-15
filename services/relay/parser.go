// Package relay 提供 HTTP 代理中继功能的模块化实现
package relay

import (
	"strings"

	"github.com/tidwall/gjson"
)

// UsageData 用量数据结构，用于解析器回调
type UsageData struct {
	InputTokens       int
	OutputTokens      int
	CacheCreateTokens int
	CacheReadTokens   int
	ReasoningTokens   int
}

// ParserFunc 定义解析器函数类型
type ParserFunc func(data string, usage *UsageData)

// ParseEventPayload 解析 SSE 事件负载
func ParseEventPayload(payload string, parser ParserFunc, usage *UsageData) {
	lines := strings.Split(payload, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			parser(strings.TrimPrefix(line, "data: "), usage)
		}
	}
}

// ClaudeCodeParser 解析 Claude Code API 响应中的 token 用量
func ClaudeCodeParser(data string, usage *UsageData) {
	// message.usage 格式（Claude API）
	usage.InputTokens += int(gjson.Get(data, "message.usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "message.usage.output_tokens").Int())
	usage.CacheCreateTokens += int(gjson.Get(data, "message.usage.cache_creation_input_tokens").Int())
	usage.CacheReadTokens += int(gjson.Get(data, "message.usage.cache_read_input_tokens").Int())

	// usage 格式（直接）
	usage.InputTokens += int(gjson.Get(data, "usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.output_tokens").Int())
}

// CodexParser 解析 Codex/OpenAI 兼容 API 响应中的 token 用量
func CodexParser(data string, usage *UsageData) {
	// OpenAI Responses API 格式 (response.usage)
	usage.InputTokens += int(gjson.Get(data, "response.usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "response.usage.output_tokens").Int())
	usage.CacheReadTokens += int(gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
	usage.ReasoningTokens += int(gjson.Get(data, "response.usage.output_tokens_details.reasoning_tokens").Int())

	// 标准 OpenAI Chat Completions 格式 (usage.prompt_tokens / completion_tokens)
	usage.InputTokens += int(gjson.Get(data, "usage.prompt_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.completion_tokens").Int())

	// 兼容 input_tokens/output_tokens 格式
	usage.InputTokens += int(gjson.Get(data, "usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.output_tokens").Int())

	// DeepSeek 推理 tokens
	usage.ReasoningTokens += int(gjson.Get(data, "usage.completion_tokens_details.reasoning_tokens").Int())
	// 缓存 tokens
	usage.CacheReadTokens += int(gjson.Get(data, "usage.prompt_tokens_details.cached_tokens").Int())
}

// GetParser 根据平台类型返回对应的解析器
func GetParser(platform string) ParserFunc {
	if platform == "codex" {
		return CodexParser
	}
	return ClaudeCodeParser
}
