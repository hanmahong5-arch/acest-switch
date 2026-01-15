package admin

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// MetricsExporter Prometheus 格式指标导出器
type MetricsExporter struct {
	statsService   *StatsService
	monitorService *MonitorService
	userManager    *UserManager
	startTime      time.Time
	version        string
}

// NewMetricsExporter 创建指标导出器
func NewMetricsExporter(
	statsService *StatsService,
	monitorService *MonitorService,
	userManager *UserManager,
	version string,
) *MetricsExporter {
	return &MetricsExporter{
		statsService:   statsService,
		monitorService: monitorService,
		userManager:    userManager,
		startTime:      time.Now(),
		version:        version,
	}
}

// Export 导出 Prometheus 格式指标
func (m *MetricsExporter) Export() string {
	var sb strings.Builder

	// 服务信息
	sb.WriteString("# HELP codeswitch_sync_info Sync service information\n")
	sb.WriteString("# TYPE codeswitch_sync_info gauge\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_info{version=\"%s\",go_version=\"%s\"} 1\n",
		m.version, runtime.Version()))

	// 运行时间
	sb.WriteString("# HELP codeswitch_sync_uptime_seconds Sync service uptime in seconds\n")
	sb.WriteString("# TYPE codeswitch_sync_uptime_seconds counter\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_uptime_seconds %.0f\n", time.Since(m.startTime).Seconds()))

	// 内存使用
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	sb.WriteString("# HELP codeswitch_sync_memory_alloc_bytes Current memory allocation in bytes\n")
	sb.WriteString("# TYPE codeswitch_sync_memory_alloc_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_memory_alloc_bytes %d\n", memStats.Alloc))

	sb.WriteString("# HELP codeswitch_sync_memory_sys_bytes Total memory obtained from system\n")
	sb.WriteString("# TYPE codeswitch_sync_memory_sys_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_memory_sys_bytes %d\n", memStats.Sys))

	sb.WriteString("# HELP codeswitch_sync_gc_total Total number of GC cycles\n")
	sb.WriteString("# TYPE codeswitch_sync_gc_total counter\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_gc_total %d\n", memStats.NumGC))

	// Goroutines
	sb.WriteString("# HELP codeswitch_sync_goroutines Current number of goroutines\n")
	sb.WriteString("# TYPE codeswitch_sync_goroutines gauge\n")
	sb.WriteString(fmt.Sprintf("codeswitch_sync_goroutines %d\n", runtime.NumGoroutine()))

	// 请求统计
	if m.statsService != nil {
		overview := m.statsService.GetOverview()

		sb.WriteString("# HELP codeswitch_sync_requests_total Total number of LLM requests\n")
		sb.WriteString("# TYPE codeswitch_sync_requests_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_requests_total %v\n", overview["total_requests"]))

		sb.WriteString("# HELP codeswitch_sync_tokens_in_total Total input tokens\n")
		sb.WriteString("# TYPE codeswitch_sync_tokens_in_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_tokens_in_total %v\n", overview["total_tokens_in"]))

		sb.WriteString("# HELP codeswitch_sync_tokens_out_total Total output tokens\n")
		sb.WriteString("# TYPE codeswitch_sync_tokens_out_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_tokens_out_total %v\n", overview["total_tokens_out"]))

		sb.WriteString("# HELP codeswitch_sync_cost_total Total cost in USD\n")
		sb.WriteString("# TYPE codeswitch_sync_cost_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_cost_total %v\n", overview["total_cost"]))

		sb.WriteString("# HELP codeswitch_sync_errors_total Total number of errors\n")
		sb.WriteString("# TYPE codeswitch_sync_errors_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_errors_total %v\n", overview["total_errors"]))

		sb.WriteString("# HELP codeswitch_sync_success_rate Success rate percentage\n")
		sb.WriteString("# TYPE codeswitch_sync_success_rate gauge\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_success_rate %v\n", overview["success_rate"]))

		// 按供应商统计
		sb.WriteString("# HELP codeswitch_sync_provider_requests_total Requests by provider\n")
		sb.WriteString("# TYPE codeswitch_sync_provider_requests_total counter\n")
		for _, ps := range m.statsService.GetProviderStats() {
			sb.WriteString(fmt.Sprintf("codeswitch_sync_provider_requests_total{provider=\"%s\"} %d\n",
				ps.Provider, ps.Requests))
		}

		sb.WriteString("# HELP codeswitch_sync_provider_cost_total Cost by provider\n")
		sb.WriteString("# TYPE codeswitch_sync_provider_cost_total counter\n")
		for _, ps := range m.statsService.GetProviderStats() {
			sb.WriteString(fmt.Sprintf("codeswitch_sync_provider_cost_total{provider=\"%s\"} %f\n",
				ps.Provider, ps.Cost))
		}

		sb.WriteString("# HELP codeswitch_sync_provider_latency_avg_ms Average latency by provider\n")
		sb.WriteString("# TYPE codeswitch_sync_provider_latency_avg_ms gauge\n")
		for _, ps := range m.statsService.GetProviderStats() {
			sb.WriteString(fmt.Sprintf("codeswitch_sync_provider_latency_avg_ms{provider=\"%s\"} %f\n",
				ps.Provider, ps.AvgLatencyMs))
		}

		// 按模型统计
		sb.WriteString("# HELP codeswitch_sync_model_requests_total Requests by model\n")
		sb.WriteString("# TYPE codeswitch_sync_model_requests_total counter\n")
		for _, ms := range m.statsService.GetModelStats() {
			sb.WriteString(fmt.Sprintf("codeswitch_sync_model_requests_total{provider=\"%s\",model=\"%s\"} %d\n",
				ms.Provider, ms.Model, ms.Requests))
		}
	}

	// 用户统计
	if m.userManager != nil {
		sb.WriteString("# HELP codeswitch_sync_users_online Current online users\n")
		sb.WriteString("# TYPE codeswitch_sync_users_online gauge\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_users_online %d\n", m.userManager.GetOnlineUsersCount()))

		sb.WriteString("# HELP codeswitch_sync_users_active Active users in last 24h\n")
		sb.WriteString("# TYPE codeswitch_sync_users_active gauge\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_users_active %d\n", m.userManager.GetActiveUsersCount()))
	}

	// NATS 状态
	if m.monitorService != nil {
		status := m.monitorService.GetSystemStatus(m.version)

		sb.WriteString("# HELP codeswitch_sync_nats_connected NATS connection status\n")
		sb.WriteString("# TYPE codeswitch_sync_nats_connected gauge\n")
		connected := 0
		if status.NATS.Connected {
			connected = 1
		}
		sb.WriteString(fmt.Sprintf("codeswitch_sync_nats_connected %d\n", connected))

		sb.WriteString("# HELP codeswitch_sync_nats_reconnects_total NATS reconnection count\n")
		sb.WriteString("# TYPE codeswitch_sync_nats_reconnects_total counter\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_nats_reconnects_total %d\n", status.NATS.Reconnects))

		sb.WriteString("# HELP codeswitch_sync_requests_active Current active requests\n")
		sb.WriteString("# TYPE codeswitch_sync_requests_active gauge\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_requests_active %d\n", status.Requests.Active))

		sb.WriteString("# HELP codeswitch_sync_requests_peak Peak concurrent requests\n")
		sb.WriteString("# TYPE codeswitch_sync_requests_peak gauge\n")
		sb.WriteString(fmt.Sprintf("codeswitch_sync_requests_peak %d\n", status.Requests.Peak))
	}

	return sb.String()
}
