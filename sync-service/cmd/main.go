package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/admin"
	"github.com/aspect-code/codeswitch/sync-service/internal/api"
	"github.com/aspect-code/codeswitch/sync-service/internal/auth"
	"github.com/aspect-code/codeswitch/sync-service/internal/message"
	"github.com/aspect-code/codeswitch/sync-service/internal/nats"
	"github.com/aspect-code/codeswitch/sync-service/internal/presence"
	"github.com/aspect-code/codeswitch/sync-service/internal/session"
	"gopkg.in/yaml.v3"
)

// 版本号
const Version = "1.0.0"

// Config 应用配置
type Config struct {
	Server struct {
		Port string `yaml:"port"`
		Mode string `yaml:"mode"`
	} `yaml:"server"`
	NATS    nats.Config    `yaml:"nats"`
	NewAPI  auth.NewAPIConfig  `yaml:"newapi"`
	JWT     auth.JWTConfig     `yaml:"jwt"`
	Presence presence.Config  `yaml:"presence"`
}

func main() {
	// 初始化日志
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting CodeSwitch Sync Service")

	// 加载配置
	cfg, err := loadConfig("configs/config.yaml")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// 创建 NATS 客户端
	natsClient := nats.NewClient(&cfg.NATS, logger)
	ctx := context.Background()

	if err := natsClient.Connect(ctx); err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// 创建服务
	authService := auth.NewService(&cfg.JWT, &cfg.NewAPI, logger)
	sessionManager := session.NewManager(natsClient, logger)
	messageHandler := message.NewHandler(natsClient, logger)
	presenceTracker := presence.NewTracker(natsClient, &cfg.Presence, logger)

	// 创建管理后台服务
	statsService := admin.NewStatsService()
	monitorService := admin.NewMonitorService()
	userManager := admin.NewUserManager()
	auditService := admin.NewAuditService()
	alertService := admin.NewAlertService(statsService)
	billingService := admin.NewBillingService("")

	// 设置 NATS 连接状态
	monitorService.SetNATSStatus(true, "")

	logger.Info("Admin services initialized",
		"stats", "enabled",
		"monitor", "enabled",
		"userManager", "enabled",
		"audit", "enabled",
		"alerts", "enabled",
		"billing", "enabled",
	)

	// 启动 API 服务器
	server := api.NewServer(
		authService,
		sessionManager,
		messageHandler,
		presenceTracker,
		statsService,
		monitorService,
		userManager,
		auditService,
		alertService,
		billingService,
		logger,
		cfg.Server.Mode,
		Version,
	)

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutting down...")
		natsClient.Close()
		os.Exit(0)
	}()

	// 启动服务器
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	if err := server.Run(addr); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// 使用默认配置
		return defaultConfig(), nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 设置默认值
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8081"
	}
	if cfg.NATS.URL == "" {
		cfg.NATS.URL = "nats://localhost:4222"
	}
	if cfg.NATS.ReconnectWait == 0 {
		cfg.NATS.ReconnectWait = 2 * time.Second
	}
	if cfg.NATS.MaxReconnects == 0 {
		cfg.NATS.MaxReconnects = -1
	}
	if cfg.Presence.HeartbeatInterval == 0 {
		cfg.Presence.HeartbeatInterval = 30 * time.Second
	}
	if cfg.Presence.Timeout == 0 {
		cfg.Presence.Timeout = 90 * time.Second
	}

	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: struct {
			Port string `yaml:"port"`
			Mode string `yaml:"mode"`
		}{
			Port: "8081",
			Mode: "debug",
		},
		NATS: nats.Config{
			URL:                 "nats://localhost:4222",
			ReconnectWait:       2 * time.Second,
			MaxReconnects:       -1,
			ReconnectBufferSize: 8 * 1024 * 1024,
		},
		JWT: auth.JWTConfig{
			Secret:          "codeswitch-sync-service-jwt-secret-key",
			Issuer:          "codeswitch",
			AccessTokenTTL:  24 * time.Hour,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		},
		Presence: presence.Config{
			HeartbeatInterval: 30 * time.Second,
			Timeout:           90 * time.Second,
		},
	}
}
