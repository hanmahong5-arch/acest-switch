package services

import (
	"encoding/json"
	"fmt"
)

// GeminiConverter 处理OpenAI与Gemini原生API格式之间的转换
type GeminiConverter struct{}

// GeminiContent Gemini原生API的content结构
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role"` // "user" or "model"
}

// GeminiPart Gemini内容部分
type GeminiPart struct {
	Text             string                 `json:"text,omitempty"`
	ThoughtSignature string                 `json:"thoughtSignature,omitempty"`
	FunctionCall     *GeminiFunctionCall    `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	InlineData       *GeminiInlineData      `json:"inlineData,omitempty"`
}

// GeminiFunctionCall Gemini函数调用
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// GeminiFunctionResponse Gemini函数响应
type GeminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// GeminiInlineData 内联数据（图片等）
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiRequest Gemini原生请求格式
type GeminiRequest struct {
	Contents          []GeminiContent       `json:"contents"`
	Tools             []GeminiTool          `json:"tools,omitempty"`
	SystemInstruction *GeminiSystemInstruction `json:"systemInstruction,omitempty"`
}

// GeminiTool Gemini工具定义
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations"`
}

// GeminiFunctionDeclaration Gemini函数声明
type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GeminiSystemInstruction 系统指令
type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiResponse Gemini原生响应格式
type GeminiResponse struct {
	Candidates    []GeminiCandidate   `json:"candidates"`
	UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
	ModelVersion  string              `json:"modelVersion,omitempty"`
}

// GeminiCandidate 候选响应
type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

// GeminiUsageMetadata 使用统计
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
	ThoughtsTokenCount   int `json:"thoughtsTokenCount,omitempty"`
}

// ConvertOpenAIToGemini 将OpenAI格式的请求转换为Gemini原生格式
func (gc *GeminiConverter) ConvertOpenAIToGemini(openAIRequest map[string]interface{}) (*GeminiRequest, error) {
	geminiReq := &GeminiRequest{
		Contents: []GeminiContent{},
	}

	// 转换messages
	messages, ok := openAIRequest["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid messages format")
	}

	var systemMessage string
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)

		// 处理system消息
		if role == "system" {
			if content, ok := msgMap["content"].(string); ok {
				systemMessage = content
			}
			continue
		}

		// 转换role: assistant -> model, user -> user
		geminiRole := role
		if role == "assistant" {
			geminiRole = "model"
		}

		content := GeminiContent{
			Role:  geminiRole,
			Parts: []GeminiPart{},
		}

		// 处理文本内容
		if textContent, ok := msgMap["content"].(string); ok && textContent != "" {
			content.Parts = append(content.Parts, GeminiPart{
				Text: textContent,
			})
		}

		// 处理tool_calls
		if toolCalls, ok := msgMap["tool_calls"].([]interface{}); ok {
			for _, tc := range toolCalls {
				tcMap, _ := tc.(map[string]interface{})
				if function, ok := tcMap["function"].(map[string]interface{}); ok {
					name, _ := function["name"].(string)
					argsStr, _ := function["arguments"].(string)

					var args map[string]interface{}
					if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
						content.Parts = append(content.Parts, GeminiPart{
							FunctionCall: &GeminiFunctionCall{
								Name: name,
								Args: args,
							},
						})
					}
				}
			}
		}

		// 处理tool响应
		if role == "tool" {
			if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
				if contentStr, ok := msgMap["content"].(string); ok {
					var response map[string]interface{}
					if err := json.Unmarshal([]byte(contentStr), &response); err != nil {
						response = map[string]interface{}{"result": contentStr}
					}

					// 查找对应的function name
					functionName := toolCallID // 默认使用tool_call_id

					content.Parts = append(content.Parts, GeminiPart{
						FunctionResponse: &GeminiFunctionResponse{
							Name:     functionName,
							Response: response,
						},
					})
				}
			}
		}

		if len(content.Parts) > 0 {
			geminiReq.Contents = append(geminiReq.Contents, content)
		}
	}

	// 添加系统指令
	if systemMessage != "" {
		geminiReq.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{
				{Text: systemMessage},
			},
		}
	}

	// 转换tools
	if tools, ok := openAIRequest["tools"].([]interface{}); ok && len(tools) > 0 {
		geminiTools := GeminiTool{
			FunctionDeclarations: []GeminiFunctionDeclaration{},
		}

		for _, tool := range tools {
			toolMap, _ := tool.(map[string]interface{})
			if function, ok := toolMap["function"].(map[string]interface{}); ok {
				name, _ := function["name"].(string)
				description, _ := function["description"].(string)
				parameters, _ := function["parameters"].(map[string]interface{})

				geminiTools.FunctionDeclarations = append(geminiTools.FunctionDeclarations, GeminiFunctionDeclaration{
					Name:        name,
					Description: description,
					Parameters:  parameters,
				})
			}
		}

		if len(geminiTools.FunctionDeclarations) > 0 {
			geminiReq.Tools = []GeminiTool{geminiTools}
		}
	}

	return geminiReq, nil
}

