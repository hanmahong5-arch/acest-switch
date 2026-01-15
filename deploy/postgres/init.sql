-- CodeSwitch 数据库初始化脚本
-- PostgreSQL 16+

-- 启用必要扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";  -- 用于模糊搜索

-- ============================================
-- 用户表 (从 NEW-API 同步)
-- ============================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    newapi_user_id VARCHAR(64) UNIQUE NOT NULL,
    username VARCHAR(128) NOT NULL,
    email VARCHAR(256),
    avatar_url VARCHAR(512),
    plan VARCHAR(32) DEFAULT 'free',
    quota_total DECIMAL(12,4) DEFAULT 0,
    quota_used DECIMAL(12,4) DEFAULT 0,
    is_admin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_newapi_id ON users(newapi_user_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users USING gin(username gin_trgm_ops);

-- ============================================
-- 设备表
-- ============================================
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(128) NOT NULL,
    device_name VARCHAR(128),
    device_type VARCHAR(32) NOT NULL, -- desktop, mobile, cli, web
    os_info VARCHAR(128),
    client_version VARCHAR(32),
    push_token VARCHAR(512),           -- 推送通知 token
    is_trusted BOOLEAN DEFAULT FALSE,  -- 可信设备
    last_seen_at TIMESTAMPTZ,
    last_ip INET,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_devices_user ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen_at DESC);

-- ============================================
-- 会话表
-- ============================================
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(256),
    summary TEXT,                      -- AI 生成的会话摘要
    model VARCHAR(64),                 -- 主要使用的模型
    provider VARCHAR(64),              -- 主要使用的供应商
    message_count INT DEFAULT 0,
    token_count INT DEFAULT 0,
    cost DECIMAL(12,6) DEFAULT 0,
    is_pinned BOOLEAN DEFAULT FALSE,   -- 置顶
    is_archived BOOLEAN DEFAULT FALSE, -- 归档
    is_shared BOOLEAN DEFAULT FALSE,   -- 是否分享
    share_token VARCHAR(64),           -- 分享链接 token
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_active ON sessions(user_id, is_archived, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_share ON sessions(share_token) WHERE share_token IS NOT NULL;

-- ============================================
-- 消息表
-- ============================================
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    parent_id UUID REFERENCES messages(id),  -- 用于分支对话
    role VARCHAR(16) NOT NULL,         -- user, assistant, system, tool
    content TEXT NOT NULL,
    content_type VARCHAR(32) DEFAULT 'text', -- text, markdown, code, image
    model VARCHAR(64),
    provider VARCHAR(64),
    tokens_input INT DEFAULT 0,
    tokens_output INT DEFAULT 0,
    tokens_reasoning INT DEFAULT 0,
    tokens_cache_read INT DEFAULT 0,
    tokens_cache_create INT DEFAULT 0,
    cost DECIMAL(12,6) DEFAULT 0,
    duration_ms INT,                   -- 响应耗时
    finish_reason VARCHAR(32),         -- stop, length, tool_calls, error
    error_message TEXT,
    metadata JSONB,                    -- 工具调用、引用等
    is_edited BOOLEAN DEFAULT FALSE,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_user ON messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_parent ON messages(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_content ON messages USING gin(content gin_trgm_ops);

-- ============================================
-- 消息附件表 (图片、文件等)
-- ============================================
CREATE TABLE IF NOT EXISTS message_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name VARCHAR(256) NOT NULL,
    file_type VARCHAR(64) NOT NULL,    -- image/png, application/pdf, etc.
    file_size INT NOT NULL,
    storage_path VARCHAR(512) NOT NULL,
    thumbnail_path VARCHAR(512),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON message_attachments(message_id);

-- ============================================
-- 审计日志表
-- ============================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64),
    resource_id VARCHAR(128),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    device_id VARCHAR(128),
    result VARCHAR(16) NOT NULL,       -- success, failure, blocked
    error_message TEXT,
    duration_ms INT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);

-- 分区表 (按月分区，可选)
-- CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
--     FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- ============================================
-- 告警规则表
-- ============================================
CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    description TEXT,
    metric VARCHAR(64) NOT NULL,
    operator VARCHAR(4) NOT NULL,      -- >, <, >=, <=, ==, !=
    threshold DECIMAL(12,4) NOT NULL,
    duration_seconds INT DEFAULT 60,
    severity VARCHAR(16) DEFAULT 'warning', -- info, warning, critical
    channels JSONB DEFAULT '[]',       -- ["email", "webhook", "slack"]
    webhook_url VARCHAR(512),
    enabled BOOLEAN DEFAULT TRUE,
    last_triggered_at TIMESTAMPTZ,
    trigger_count INT DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled, metric);

