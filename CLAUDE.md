# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Prerequisites

- Go 1.24+
- Node.js 18+
- Wails 3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## Build & Development Commands

```bash
# Development mode (hot reload)
wails3 task dev

# Build for current platform
wails3 task build

# Build production package (sync build metadata first if .app won't open)
wails3 task common:update:build-assets
wails3 task package

# Run tests
go test ./...

# Run specific test file
go test ./services/providerservice_test.go

# Frontend only (from frontend/ directory)
npm run dev          # Dev server
npm run build        # Production build
npm run build:dev    # Development build

# Windows cross-compile from macOS (requires mingw-w64)
env ARCH=amd64 wails3 task windows:build
env ARCH=amd64 wails3 task windows:package
```

## System Architecture

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
└────────┬────────┘ └───────────┬─────────────┘
         │                      │
         └──────────┬───────────┘
                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                       AI Providers                               │
│  (OpenAI / Anthropic / Google Gemini / DeepSeek / ...)          │
└─────────────────────────────────────────────────────────────────┘
```

### NEW-API 统一网关模式

启用 NEW-API 模式后 (`app.json` 中 `new_api_enabled: true`)：
- 所有 LLM 请求统一转发到 NEW-API
- 由 NEW-API 处理配额、计费、多供应商路由
- 支持 40+ AI 供应商

### NATS 消息总线 (可选)

启用 NATS 后支持：
- 多端消息同步 (`chat.{user_id}.{session}.msg`)
- LLM 请求/响应事件 (`llm.request.*`, `llm.response.*`)
- 配额变更广播 (`user.{user_id}.quota`)

### Core Data Flow

```
Claude Code/Codex CLI
        │
        ▼ HTTP Request
┌───────────────────┐
│  :18100 Proxy     │ ◄─── Provider Config (JSON)
│  ProviderRelay    │
└────────┬──────────┘
         │
         ▼ Model Matching
┌───────────────────┐
│  Provider Select  │ ◄─── Round-Robin / Priority
│  + Failover       │
└────────┬──────────┘
         │
         ▼ Forward Request
┌───────────────────┐
│  AI Provider API  │
└────────┬──────────┘
         │
         ▼ Response + Logging
