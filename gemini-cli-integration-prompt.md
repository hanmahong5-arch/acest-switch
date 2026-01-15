# Gemini-CLI 对接 Ailurus PaaS (CodeSwitch) 平台改造指南

## 改造目标

将 gemini-cli 工具改造为通过本地 Ailurus PaaS 平台（http://127.0.0.1:18100）转发请求，实现以下功能：
1. 所有 Gemini API 请求通过本地代理转发
2. 自动使用配置的 API Key
3. 支持多 Provider 负载均衡和自动故障转移
4. 请求日志和统计分析

## 核心改造点

### 1. API Endpoint 重定向

**原始代码**：
```javascript
const GEMINI_API_BASE = 'https://generativelanguage.googleapis.com/v1beta'
```

**改造后**：
```javascript
// 使用本地代理（OpenAI 兼容端点）
const GEMINI_API_BASE = 'http://127.0.0.1:18100/v1'

// 或者使用原生 Gemini 端点路由
const GEMINI_API_BASE = 'http://127.0.0.1:18100'
```

### 2. 请求格式转换（推荐使用 OpenAI 格式）

**方案 A：使用 OpenAI Chat Completions 格式（推荐）**

```javascript
// 将 Gemini-CLI 的请求转换为 OpenAI Chat Completions 格式
async function callGemini(messages, tools, model) {
  const payload = {
    model: model || 'gemini-3-flash-preview',  // 本地平台会自动路由到配置的 Provider
    messages: messages,
    tools: tools || [],
    stream: false  // 或 true，平台支持流式
  };

  const response = await fetch('http://127.0.0.1:18100/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
      // 不需要 Authorization header，平台会自动添加
    },
    body: JSON.stringify(payload)
  });

  return await response.json();
}
```

**响应格式**：
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1767187682,
  "model": "gemini-3-flash-preview",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "回答内容",
      "tool_calls": [{
        "id": "call_xxx",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"city\":\"烟台\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }],
  "usage": {
    "prompt_tokens": 107,
    "completion_tokens": 17,
    "total_tokens": 271
  }
}
```

**方案 B：使用 Gemini 原生格式**

如果要保持 Gemini 原生格式，平台也支持透传：

```javascript
async function callGeminiNative(contents, tools, model) {
  const payload = {
    contents: contents,  // Gemini 原生格式
    tools: tools || [],
    systemInstruction: {
      parts: [{ text: "系统提示" }]
    }
  };

  // 直接请求 Gemini 原生 API，平台会自动转换
  const response = await fetch(`http://127.0.0.1:18100/v1beta/models/${model}:generateContent?key=PLACEHOLDER`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });

  return await response.json();
}
```

### 3. API Key 处理

**删除或注释掉硬编码的 API Key**：
```javascript
// 不需要在客户端配置 API Key
// const GOOGLE_API_KEY = 'AIzaSy...'

// 平台会自动从以下配置文件读取：
// ~/.code-switch/codex.json (Codex 平台)
// ~/.code-switch/claude-code.json (Claude Code 平台)
```

### 4. 错误处理和重试

利用平台的自动故障转移功能：

```javascript
async function callWithRetry(payload, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch('http://127.0.0.1:18100/v1/chat/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (response.ok) {
        return await response.json();
      }

      // 平台会自动尝试下一个可用的 Provider
      console.warn(`请求失败 (${response.status})，平台正在切换 Provider...`);

      if (i === maxRetries - 1) {
        throw new Error(`所有 ${maxRetries} 次尝试均失败`);
      }
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await new Promise(r => setTimeout(r, 1000 * (i + 1)));  // 指数退避
    }
  }
}
```

### 5. 流式响应处理

如果使用流式模式：

```javascript
async function streamGemini(messages, onChunk) {
  const payload = {
    model: 'gemini-3-flash-preview',
    messages: messages,
    stream: true
  };

  const response = await fetch('http://127.0.0.1:18100/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });

  const reader = response.body.getReader();
  const decoder = new TextDecoder();

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    const chunk = decoder.decode(value);
    const lines = chunk.split('\n').filter(line => line.trim().startsWith('data:'));

    for (const line of lines) {
      const data = line.replace('data: ', '').trim();
      if (data === '[DONE]') return;

      try {
        const parsed = JSON.parse(data);
        onChunk(parsed.choices[0]?.delta);
      } catch (e) {
        console.error('解析 SSE 数据失败:', e);
      }
    }
  }
}

// 使用示例
await streamGemini([{ role: 'user', content: '你好' }], (delta) => {
  if (delta.content) {
    process.stdout.write(delta.content);
  }
});
```

## 配置示例

### Ailurus PaaS 配置文件（~/.code-switch/codex.json）

```json
{
  "providers": [
    {
      "id": 1735627200000,
      "name": "Google Gemini",
      "apiUrl": "https://generativelanguage.googleapis.com/v1beta/openai",
      "apiKey": "YOUR_GEMINI_API_KEY",
      "enabled": true,
      "supportedModels": {
        "gemini-*": true,
        "gemini-3-flash-preview": true
      },
      "modelMapping": {
        "acest": "gemini-3-flash-preview"
      },
      "level": 1
    },
    {
      "id": 1764233151106,
      "name": "Deepseek",
      "apiUrl": "https://api.deepseek.com",
      "apiKey": "YOUR_DEEPSEEK_API_KEY",
      "enabled": true,
      "supportedModels": {
        "acest": true,
        "deepseek-*": true
      },
      "modelMapping": {
        "acest": "deepseek-chat"
      },
      "level": 1
    }
  ]
}
```

**说明**：
- 两个 Provider 都支持 `acest` 模型
- 优先级相同（level=1）时，按配置顺序选择（Deepseek 优先）
- 如果 Deepseek 失败，自动切换到 Google Gemini

## 完整改造示例（Node.js）

```javascript
#!/usr/bin/env node