-- ============================================
-- 告警历史表
-- ============================================
CREATE TABLE IF NOT EXISTS alert_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    metric_value DECIMAL(12,4) NOT NULL,
    threshold DECIMAL(12,4) NOT NULL,
    severity VARCHAR(16) NOT NULL,
    message TEXT,
    notified_channels JSONB,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_history_rule ON alert_history(rule_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_history_unresolved ON alert_history(created_at DESC) WHERE resolved_at IS NULL;

-- ============================================
-- 系统配置表
-- ============================================
CREATE TABLE IF NOT EXISTS system_config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    description TEXT,
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 初始化系统配置
INSERT INTO system_config (key, value, description) VALUES
    ('sync.enabled', 'true', '是否启用多端同步'),
    ('sync.max_devices_per_user', '5', '每用户最大设备数'),
    ('sync.message_retention_days', '365', '消息保留天数'),
    ('quota.default_free', '1000', '免费用户默认配额'),
    ('quota.default_pro', '100000', 'Pro用户默认配额'),
    ('rate_limit.requests_per_minute', '60', '每分钟请求限制'),
    ('rate_limit.tokens_per_day', '1000000', '每日Token限制')
ON CONFLICT (key) DO NOTHING;

-- ============================================
-- 用户配额历史表 (追踪配额变动)
-- ============================================
CREATE TABLE IF NOT EXISTS quota_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    change_type VARCHAR(32) NOT NULL,  -- usage, recharge, bonus, adjustment
    amount DECIMAL(12,4) NOT NULL,
    balance_before DECIMAL(12,4) NOT NULL,
    balance_after DECIMAL(12,4) NOT NULL,
    description TEXT,
    related_session_id UUID REFERENCES sessions(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quota_history_user ON quota_history(user_id, created_at DESC);

-- ============================================
-- 统计汇总表 (预聚合，提升查询性能)
-- ============================================
CREATE TABLE IF NOT EXISTS daily_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stat_date DATE NOT NULL,
    user_id UUID REFERENCES users(id),  -- NULL 表示全局统计
    provider VARCHAR(64),
    model VARCHAR(64),
    request_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    error_count INT DEFAULT 0,
    tokens_input BIGINT DEFAULT 0,
    tokens_output BIGINT DEFAULT 0,
    tokens_total BIGINT DEFAULT 0,
    cost_total DECIMAL(12,6) DEFAULT 0,
    avg_duration_ms INT,
    p50_duration_ms INT,
    p95_duration_ms INT,
    p99_duration_ms INT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(stat_date, user_id, provider, model)
);

CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(stat_date DESC);
CREATE INDEX IF NOT EXISTS idx_daily_stats_user ON daily_stats(user_id, stat_date DESC);

-- ============================================
-- 触发器: 自动更新 updated_at
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_alert_rules_updated_at
    BEFORE UPDATE ON alert_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- 触发器: 更新会话统计
-- ============================================
CREATE OR REPLACE FUNCTION update_session_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE sessions SET
            message_count = message_count + 1,
            token_count = token_count + NEW.tokens_input + NEW.tokens_output,
            cost = cost + NEW.cost,
            last_message_at = NEW.created_at,
            updated_at = NOW()
        WHERE id = NEW.session_id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_session_stats
    AFTER INSERT ON messages
    FOR EACH ROW EXECUTE FUNCTION update_session_stats();

-- ============================================
-- 视图: 用户会话概览
-- ============================================
CREATE OR REPLACE VIEW user_session_overview AS
SELECT
    u.id AS user_id,
    u.username,
    COUNT(DISTINCT s.id) AS total_sessions,
    COUNT(DISTINCT s.id) FILTER (WHERE NOT s.is_archived) AS active_sessions,
    SUM(s.message_count) AS total_messages,
    SUM(s.token_count) AS total_tokens,
    SUM(s.cost) AS total_cost,
    MAX(s.last_message_at) AS last_activity
FROM users u
LEFT JOIN sessions s ON u.id = s.user_id
GROUP BY u.id, u.username;

-- ============================================
-- 视图: 今日统计
-- ============================================
CREATE OR REPLACE VIEW today_stats AS
SELECT
    COUNT(*) AS request_count,
    COUNT(*) FILTER (WHERE error_message IS NULL) AS success_count,
    COUNT(*) FILTER (WHERE error_message IS NOT NULL) AS error_count,
    SUM(tokens_input) AS tokens_input,
    SUM(tokens_output) AS tokens_output,
    SUM(cost) AS total_cost,
    AVG(duration_ms) AS avg_duration_ms,
    COUNT(DISTINCT user_id) AS active_users,
    COUNT(DISTINCT session_id) AS active_sessions
FROM messages
WHERE created_at >= CURRENT_DATE;

-- ============================================
-- 完成
-- ============================================
COMMENT ON DATABASE codeswitch IS 'CodeSwitch 多端同步数据库';
