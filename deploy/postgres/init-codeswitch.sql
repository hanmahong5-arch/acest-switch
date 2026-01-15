-- ============================================================
-- CodeSwitch Extended Tables
-- Tables for features not covered by Casdoor/Lago
-- ============================================================

-- ============================================================
-- User Extensions (links Casdoor user to Lago customer)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_user_extensions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    casdoor_user_id VARCHAR(64) NOT NULL UNIQUE,  -- Casdoor user ID
    lago_customer_id VARCHAR(64) UNIQUE,           -- Lago customer ID
    newapi_user_id INT,                            -- NEW-API user ID (for sync)

    -- Invite system
    invite_code VARCHAR(16) UNIQUE,
    invited_by UUID REFERENCES cs_user_extensions(id),

    -- VIP features not in Lago
    vip_features JSONB DEFAULT '{}',

    -- Preferences
    preferences JSONB DEFAULT '{}',

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cs_user_ext_casdoor ON cs_user_extensions(casdoor_user_id);
CREATE INDEX IF NOT EXISTS idx_cs_user_ext_lago ON cs_user_extensions(lago_customer_id);
CREATE INDEX IF NOT EXISTS idx_cs_user_ext_invite ON cs_user_extensions(invite_code);

-- ============================================================
-- Checkin Records (签到记录 - Lago doesn't support this)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_checkin_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES cs_user_extensions(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    streak_days INT NOT NULL DEFAULT 1,        -- Consecutive days
    credits_earned BIGINT NOT NULL,            -- Credits rewarded
    is_vip BOOLEAN DEFAULT FALSE,              -- Was VIP at checkin time
    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS idx_cs_checkin_user ON cs_checkin_records(user_id, checkin_date DESC);

-- ============================================================
-- Invite Relations (邀请关系 - Lago doesn't support this)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_invite_relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inviter_id UUID NOT NULL REFERENCES cs_user_extensions(id) ON DELETE CASCADE,
    invitee_id UUID NOT NULL REFERENCES cs_user_extensions(id) ON DELETE CASCADE,

    -- Invite tracking
    status VARCHAR(16) NOT NULL DEFAULT 'pending', -- 'pending', 'activated', 'rewarded'

    -- Rewards
    inviter_reward_credits BIGINT DEFAULT 0,  -- Credits given to inviter
    invitee_reward_credits BIGINT DEFAULT 0,  -- Credits given to invitee

    -- First recharge bonus tracking
    invitee_first_recharge BIGINT DEFAULT 0,  -- Invitee's first recharge amount
    inviter_bonus_paid BOOLEAN DEFAULT FALSE, -- Whether 10% bonus was paid

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    activated_at TIMESTAMPTZ,                  -- When invitee registered
    rewarded_at TIMESTAMPTZ,                   -- When bonus was paid

    UNIQUE(invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_cs_invite_inviter ON cs_invite_relations(inviter_id);
CREATE INDEX IF NOT EXISTS idx_cs_invite_status ON cs_invite_relations(status);

-- ============================================================
-- Device Management (设备管理 - Extended from Casdoor)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_user_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES cs_user_extensions(id) ON DELETE CASCADE,

    -- Device info
    device_id VARCHAR(128) NOT NULL,           -- Unique device identifier
    device_name VARCHAR(128),                   -- User-friendly name
    device_type VARCHAR(32) NOT NULL,          -- 'desktop', 'mobile', 'cli', 'web'
    client_name VARCHAR(64),                    -- 'codeswitch', 'claude-code', 'codex', 'gemini-cli'
    client_version VARCHAR(32),
    os_name VARCHAR(32),
    os_version VARCHAR(32),

    -- Network
    ip_address INET,
    user_agent TEXT,

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    last_active_at TIMESTAMPTZ DEFAULT NOW(),

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_cs_devices_user ON cs_user_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_cs_devices_active ON cs_user_devices(last_active_at DESC) WHERE is_active = TRUE;

-- ============================================================
-- Activity Rewards (活动奖励记录)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_activity_rewards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES cs_user_extensions(id) ON DELETE CASCADE,

    -- Activity info
    activity_code VARCHAR(64) NOT NULL,        -- Activity identifier
    activity_name VARCHAR(128),

    -- Reward
    credits_earned BIGINT NOT NULL,

    -- Metadata
    metadata JSONB DEFAULT '{}',

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Prevent duplicate claims
    UNIQUE(user_id, activity_code)
);

CREATE INDEX IF NOT EXISTS idx_cs_activity_user ON cs_activity_rewards(user_id);

-- ============================================================
-- Lago Webhook Events (Webhook 事件日志)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_lago_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Webhook data
    event_type VARCHAR(64) NOT NULL,           -- e.g., 'invoice.created', 'subscription.started'
    lago_id VARCHAR(64),                        -- Lago resource ID
    payload JSONB NOT NULL,

    -- Processing
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cs_webhook_type ON cs_lago_webhooks(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cs_webhook_unprocessed ON cs_lago_webhooks(created_at) WHERE processed = FALSE;

-- ============================================================
-- System Configuration (系统配置)
-- ============================================================
CREATE TABLE IF NOT EXISTS cs_system_config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default configurations
INSERT INTO cs_system_config (key, value, description) VALUES
    ('checkin_rewards', '{"base": 100, "streak_3": 20, "streak_7": 50, "streak_14": 100, "streak_30": 200, "vip_multiplier": 2}', 'Checkin reward configuration'),
    ('invite_rewards', '{"inviter_base": 500, "invitee_base": 200, "first_recharge_bonus_percent": 10}', 'Invite reward configuration'),
    ('rate_limits', '{"free": 10, "monthly_vip": 60, "yearly_vip": 120, "enterprise": 0}', 'Rate limits per plan (0 = unlimited)')
ON CONFLICT (key) DO NOTHING;

-- ============================================================
-- Functions
-- ============================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to tables with updated_at
CREATE TRIGGER update_cs_user_extensions_updated_at
    BEFORE UPDATE ON cs_user_extensions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- Views
-- ============================================================

-- User dashboard view (combines extension data)
CREATE OR REPLACE VIEW cs_user_dashboard AS
SELECT
    u.id,
    u.casdoor_user_id,
    u.lago_customer_id,
    u.invite_code,
    u.created_at,
    COALESCE(c.total_checkins, 0) AS total_checkins,
    COALESCE(c.current_streak, 0) AS current_streak,
    COALESCE(c.total_checkin_credits, 0) AS total_checkin_credits,
    COALESCE(i.invite_count, 0) AS invite_count,
    COALESCE(i.total_invite_credits, 0) AS total_invite_credits,
    COALESCE(d.device_count, 0) AS device_count
FROM cs_user_extensions u
LEFT JOIN (
    SELECT
        user_id,
        COUNT(*) AS total_checkins,
        MAX(streak_days) AS current_streak,
        SUM(credits_earned) AS total_checkin_credits
    FROM cs_checkin_records
    GROUP BY user_id
) c ON c.user_id = u.id
LEFT JOIN (
    SELECT
        inviter_id,
        COUNT(*) AS invite_count,
        SUM(inviter_reward_credits) AS total_invite_credits
    FROM cs_invite_relations
    WHERE status = 'rewarded'
    GROUP BY inviter_id
) i ON i.inviter_id = u.id
LEFT JOIN (
    SELECT
        user_id,
        COUNT(*) AS device_count
    FROM cs_user_devices
    WHERE is_active = TRUE
    GROUP BY user_id
) d ON d.user_id = u.id;

-- ============================================================
-- Comments
-- ============================================================
COMMENT ON TABLE cs_user_extensions IS 'Links Casdoor users to Lago customers with additional features';
COMMENT ON TABLE cs_checkin_records IS 'Daily checkin records for bonus credits';
COMMENT ON TABLE cs_invite_relations IS 'User invitation relationships and rewards';
COMMENT ON TABLE cs_user_devices IS 'User device management';
COMMENT ON TABLE cs_activity_rewards IS 'Special activity reward records';
COMMENT ON TABLE cs_lago_webhooks IS 'Lago webhook event log';
COMMENT ON TABLE cs_system_config IS 'System configuration key-value store';