const PROXY_BASE_URL = 'http://127.0.0.1:18100';

class GeminiCLI {
  constructor(model = 'acest') {  // 默认使用 acest，会路由到 Deepseek
    this.model = model;
  }

  async chat(userMessage, tools = []) {
    const payload = {
      model: this.model,
      messages: [
        { role: 'user', content: userMessage }
      ],
      tools: tools.length > 0 ? tools : undefined,
      stream: false
    };

    try {
      const response = await fetch(`${PROXY_BASE_URL}/v1/chat/completions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${await response.text()}`);
      }

      const data = await response.json();
      return data.choices[0].message;
    } catch (error) {
      console.error('请求失败:', error.message);
      throw error;
    }
  }

  async chatWithTools(userMessage) {
    const tools = [
      {
        type: 'function',
        function: {
          name: 'get_weather',
          description: '获取指定城市的天气信息',
          parameters: {
            type: 'object',
            properties: {
              city: {
                type: 'string',
                description: '城市名称'
              },
              unit: {
                type: 'string',
                enum: ['celsius', 'fahrenheit']
              }
            },
            required: ['city']
          }
        }
      }
    ];

    const response = await this.chat(userMessage, tools);

    if (response.tool_calls) {
      console.log('模型请求调用工具:');
      for (const call of response.tool_calls) {
        console.log(`  - ${call.function.name}(${call.function.arguments})`);
      }
    } else {
      console.log('模型回复:', response.content);
    }

    return response;
  }
}

// 使用示例
async function main() {
  const cli = new GeminiCLI('acest');  // 使用 acest 模型（路由到 Deepseek）

  console.log('测试1: 普通对话');
  await cli.chat('你好，介绍一下自己');

  console.log('\n测试2: Tool Calling');
  await cli.chatWithTools('今天烟台的天气怎么样？');
}

main().catch(console.error);
```

## 验证改造是否成功

### 1. 启动 Ailurus PaaS 平台
```bash
./codeswitch.exe  # Windows
# 或
./CodeSwitch      # macOS/Linux
```

### 2. 运行改造后的 gemini-cli
```bash
node gemini-cli.js
```

### 3. 检查平台日志
在 Ailurus PaaS GUI 中查看：
- **日志页面**：查看请求记录，确认 Provider 路由
- **统计页面**：查看 Token 使用量和成本
- **热力图**：查看请求分布

### 4. 检查请求路由
```bash
# 查询最近的请求记录
sqlite3 ~/.code-switch/app.db "SELECT created_at, model, provider, http_code, input_tokens, output_tokens FROM request_log ORDER BY created_at DESC LIMIT 5;"
```

预期输出：
```
2025-12-31 21:30:00|acest|Deepseek|200|351|69
2025-12-31 21:29:00|gemini-3-flash-preview|Google Gemini|200|101|24
```

## 高级功能

### 多模型切换
```javascript
// 根据任务复杂度动态选择模型
const simpleTask = new GeminiCLI('acest');           // Deepseek (快速+便宜)
const complexTask = new GeminiCLI('gemini-3-flash-preview');  // Gemini (高级推理)
```

### 自定义 Provider 优先级
编辑 `~/.code-switch/codex.json`：
```json
{
  "providers": [
    {
      "name": "Deepseek",
      "level": 1,  // 最高优先级
      "enabled": true
    },
    {
      "name": "Google Gemini",
      "level": 2,  // 备用
      "enabled": true
    }
  ]
}
```

### 启用 Round-Robin 负载均衡
在 Ailurus PaaS GUI 中开启「轮询模式」，请求将在所有启用的 Provider 间均匀分配。

## 故障排查

### 问题1: 连接拒绝
```
Error: connect ECONNREFUSED 127.0.0.1:18100
```
**解决**：确保 Ailurus PaaS 平台已启动

### 问题2: 所有 Provider 不可用
```json
{
  "error": {
    "message": "所有 Provider 均不可用",
    "type": "no_available_provider"
  }
}
```
**解决**：检查 `~/.code-switch/codex.json` 中至少有一个 enabled=true 且 API Key 有效的 Provider

### 问题3: Token 统计为 0
```
input_tokens=0, output_tokens=0
```
**解决**：这是 Gemini 原生 API 的已知问题，平台会自动从响应中提取 token 统计。如果仍为 0，请更新到最新版本。

## 总结

改造要点：
1. ✅ 修改 API Base URL 为 `http://127.0.0.1:18100`
2. ✅ 使用 OpenAI Chat Completions 格式（推荐）
3. ✅ 删除客户端 API Key 配置
4. ✅ 利用平台的自动故障转移和负载均衡
5. ✅ 在平台 GUI 中监控请求和成本

改造后的优势：
- 🔒 **API Key 安全**：密钥存储在本地配置文件
- 🔄 **自动故障转移**：Provider 失败自动切换
- 📊 **统一监控**：所有请求的日志和统计
- 💰 **成本控制**：实时追踪 Token 使用和费用
- ⚡ **性能优化**：请求缓存和批处理
