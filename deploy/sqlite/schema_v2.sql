-- ============================================================
-- CodeSwitch v2.0 SSOT Schema
-- Single Source of Truth (SSOT) Architecture
--
-- 功能说明：
-- 1. provider_config: Provider 配置（替代分散的 JSON 文件）
-- 2. provider_health: Provider 健康状态（熔断器支持）
-- 3. proxy_control: 代理控制（按应用独立开关）
-- 4. proxy_live_backup: 配置热备份（异常恢复）
-- 5. schema_version: Schema 版本管理
--
-- 创建日期: 2026-01-14
-- 版本: 2.0
-- ============================================================

-- ============================================================
-- 1. Provider Configuration (替代 JSON 文件)
-- ============================================================
CREATE TABLE IF NOT EXISTS provider_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform TEXT NOT NULL,           -- 'claude', 'codex', 'gemini'
    name TEXT NOT NULL,
    api_url TEXT NOT NULL,
    api_key TEXT NOT NULL,            -- 加密存储
    official_site TEXT,
    icon TEXT,

    -- 新增: 外观自定义 (从 cc-switch 借鉴)
    tint TEXT DEFAULT '#f0f0f0',      -- 背景色
    accent TEXT DEFAULT '#0a84ff',    -- 强调色

    enabled INTEGER DEFAULT 1,        -- 0=disabled, 1=enabled

    -- 模型配置 (JSON 格式)
    supported_models TEXT,            -- JSON: {"claude-*": true, ...}
    model_mapping TEXT,               -- JSON: {"claude-*": "anthropic/claude-*", ...}

    -- 优先级与负载均衡
    priority_level INTEGER DEFAULT 1, -- 1-10, 越小越优先
    weight INTEGER DEFAULT 100,       -- 权重 (用于加权 Round-Robin)

    -- 时间戳
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(platform, name)
);

-- 索引优化
CREATE INDEX IF NOT EXISTS idx_provider_platform ON provider_config(platform, enabled);
CREATE INDEX IF NOT EXISTS idx_provider_priority ON provider_config(priority_level, enabled);
CREATE INDEX IF NOT EXISTS idx_provider_updated ON provider_config(updated_at DESC);

