#!/bin/bash
# 初始化 NATS JetStream Streams
# 需要安装 nats CLI: https://github.com/nats-io/natscli

NATS_URL="${NATS_URL:-nats://localhost:4222}"

echo "Connecting to NATS at $NATS_URL..."

# 等待 NATS 启动
for i in {1..30}; do
    if nats server ping -s "$NATS_URL" > /dev/null 2>&1; then
        echo "NATS is ready!"
        break
    fi
    echo "Waiting for NATS... ($i/30)"
    sleep 1
done

# 1. 聊天消息流 - 永久保存
echo "Creating CHAT_MESSAGES stream..."
nats stream add CHAT_MESSAGES \
    --subjects "chat.*.*.msg" \
    --retention limits \
    --max-age 0 \
    --max-bytes 10GB \
    --storage file \
    --replicas 1 \
    --discard old \
    --dupe-window 1h \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update CHAT_MESSAGES \
    --subjects "chat.*.*.msg" \
    -s "$NATS_URL" 2>/dev/null

# 2. 会话状态流 - 短期保存
echo "Creating SESSION_STATUS stream..."
nats stream add SESSION_STATUS \
    --subjects "chat.*.*.status,chat.*.*.typing" \
    --retention limits \
    --max-age 1d \
    --storage memory \
    --replicas 1 \
    --discard old \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update SESSION_STATUS \
    --subjects "chat.*.*.status,chat.*.*.typing" \
    -s "$NATS_URL" 2>/dev/null

# 3. 用户事件流（包含配额变更）
echo "Creating USER_EVENTS stream..."
nats stream add USER_EVENTS \
    --subjects "user.*.auth,user.*.presence,user.*.notification,user.*.quota" \
    --retention limits \
    --max-age 7d \
    --storage file \
    --replicas 1 \
    --discard old \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update USER_EVENTS \
    --subjects "user.*.auth,user.*.presence,user.*.notification,user.*.quota" \
    -s "$NATS_URL" 2>/dev/null

# 4. LLM 请求流
echo "Creating LLM_REQUESTS stream..."
nats stream add LLM_REQUESTS \
    --subjects "llm.request.*,llm.response.*" \
    --retention limits \
    --max-age 1h \
    --storage memory \
    --replicas 1 \
    --discard old \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update LLM_REQUESTS \
    --subjects "llm.request.*,llm.response.*" \
    -s "$NATS_URL" 2>/dev/null

# 5. 审计日志流 - 长期保存
echo "Creating AUDIT_LOG stream..."
nats stream add AUDIT_LOG \
    --subjects "admin.audit" \
    --retention limits \
    --max-age 365d \
    --storage file \
    --replicas 1 \
    --discard old \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update AUDIT_LOG \
    --subjects "admin.audit" \
    -s "$NATS_URL" 2>/dev/null

# 6. 系统广播流
echo "Creating SYSTEM_BROADCAST stream..."
nats stream add SYSTEM_BROADCAST \
    --subjects "admin.broadcast,admin.metrics" \
    --retention limits \
    --max-age 1d \
    --storage memory \
    --replicas 1 \
    --discard old \
    -s "$NATS_URL" \
    --defaults 2>/dev/null || \
nats stream update SYSTEM_BROADCAST \
    --subjects "admin.broadcast,admin.metrics" \
    -s "$NATS_URL" 2>/dev/null

echo ""
echo "=== JetStream Streams Status ==="
nats stream list -s "$NATS_URL"

echo ""
echo "JetStream initialization complete!"
