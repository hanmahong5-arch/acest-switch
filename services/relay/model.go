package relay

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ReplaceModelInRequestBody 替换请求体中的模型名
// 使用 gjson + sjson 实现高性能 JSON 操作，避免完整反序列化
func ReplaceModelInRequestBody(bodyBytes []byte, newModel string) ([]byte, error) {
	result := gjson.GetBytes(bodyBytes, "model")
	if !result.Exists() {
		return bodyBytes, fmt.Errorf("请求体中未找到 model 字段")
	}

	modified, err := sjson.SetBytes(bodyBytes, "model", newModel)
	if err != nil {
		return bodyBytes, fmt.Errorf("替换模型名失败: %w", err)
	}

	return modified, nil
}

// MatchModel 检查请求的模型是否匹配 Provider 的白名单
// 支持精确匹配和通配符匹配（如 "claude-*"）
func MatchModel(requestModel string, supportedModels map[string]bool) bool {
	if len(supportedModels) == 0 {
		return true // 空白名单表示支持所有模型
	}

	// 精确匹配
	if supportedModels[requestModel] {
		return true
	}

	// 通配符匹配
	for pattern := range supportedModels {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(requestModel, prefix) {
				return true
			}
		}
	}

	return false
}

// MapModel 根据映射规则转换模型名
// 支持精确匹配和通配符映射
func MapModel(requestModel string, modelMapping map[string]string) string {
	if len(modelMapping) == 0 {
		return requestModel
	}

	// 精确匹配
	if mapped, ok := modelMapping[requestModel]; ok {
		return mapped
	}

	// 通配符映射（如 "claude-*" -> "anthropic/claude-*"）
	for pattern, target := range modelMapping {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(requestModel, prefix) {
				// 如果目标也包含 *，替换为实际后缀
				if strings.Contains(target, "*") {
					suffix := strings.TrimPrefix(requestModel, prefix)
					return strings.Replace(target, "*", suffix, 1)
				}
				return target
			}
		}
	}

	return requestModel
}
