package admin

import (
	"runtime"
	"sync"
	"time"
)

// MonitorService 系统监控服务
type MonitorService struct {
	mu        sync.RWMutex
	startTime time.Time

	// 连接统计
	natsConnected   bool
	natsLastError   string
	natsReconnects  int64

	// 请求统计
	activeRequests  int64
	peakRequests    int64
	peakRequestTime time.Time

	// 健康检查
	healthChecks map[string]*HealthCheck
}

// HealthCheck 健康检查结果
type HealthCheck struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // healthy, degraded, unhealthy
	Message   string    `json:"message,omitempty"`
	LastCheck time.Time `json:"last_check"`
	Latency   int64     `json:"latency_ms"`
}

// SystemStatus 系统状态
type SystemStatus struct {
	Status       string                  `json:"status"` // healthy, degraded, unhealthy
	Uptime       int64                   `json:"uptime_seconds"`
	UptimeHuman  string                  `json:"uptime_human"`
	StartTime    time.Time               `json:"start_time"`
	Version      string                  `json:"version"`
	GoVersion    string                  `json:"go_version"`
	NumGoroutine int                     `json:"num_goroutine"`
	NumCPU       int                     `json:"num_cpu"`
	Memory       *MemoryStats            `json:"memory"`
	NATS         *NATSStatus             `json:"nats"`
	Requests     *RequestStats           `json:"requests"`
	Components   map[string]*HealthCheck `json:"components"`
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	NumGC      uint32 `json:"num_gc"`
	HeapAlloc  uint64 `json:"heap_alloc_bytes"`
	HeapSys    uint64 `json:"heap_sys_bytes"`
}

// NATSStatus NATS 状态
type NATSStatus struct {
	Connected  bool   `json:"connected"`
	LastError  string `json:"last_error,omitempty"`
	Reconnects int64  `json:"reconnects"`
}

// RequestStats 请求统计
type RequestStats struct {
	Active   int64     `json:"active"`
	Peak     int64     `json:"peak"`
	PeakTime time.Time `json:"peak_time,omitempty"`
}

// NewMonitorService 创建监控服务
func NewMonitorService() *MonitorService {
	return &MonitorService{
		startTime:    time.Now(),
		healthChecks: make(map[string]*HealthCheck),
	}
}

// SetNATSStatus 设置 NATS 状态
func (m *MonitorService) SetNATSStatus(connected bool, lastError string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.natsConnected = connected
	m.natsLastError = lastError
}

// IncrementNATSReconnects 增加 NATS 重连次数
func (m *MonitorService) IncrementNATSReconnects() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.natsReconnects++
}

// IncrementActiveRequests 增加活跃请求数
func (m *MonitorService) IncrementActiveRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeRequests++
	if m.activeRequests > m.peakRequests {
		m.peakRequests = m.activeRequests
		m.peakRequestTime = time.Now()
	}
}

// DecrementActiveRequests 减少活跃请求数
func (m *MonitorService) DecrementActiveRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeRequests > 0 {
		m.activeRequests--
	}
}

// UpdateHealthCheck 更新健康检查结果
func (m *MonitorService) UpdateHealthCheck(name, status, message string, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthChecks[name] = &HealthCheck{
		Name:      name,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
		Latency:   latencyMs,
	}
}

// GetSystemStatus 获取系统状态
func (m *MonitorService) GetSystemStatus(version string) *SystemStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(m.startTime)

	// 计算整体状态
	overallStatus := "healthy"
	for _, check := range m.healthChecks {
		if check.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if check.Status == "degraded" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}
	if !m.natsConnected {
		overallStatus = "degraded"
	}

	return &SystemStatus{
		Status:       overallStatus,
		Uptime:       int64(uptime.Seconds()),
		UptimeHuman:  formatDuration(uptime),
		StartTime:    m.startTime,
		Version:      version,
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		Memory: &MemoryStats{
			Alloc:      memStats.Alloc,
			TotalAlloc: memStats.TotalAlloc,
			Sys:        memStats.Sys,
			NumGC:      memStats.NumGC,
			HeapAlloc:  memStats.HeapAlloc,
			HeapSys:    memStats.HeapSys,
		},
		NATS: &NATSStatus{
			Connected:  m.natsConnected,
			LastError:  m.natsLastError,
			Reconnects: m.natsReconnects,
		},
		Requests: &RequestStats{
			Active:   m.activeRequests,
			Peak:     m.peakRequests,
			PeakTime: m.peakRequestTime,
		},
		Components: m.copyHealthChecks(),
	}
}

func (m *MonitorService) copyHealthChecks() map[string]*HealthCheck {
	result := make(map[string]*HealthCheck, len(m.healthChecks))
	for k, v := range m.healthChecks {
		result[k] = v
	}
	return result
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return formatPlural(days, "day") + " " + formatPlural(hours, "hour")
	}
	if hours > 0 {
		return formatPlural(hours, "hour") + " " + formatPlural(minutes, "minute")
	}
	if minutes > 0 {
		return formatPlural(minutes, "minute") + " " + formatPlural(seconds, "second")
	}
	return formatPlural(seconds, "second")
}

func formatPlural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10)) + " " + unit + "s"
}