// ConvertGeminiToOpenAI 将Gemini原生响应转换为OpenAI格式
func (gc *GeminiConverter) ConvertGeminiToOpenAI(geminiResp *GeminiResponse, model string) map[string]interface{} {
	openAIResp := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", generateID()),
		"object":  "chat.completion",
		"created": getCurrentTimestamp(),
		"model":   model,
		"choices": []interface{}{},
		"usage": map[string]interface{}{
			"prompt_tokens":     geminiResp.UsageMetadata.PromptTokenCount,
			"completion_tokens": geminiResp.UsageMetadata.CandidatesTokenCount,
			"total_tokens":      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}

	if len(geminiResp.Candidates) == 0 {
		return openAIResp
	}

	candidate := geminiResp.Candidates[0]
	message := map[string]interface{}{
		"role": "assistant",
	}

	// 提取文本内容和tool_calls
	var textContent string
	var toolCalls []interface{}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}

		if part.FunctionCall != nil {
			argsBytes, _ := json.Marshal(part.FunctionCall.Args)
			toolCall := map[string]interface{}{
				"id":   fmt.Sprintf("call_%d", generateID()),
				"type": "function",
				"function": map[string]interface{}{
					"name":      part.FunctionCall.Name,
					"arguments": string(argsBytes),
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	if textContent != "" {
		message["content"] = textContent
	}

	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	// 转换finish_reason
	finishReason := "stop"
	switch candidate.FinishReason {
	case "STOP":
		finishReason = "stop"
	case "MAX_TOKENS":
		finishReason = "length"
	case "SAFETY":
		finishReason = "content_filter"
	case "RECITATION":
		finishReason = "content_filter"
	}

	openAIResp["choices"] = []interface{}{
		map[string]interface{}{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		},
	}

	return openAIResp
}

func generateID() int64 {
	return getCurrentTimestamp()
}

func getCurrentTimestamp() int64 {
	return 1735660800 // Placeholder
}

// ConvertGeminiRequestToOpenAI 将 Gemini 原生请求格式转换为 OpenAI 格式
func (gc *GeminiConverter) ConvertGeminiRequestToOpenAI(geminiReq map[string]interface{}, model string) (map[string]interface{}, error) {
	openAIReq := make(map[string]interface{})
	openAIReq["model"] = model

	// 转换 contents 到 messages
	contents, ok := geminiReq["contents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid contents format")
	}

	messages := []interface{}{}
	for _, contentRaw := range contents {
		content, ok := contentRaw.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := content["role"].(string)
		if role == "model" {
			role = "assistant"
		}

		parts, ok := content["parts"].([]interface{})
		if !ok {
			continue
		}

		// 合并所有 text parts
		var textContent string
		for _, partRaw := range parts {
			part, ok := partRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if text, exists := part["text"]; exists {
				if textStr, ok := text.(string); ok {
					textContent += textStr
				}
			}
		}

		if textContent != "" {
			messages = append(messages, map[string]interface{}{
				"role":    role,
				"content": textContent,
			})
		}
	}

	openAIReq["messages"] = messages

	// 转换 tools（如果存在）
	if tools, ok := geminiReq["tools"].([]interface{}); ok && len(tools) > 0 {
		openAITools := []interface{}{}
		for _, toolRaw := range tools {
			tool, ok := toolRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if funcDecls, ok := tool["functionDeclarations"].([]interface{}); ok {
				for _, declRaw := range funcDecls {
					decl, ok := declRaw.(map[string]interface{})
					if !ok {
						continue
					}
					openAITools = append(openAITools, map[string]interface{}{
						"type": "function",
						"function": map[string]interface{}{
							"name":        decl["name"],
							"description": decl["description"],
							"parameters":  decl["parameters"],
						},
					})
				}
			}
		}
		if len(openAITools) > 0 {
			openAIReq["tools"] = openAITools
		}
	}

	// 处理 systemInstruction
	if sysInst, ok := geminiReq["systemInstruction"].(map[string]interface{}); ok {
		if parts, ok := sysInst["parts"].([]interface{}); ok {
			var systemText string
			for _, partRaw := range parts {
				part, ok := partRaw.(map[string]interface{})
				if !ok {
					continue
				}
				if text, exists := part["text"]; exists {
					if textStr, ok := text.(string); ok {
						systemText += textStr
					}
				}
			}
			if systemText != "" {
				// 将 system instruction 插入到 messages 开头
				messages = append([]interface{}{
					map[string]interface{}{
						"role":    "system",
						"content": systemText,
					},
				}, messages...)
				openAIReq["messages"] = messages
			}
		}
	}

	return openAIReq, nil
}