-- ============================================================
-- 2. Provider Health (熔断器状态)
-- ============================================================
CREATE TABLE IF NOT EXISTS provider_health (
    provider_id INTEGER PRIMARY KEY REFERENCES provider_config(id) ON DELETE CASCADE,

    -- 熔断状态 (从 cc-switch 借鉴)
    circuit_state TEXT DEFAULT 'closed', -- 'closed', 'open', 'half_open'
    consecutive_fails INTEGER DEFAULT 0,
    fail_threshold INTEGER DEFAULT 5,    -- 连续失败阈值
    recovery_timeout_sec INTEGER DEFAULT 30, -- 恢复超时（秒）

    -- 统计信息
    total_requests INTEGER DEFAULT 0,
    total_failures INTEGER DEFAULT 0,
    success_rate REAL DEFAULT 1.0,       -- 成功率 (0-1)
    avg_latency_ms REAL DEFAULT 0,       -- 平均延迟

    -- 时间戳
    last_success_at DATETIME,
    last_failure_at DATETIME,
    circuit_opened_at DATETIME,
    last_checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引优化
CREATE INDEX IF NOT EXISTS idx_health_state ON provider_health(circuit_state);
CREATE INDEX IF NOT EXISTS idx_health_updated ON provider_health(updated_at DESC);

-- ============================================================
-- 3. Proxy Control (按应用代理控制)
-- ============================================================
CREATE TABLE IF NOT EXISTS proxy_control (
    app_name TEXT PRIMARY KEY,        -- 'claude', 'codex', 'gemini'
    proxy_enabled INTEGER DEFAULT 1,  -- 0=disabled, 1=enabled
    proxy_mode TEXT DEFAULT 'shared', -- 'shared' (18100), 'dedicated' (独立端口)
    proxy_port INTEGER,               -- dedicated 模式的端口
    intercept_domains TEXT,           -- JSON array: ["api.anthropic.com", ...]

    -- 统计
    total_requests INTEGER DEFAULT 0,
    last_request_at DATETIME,

    -- 时间戳
    last_toggled_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 初始化三个应用
INSERT OR IGNORE INTO proxy_control (app_name, proxy_enabled) VALUES
    ('claude', 1),
    ('codex', 1),
    ('gemini', 1),
    ('picoclaw', 1);

-- ============================================================
-- 4. Proxy Live Backup (配置热备份)
-- ============================================================
CREATE TABLE IF NOT EXISTS proxy_live_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backup_type TEXT NOT NULL,        -- 'provider_config', 'app_settings', 'mcp_config'
    backup_data TEXT NOT NULL,        -- JSON snapshot
    trigger_event TEXT,               -- 'manual', 'auto', 'pre_update', 'crash_recovery'
    backup_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    restored INTEGER DEFAULT 0,       -- 是否已恢复
    restored_at DATETIME
);

-- 索引优化
CREATE INDEX IF NOT EXISTS idx_backup_type_time ON proxy_live_backup(backup_type, backup_time DESC);
CREATE INDEX IF NOT EXISTS idx_backup_restored ON proxy_live_backup(restored, backup_time DESC);

-- ============================================================
-- 5. Schema Version (数据库版本管理)
-- ============================================================
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    description TEXT,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 插入初始版本记录（如果不存在）
INSERT OR IGNORE INTO schema_version (version, description) VALUES
    (1, 'Initial schema'),
    (2, 'SSOT architecture migration - Phase 1');

-- ============================================================
-- 6. Triggers (自动备份触发器)
-- ============================================================

-- 触发器: Provider 配置更新时自动备份
DROP TRIGGER IF EXISTS backup_provider_on_update;
CREATE TRIGGER backup_provider_on_update
AFTER UPDATE ON provider_config
BEGIN
    INSERT INTO proxy_live_backup (backup_type, backup_data, trigger_event)
    VALUES (
        'provider_config',
        json_object(
            'platform', NEW.platform,
            'provider_id', NEW.id,
            'snapshot', json_object(
                'id', NEW.id,
                'name', NEW.name,
                'api_url', NEW.api_url,
                'api_key', NEW.api_key,
                'enabled', NEW.enabled,
                'supported_models', NEW.supported_models,
                'model_mapping', NEW.model_mapping,
                'priority_level', NEW.priority_level,
                'tint', NEW.tint,
                'accent', NEW.accent,
                'updated_at', CURRENT_TIMESTAMP
            )
        ),
        'auto_update'
    );
END;

-- 触发器: Provider 配置插入时自动备份
DROP TRIGGER IF EXISTS backup_provider_on_insert;
CREATE TRIGGER backup_provider_on_insert
AFTER INSERT ON provider_config
BEGIN
    INSERT INTO proxy_live_backup (backup_type, backup_data, trigger_event)
    VALUES (
        'provider_config',
        json_object(
            'platform', NEW.platform,
            'provider_id', NEW.id,
            'snapshot', json_object(
                'id', NEW.id,
                'name', NEW.name,
                'api_url', NEW.api_url,
                'enabled', NEW.enabled,
                'created_at', CURRENT_TIMESTAMP
            )
        ),
        'auto_insert'
    );
END;

-- 触发器: Provider 配置删除时自动备份
DROP TRIGGER IF EXISTS backup_provider_on_delete;
CREATE TRIGGER backup_provider_on_delete
AFTER DELETE ON provider_config
BEGIN
    INSERT INTO proxy_live_backup (backup_type, backup_data, trigger_event)
    VALUES (
        'provider_config',
        json_object(
            'platform', OLD.platform,
            'provider_id', OLD.id,
            'snapshot', json_object(
                'id', OLD.id,
                'name', OLD.name,
                'api_url', OLD.api_url,
                'deleted_at', CURRENT_TIMESTAMP
            )
        ),
        'auto_delete'
    );
END;

-- ============================================================
-- 7. Views (便捷查询)
-- ============================================================

-- 视图: Provider 状态总览
DROP VIEW IF EXISTS provider_status;
CREATE VIEW provider_status AS
SELECT
    p.id,
    p.platform,
    p.name,
    p.api_url,
    p.enabled,
    p.priority_level,
    p.tint,
    p.accent,
    COALESCE(h.circuit_state, 'closed') AS circuit_state,
    COALESCE(h.consecutive_fails, 0) AS consecutive_fails,
    COALESCE(h.success_rate, 1.0) AS success_rate,
    COALESCE(h.avg_latency_ms, 0) AS avg_latency_ms,
    h.last_success_at,
    h.last_failure_at,
    h.circuit_opened_at,
    p.created_at,
    p.updated_at
FROM provider_config p
LEFT JOIN provider_health h ON p.id = h.provider_id;

-- 视图: 可用 Provider（启用且熔断器未打开）
DROP VIEW IF EXISTS available_providers;
CREATE VIEW available_providers AS
SELECT
    p.id,
    p.platform,
    p.name,
    p.api_url,
    p.priority_level,
    p.supported_models,
    p.model_mapping
FROM provider_config p
LEFT JOIN provider_health h ON p.id = h.provider_id
WHERE p.enabled = 1
  AND (h.circuit_state IS NULL OR h.circuit_state != 'open')
ORDER BY p.priority_level ASC, p.id ASC;

-- ============================================================
-- 8. Utility Functions (SQLite JSON 辅助函数示例)
-- ============================================================
-- 注意: SQLite 的 JSON 函数在查询时使用，无需创建
-- 示例查询：
-- SELECT json_extract(supported_models, '$.claude-*') FROM provider_config;
-- SELECT json_array_length(intercept_domains) FROM proxy_control;

-- ============================================================
-- 9. Data Integrity Constraints
-- ============================================================

-- 确保 circuit_state 只能是有效值
-- 注意: SQLite 不支持 CHECK 约束中的枚举，需在应用层验证

-- ============================================================
-- End of Schema v2.0
-- ============================================================