┌───────────────────┐
│  SQLite (app.db)  │ ◄─── Write Queue (Async)
│  Request Logs     │
└───────────────────┘
```

### Key Backend Services (in `services/`)

| Service | File | Purpose |
|---------|------|---------|
| `ProviderRelayService` | `providerrelay.go` | HTTP proxy, NEW-API forwarding, Gemini conversion |
| `ProviderService` | `providerservice.go` | CRUD for provider configs, model validation |
| `LogService` | `logservice.go` | Request logging, statistics, cost analysis |
| `SyncIntegration` | `sync_integration.go` | NATS event publishing hooks |
| `SyncService` | `sync/sync_service.go` | NATS client, LLM request consumer |
| `NATSClient` | `sync/nats_client.go` | NATS connection management |
| `MCPService` | `mcpservice.go` | MCP server configuration management |
| `SkillService` | `skillservice.go` | Claude skill repository management |
| `ClaudeSettingsService` | `claudesettings.go` | Manages Claude CLI config files |
| `CodexSettingsService` | `codexsettings.go` | Manages Codex CLI config files |
| `AppSettingsService` | `appsettings.go` | App settings + NEW-API configuration |
| `ImportService` | `importservice.go` | Import from external sources |

### Provider Configuration

Stored in `~/.code-switch/`:
```
~/.code-switch/
├── claude-code.json    # Claude Code providers
├── codex.json          # Codex providers
├── mcp.json            # MCP servers
├── app.json            # App settings (NEW-API config, NATS, etc.)
├── sync-settings.json  # NATS sync settings
└── app.db              # SQLite database (logs)
```

### NEW-API Configuration (`app.json`)

```json
{
  "new_api_enabled": true,
  "new_api_url": "http://localhost:3000",
  "new_api_token": "sk-your-token",
  "show_heatmap": true,
  "auto_start": false
}
```

### NATS Sync Settings (`sync-settings.json`)

```json
{
  "enabled": false,
  "url": "nats://localhost:4222"
}
```

### Proxy Routes

| Route | Platform | Format |
|-------|----------|--------|
| `POST /v1/messages` | Claude Code | Anthropic API |
| `POST /responses` | Codex | OpenAI Responses API |
| `POST /v1/chat/completions` | Generic | OpenAI-compatible |
| `POST /chat/completions` | Generic | OpenAI-compatible |
| `POST /v1beta/models/*` | Gemini CLI | Gemini Native (auto-converted) |

## Frontend Architecture

### UI Component Library

**Naive UI** (v2.x) - Vue 3 component library:
- NButton, NInput, NSwitch, NCard, NProgress, NTag
- NStatistic, NGrid, NIcon, NSpace, NDivider
- Theme support (dark/light) via NConfigProvider

### Component Hierarchy

```
App.vue (NConfigProvider theme wrapper)
├── Main/Index.vue (/)
│   ├── HeatmapSection.vue      # 热力图 (三态: loading/error/empty)
│   ├── AnalyticsSection.vue    # 运营分析卡片
│   ├── ProviderCard.vue        # 供应商卡片
│   └── ProviderModal.vue       # 供应商编辑弹窗
├── Gateway/Index.vue (/gateway) # NEW-API 网关配置
├── Sync/Index.vue (/sync)      # NATS 同步 + 配额显示
├── Logs/Index.vue (/logs)      # 请求日志
├── Mcp/index.vue (/mcp)        # MCP 服务器管理
├── Skill/Index.vue (/skill)    # Skill 仓库
├── General/Index.vue (/settings) # 设置页面
└── Admin/                      # 运维监控后台 (需管理员权限)
    ├── Index.vue               # Admin Layout
    ├── Dashboard.vue           # 仪表盘
    ├── Users/Index.vue         # 用户管理
    ├── Sessions/Index.vue      # 会话管理
    ├── Stats/Index.vue         # 统计分析
    ├── Audit/Index.vue         # 审计日志
    └── Alerts/Index.vue        # 告警管理
```

### Key Frontend Modules

| Path | Purpose |
|------|---------|
| `services/*.ts` | Backend API bindings |
| `services/gateway.ts` | NEW-API config + connection test |
| `services/sync.ts` | NATS sync service client |
| `services/admin.ts` | Admin API (users, sessions, stats, alerts) |
| `stores/providers.ts` | Pinia store for provider state |
| `stores/admin.ts` | Admin module state management |
| `data/*.ts` | Data transformations (heatmap, cards) |
| `types/error.ts` | Unified error type system |
| `utils/toast.ts` | Toast notification (4 levels) |
| `locales/*.json` | i18n (zh/en) |

### Error Handling System

统一错误类型 (`types/error.ts`):
```typescript
interface AppError {
  code: ErrorCode       // 错误代码 (NETWORK_ERROR, TIMEOUT, etc.)
  message: string       // 用户友好消息
  details?: string      // 技术详情 (调试用)
  timestamp: Date
  context?: { component, action, ... }
  retryable?: boolean
}
```

三态 UI 模式:
- **Loading**: 加载动画 + 提示文字
- **Error**: 错误图标 + 消息 + 可展开详情 + 重试按钮
- **Empty**: 空状态图标 + 友好提示

### Model Matching Logic

Providers use `supportedModels` (whitelist) and `modelMapping` for routing:
- Exact match: `"claude-sonnet-4": true`
- Wildcard: `"claude-*": true` matches any `claude-` prefix
- Mapping: `"claude-*": "anthropic/claude-*"` transforms model names

### Dependencies

**Go Backend:**
- Wails 3 (desktop framework)
- Gin (HTTP router)
- gjson/sjson (JSON manipulation)
- modernc.org/sqlite (pure Go SQLite)

**Vue Frontend:**
- Vue 3 + Vite
- Tailwind CSS 4
- vue-i18n (internationalization)
- vue-chartjs (charts)
- vue-router

## Performance Optimizations

### Database Write Queue (Critical)

**Problem**: SQLite write lock contention caused deadlocks during concurrent streaming requests and statistics queries.

**Solution** (`services/providerrelay.go`):
- Single-threaded write queue (`logWriteQueue chan *ReqeustLog`, buffer 1000)
- Batch processing: 10 records or 100ms timeout
- Non-blocking queue insertion with overflow handling

```go
// Key components:
logWriteQueue chan *ReqeustLog  // Buffered channel for log entries
processLogWriteQueue()          // Single goroutine for all DB writes
```

### Price Pre-calculation

**Problem**: Statistics queries recalculated prices for every record, causing O(n×m) complexity.

**Solution**:
- Calculate prices during log insertion (before DB write)
- Store 7 price fields in database: `input_cost`, `output_cost`, `cache_create_cost`, `cache_read_cost`, `ephemeral_5m_cost`, `ephemeral_1h_cost`, `total_cost`
- Statistics queries read stored prices directly

### Model Name Lookup Cache

**Problem**: Fuzzy matching for model names caused O(n) lookups per price calculation.

**Solution** (`resources/model-pricing/price.go`):
- Added `sync.Map` cache for model name lookups
- Cache both found and not-found results

### HTTP Client with Proper Timeout

**Problem**: `xrequest` library's `WithContext` didn't properly apply timeouts, causing requests to hang indefinitely.

**Solution** (`services/providerrelay.go`):
- Replaced `xrequest` with Go standard library `http.Client`
- Timeouts: 60s for non-streaming, 300s (5 min) for streaming requests
- Proper connection pooling with `http.Transport`

### Frontend Re-entry Protection

**Problem**: Concurrent statistics queries during loading.

**Solution** (`frontend/src/components/Main/Index.vue`):
- Added `usageHeatmapLoading` and `providerStatsLoading` flags
- Early return if already loading

## Database Schema

Request logs are stored in SQLite (`~/.code-switch/app.db`):

```sql
CREATE TABLE request_log (
    id INTEGER PRIMARY KEY,
    trace_id TEXT,
    request_id TEXT,
    platform TEXT,           -- 'claude' or 'codex'
    model TEXT,
    provider TEXT,
    http_code INTEGER,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cache_create_tokens INTEGER,
    cache_read_tokens INTEGER,
    reasoning_tokens INTEGER,
    is_stream INTEGER,
    duration_sec REAL,
    -- Pre-calculated price fields (performance optimization)
    input_cost REAL,
    output_cost REAL,
    cache_create_cost REAL,
    cache_read_cost REAL,
    ephemeral_5m_cost REAL,
    ephemeral_1h_cost REAL,
    total_cost REAL,
    -- Metadata
    user_agent TEXT,
    client_ip TEXT,
    user_id TEXT,
    request_method TEXT,
    request_path TEXT,
    error_type TEXT,
    error_message TEXT,
    provider_error_code TEXT,
    created_at DATETIME
);
```

## Sync Service (独立后端服务)

Sync Service 是一个独立的 Go 后端服务，提供运维监控 API。

### 启动 Sync Service

```bash
cd sync-service
go run cmd/main.go
# 默认监听 :8081
```

### Sync Service 架构

```
sync-service/
├── cmd/main.go              # 入口
├── configs/config.yaml      # 配置文件
└── internal/
    ├── api/
    │   ├── router.go        # Gin 路由
    │   └── admin.go         # Admin API handlers
    ├── admin/
    │   ├── stats.go         # 统计服务 (带缓存)
    │   ├── audit.go         # 审计日志
    │   ├── alerts.go        # 告警系统 (去重/静默)
    │   ├── monitor.go       # 系统监控
    │   └── user.go          # 用户管理
    ├── auth/                # JWT 认证
    ├── nats/                # NATS 客户端
    ├── presence/            # 在线状态
    ├── session/             # 会话管理
    └── message/             # 消息处理
```

### Admin API Endpoints

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/v1/admin/system/status` | 系统状态 |
| GET | `/api/v1/admin/stats/overview` | 统计概览 |
| GET | `/api/v1/admin/stats/hourly` | 小时统计 |
| GET | `/api/v1/admin/stats/daily` | 日统计 |
| GET | `/api/v1/admin/users` | 用户列表 |
| POST | `/api/v1/admin/users/:id/disable` | 禁用用户 |
| GET | `/api/v1/admin/sessions` | 会话列表 |
| DELETE | `/api/v1/admin/sessions/:id` | 删除会话 |
| GET | `/api/v1/admin/audit-logs` | 审计日志 |
| GET | `/api/v1/admin/alert-rules` | 告警规则 |
| POST | `/api/v1/admin/alert-rules` | 创建规则 |
| GET | `/api/v1/admin/alert-history` | 告警历史 |

### 告警系统特性

- **去重**: 同一规则 5 分钟内只触发一次 (可配置)
- **静默期**: 避免告警风暴
- **Webhook 通知**: 支持外部告警通道
- **支持指标**: error_rate, latency_avg, latency_p99, cost_daily, requests_per_minute

## Troubleshooting

### Application Freezes on "Refresh Statistics"

**Symptoms**: GUI becomes unresponsive when clicking refresh button, especially during active requests.

**Root Cause**: Database write lock contention between streaming response logging and statistics queries.

**Fix**: Implemented in v0.1.8+ with write queue batching. If issue persists:
1. Check logs for `[WARN] 日志队列已满` messages
2. Verify `processLogWriteQueue` goroutine is running (look for startup log)

### Requests Hang Without Response

**Symptoms**: Requests sent but never complete, no timeout occurs.

**Root Cause**: HTTP client not properly applying context timeout.

**Fix**: Using standard `http.Client` with explicit `Timeout` field. Stream timeout is 5 minutes.

### Statistics Show Zero Cost

**Symptoms**: Older records show $0.00 cost in statistics.

**Cause**: Price fields were added after initial records. Only new records have pre-calculated prices.

**Workaround**: Historical data cost calculation is not retroactively applied.
