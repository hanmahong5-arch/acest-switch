# Ailurus PaaS (Code Switch)

[![GitHub Release](https://img.shields.io/github/v/release/hanmahong5-arch/acest-switch)](https://github.com/hanmahong5-arch/acest-switch/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![Wails](https://img.shields.io/badge/wails-v3-green.svg)](https://wails.io)

集中管理 Claude Code、Codex、Gemini CLI 供应商的统一 AI 网关

🌐 **GitHub**: [https://github.com/hanmahong5-arch/acest-switch](https://github.com/hanmahong5-arch/acest-switch)

## 核心特性

### 统一 LLM 调用
- **NEW-API 统一网关**：所有 AI 请求通过 NEW-API (localhost:3000) 统一路由
- **多平台支持**：Claude Code、Codex CLI、Gemini CLI 统一接入
- **格式自动转换**：Gemini Native API ↔ OpenAI 格式自动转换

### 智能路由
- 无需重启 CLI，平滑切换不同供应商
- 支持多供应商自动降级，保证使用体验
- 支持按优先级和 Round-Robin 负载均衡

### 统一支付体系
- 使用 NEW-API 的配额管理系统
- 请求级别的用量统计，花费清晰可见
- 配额变更通过 NATS 实时广播

### MCP & Skill 管理
- 支持 Claude Code & Codex MCP Server 双平台管理
- 支持 Claude Skill 自动下载与安装
- 内置 2 个流行的 skill 仓库，支持添加自定义仓库

### NATS 消息总线
- 多端消息同步 (会话、消息、状态)
- LLM 请求/响应事件发布
- 配额变更事件广播

### 运维监控后台 (Admin)
- **仪表盘**: 系统状态、统计概览、在线用户
- **用户管理**: 禁用/启用用户、设置管理员权限
- **会话管理**: 查看/删除会话及消息
- **统计分析**: 小时/日统计、供应商/模型分析
- **审计日志**: 管理操作记录、支持时间筛选
- **告警系统**: 自定义告警规则、Webhook 通知、告警去重

基于 [Wails 3](https://v3.wails.io)

---

## 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│  Client Layer (Claude Code / Codex / Gemini CLI)                │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  CodeSwitch Gateway (:18100)                                    │
│  ├─ /v1/messages      → Claude (Anthropic format)               │
│  ├─ /responses        → Codex (OpenAI Responses API)            │
│  ├─ /v1/chat/completions → Generic (OpenAI format)              │
│  └─ /v1beta/models/*  → Gemini (Native → OpenAI 转换)           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │ NEW-API Mode    │ Fallback        │
          ▼                 ▼                 │
┌─────────────────┐ ┌─────────────────────────┐
│  NEW-API :3000  │ │  Local Provider List    │
│  (统一网关)      │ │  (按优先级自动降级)      │
└─────────────────┘ └─────────────────────────┘
```

---

## 实现原理

应用启动时会在本地 18100 端口创建 HTTP 代理服务器，并自动更新 Claude Code、Codex、Gemini CLI 配置，指向 `http://127.0.0.1:18100`。

代理内部暴露的关键端点：
- `/v1/messages` - 转发到 Claude 供应商 (Anthropic 格式)
- `/responses` - 转发到 Codex 供应商 (OpenAI Responses API)
- `/v1/chat/completions` - 通用 OpenAI 兼容端点
- `/v1beta/models/*` - Gemini 原生 API (自动转换格式)

**NEW-API 统一网关模式** (推荐)：
- 所有请求转发到 NEW-API (localhost:3000)
- 由 NEW-API 统一管理配额、路由、计费
- 支持 40+ AI 供应商

---

## 配置

### App Settings (`~/.code-switch/app.json`)

```json
{
  "show_heatmap": true,
  "show_home_title": true,
  "auto_start": false,
  "enable_body_log": false,
  "new_api_enabled": true,
  "new_api_url": "http://localhost:3000",
  "new_api_token": "sk-your-token"
}
```

### NATS 同步 (可选)

启用 NATS 后，支持：
- 多设备消息同步
- 配额变更事件 (`user.{user_id}.quota`)
- LLM 请求/响应事件 (`llm.request.*`, `llm.response.*`)

---

## 下载

[macOS](https://github.com/hanmahong5-arch/acest-switch/releases) | [Windows](https://github.com/hanmahong5-arch/acest-switch/releases)

---

## 预览

![亮色主界面](resources/images/code-switch.png)
![暗色主界面](resources/images/code-swtich-dark.png)
![日志亮色](resources/images/code-switch-logs.png)
![日志暗色](resources/images/code-switch-logs-dark.png)

---

## 开发准备

- Go 1.24+
- Node.js 18+
- npm / pnpm / yarn
- Wails 3 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## 开发运行

```bash
wails3 task dev
```

## 构建流程

1. 同步 build metadata：
   ```bash
   wails3 task common:update:build-assets
   ```
2. 打包 macOS `.app`：
   ```bash
   wails3 task package
   ```

### 交叉编译 Windows (macOS 环境)

1. 安装 `mingw-w64`：
   ```bash
   brew install mingw-w64
   ```
2. 运行 Windows 任务：
   ```bash
   env ARCH=amd64 wails3 task windows:build
   env ARCH=amd64 wails3 task windows:package
   ```

---

## 目录结构

```
codeswitch/
├── main.go                    # 程序入口
├── services/
│   ├── providerrelay.go       # HTTP 代理 + NEW-API 转发
│   ├── providerservice.go     # Provider CRUD
│   ├── appsettings.go         # 应用设置 (含 NEW-API 配置)
│   ├── sync_integration.go    # NATS 事件钩子
│   └── sync/                  # NATS 客户端
│       ├── sync_service.go    # 同步服务 + LLM 消费者
│       └── nats_client.go     # NATS 连接管理
├── frontend/                  # Vue 3 前端
│   └── src/components/Admin/  # 运维监控后台
├── deploy/
│   └── nats/
│       └── init-streams.sh    # NATS JetStream 初始化
└── sync-service/              # 同步服务 (独立部署)
    └── internal/
        ├── api/               # HTTP API (Gin)
        ├── admin/             # 管理服务
        │   ├── stats.go       # 统计 (带缓存)
        │   ├── audit.go       # 审计日志
        │   └── alerts.go      # 告警系统
        ├── auth/              # JWT 认证
        └── nats/              # NATS 客户端
```

---

## 常见问题

- 若 `.app` 无法打开，先执行 `wails3 task common:update:build-assets` 后再构建。
- macOS 交叉编译需要终端拥有完全磁盘访问权限。
- NEW-API 模式需要先部署 [new-api](https://github.com/songquanpeng/one-api) 服务。

---

## 版本历史

| 版本 | 更新内容 |
|------|---------|
| v0.3.0 | 运维监控后台 (Admin) + 告警系统 + 审计日志 + 熔断器 + 代理控制 |
| v0.2.0 | NEW-API 统一网关 + NATS 消息总线 + 计费集成 |
| v0.1.9 | Gemini CLI 支持 + 格式转换 |
| v0.1.8 | 写入队列优化 + 价格预计算 |

---

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 Apache License 2.0 许可证，详见 [LICENSE](LICENSE) 文件。

## 致谢

本项目基于以下优秀开源项目构建：
- [Wails](https://wails.io) - Go + Web 桌面应用框架
- [Vue 3](https://vuejs.org) - 渐进式 JavaScript 框架
- [Naive UI](https://www.naiveui.com) - Vue 3 组件库
- [NEW-API](https://github.com/Calcium-Ion/new-api) - LLM 统一网关
- [NATS](https://nats.io) - 云原生消息系统
