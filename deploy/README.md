# CodeSwitch 基础设施部署指南

> **v4.1 开源框架版** - 使用 Casdoor + Lago + gopay

## 目录结构

```
deploy/
├── docker/
│   ├── docker-compose.yml    # Docker Compose 配置 (完整栈)
│   ├── .env.example          # 环境变量模板
│   ├── .env                  # 实际环境变量 (gitignore)
│   └── casdoor/
│       └── app.conf          # Casdoor 配置文件
├── nats/
│   ├── nats-server.conf      # NATS 服务器配置
│   └── init-streams.sh       # JetStream 初始化脚本
├── postgres/
│   ├── init-databases.sql    # 多数据库初始化
│   └── init-codeswitch.sql   # CodeSwitch 扩展表
├── scripts/
│   ├── start-stack.ps1       # Windows 启动脚本 (完整栈)
│   ├── init-lago.sh          # Lago 计费配置初始化
│   ├── start-infra.ps1       # Windows 启动脚本 (旧)
│   ├── start-infra.sh        # Linux/macOS 启动脚本
│   └── health-check.ps1      # 健康检查脚本
└── README.md                 # 本文档
```

## 服务组件

| 组件 | 端口 | 说明 |
|------|------|------|
| **Casdoor** | 8000 | 身份认证 (OAuth2/OIDC/SSO) |
| **Lago API** | 3001 | 计费系统 API |
| **Lago UI** | 8080 | 计费系统管理界面 |
| PostgreSQL | 5432 | 数据库 |
| Redis | 6379 | 缓存 |
| NATS | 4222 | 消息总线 |
| pgAdmin | 5050 | 数据库管理 (dev) |

## 快速开始

### 1. 准备环境

确保已安装:
- Docker Desktop 或 Docker Engine
- Docker Compose v2

### 2. 配置环境变量

```bash
cd deploy/docker
cp .env.example .env
# 编辑 .env 文件，设置以下必填项:
# - POSTGRES_PASSWORD
# - REDIS_PASSWORD
# - LAGO_SECRET_KEY (openssl rand -hex 64)
```

### 3. 启动完整栈

**Windows (PowerShell):**
```powershell
cd deploy/scripts
.\start-stack.ps1

# 启动开发工具 (pgAdmin, Redis Commander)
.\start-stack.ps1 -Dev

# 查看日志
.\start-stack.ps1 -Logs

# 停止服务
.\start-stack.ps1 -Down
```

**Linux/macOS:**
```bash
cd deploy/docker
docker-compose up -d

# 启动开发工具
docker-compose --profile dev up -d
```

### 4. 初始化服务

**a) 初始化 NATS JetStream:**
```bash
cd deploy/nats
chmod +x init-streams.sh
./init-streams.sh
```

**b) 配置 Casdoor:**
1. 访问 http://localhost:8000
2. 默认账号: admin / 123
3. 创建 Application (CodeSwitch)
4. 配置 OAuth 回调 URL
5. 启用社交登录 (GitHub/WeChat)

**c) 配置 Lago:**
1. 访问 http://localhost:8080
2. 注册管理员账号
3. 获取 API Key
4. 运行初始化脚本:
```bash
export LAGO_API_KEY=your_api_key
cd deploy/scripts
chmod +x init-lago.sh
./init-lago.sh
```

### 5. 验证服务

```powershell
cd deploy/scripts
.\health-check.ps1
```

访问各服务界面:
- Casdoor: http://localhost:8000
- Lago UI: http://localhost:8080
- NATS Monitor: http://localhost:8222

## 服务端点

| 服务 | 端口 | 说明 |
|------|------|------|
| NATS | 4222 | 客户端连接 |
| NATS Monitor | 8222 | HTTP 监控 API |
| NATS WebSocket | 8223 | WebSocket 连接 |
| PostgreSQL | 5432 | 数据库 |
| Redis | 6379 | 缓存 |
| pgAdmin | 5050 | 数据库管理 (dev) |
| Redis Commander | 8081 | Redis 管理 (dev) |

## 常用命令

### 服务管理

```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f
docker-compose logs -f nats

# 停止服务
docker-compose down

# 清理数据 (危险!)
docker-compose down -v
```

### NATS 管理

```bash
# 查看 JetStream 状态
nats account info

# 列出 Streams
nats stream list

# 查看 Stream 详情
nats stream info CHAT_MESSAGES

# 查看消费者
nats consumer list CHAT_MESSAGES
```

### PostgreSQL 管理

```bash
# 连接数据库
docker exec -it codeswitch-postgres psql -U codeswitch -d codeswitch

# 备份数据库
docker exec codeswitch-postgres pg_dump -U codeswitch codeswitch > backup.sql

# 恢复数据库
cat backup.sql | docker exec -i codeswitch-postgres psql -U codeswitch -d codeswitch
```

### Redis 管理

```bash
# 连接 Redis
docker exec -it codeswitch-redis redis-cli

# 常用命令
> KEYS *
> INFO
> DBSIZE
> FLUSHDB  # 清空当前数据库 (危险!)
```

## JetStream Streams

| Stream | Subjects | 保留策略 | 说明 |
|--------|----------|----------|------|
| CHAT_MESSAGES | chat.*.*.msg | 永久 (10GB) | 聊天消息 |
| SESSION_STATUS | chat.*.*.status, typing | 1天 (内存) | 会话状态 |
| USER_EVENTS | user.*.auth, presence, notification | 7天 | 用户事件 |
| LLM_REQUESTS | llm.request.*, response.* | 1小时 (内存) | LLM 请求 |
| AUDIT_LOG | admin.audit | 365天 | 审计日志 |
| SYSTEM_BROADCAST | admin.broadcast, metrics | 1天 (内存) | 系统广播 |

## 数据库表

| 表名 | 说明 |
|------|------|
| users | 用户信息 |
| devices | 设备绑定 |
| sessions | 会话列表 |
| messages | 消息内容 |
| message_attachments | 消息附件 |
| audit_logs | 审计日志 |
| alert_rules | 告警规则 |
| alert_history | 告警历史 |
| system_config | 系统配置 |
| quota_history | 配额变动 |
| daily_stats | 日统计汇总 |

## 故障排查

### NATS 连接失败

1. 检查容器状态: `docker ps | grep nats`
2. 查看日志: `docker logs codeswitch-nats`
3. 检查端口: `netstat -an | grep 4222`

### PostgreSQL 连接失败

1. 检查容器状态: `docker ps | grep postgres`
2. 查看日志: `docker logs codeswitch-postgres`
3. 测试连接: `docker exec codeswitch-postgres pg_isready`

### Redis 连接失败

1. 检查容器状态: `docker ps | grep redis`
2. 查看日志: `docker logs codeswitch-redis`
3. 测试连接: `docker exec codeswitch-redis redis-cli ping`

## 生产环境建议

1. **密码安全**: 使用强密码，通过环境变量注入
2. **数据备份**: 定期备份 PostgreSQL 和 NATS JetStream 数据
3. **监控告警**: 配置 Prometheus + Grafana 监控
4. **日志收集**: 配置 ELK 或 Loki 收集日志
5. **TLS 加密**: 生产环境启用 TLS
6. **资源限制**: 配置 Docker 资源限制
