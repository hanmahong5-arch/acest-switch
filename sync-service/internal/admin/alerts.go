package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AlertRule represents an alert rule
type AlertRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Metric         string   `json:"metric"` // error_rate, latency_p99, latency_avg, cost_daily, requests_per_minute
	Condition      string   `json:"condition"` // gt, lt, eq, gte, lte
	Threshold      float64  `json:"threshold"`
	WindowSeconds  int      `json:"window_seconds"` // Time window for evaluation
	Severity       string   `json:"severity"` // info, warning, critical
	Enabled        bool     `json:"enabled"`
	NotifyChannels []string `json:"notify_channels"` // webhook, email, etc.
	WebhookURL     string   `json:"webhook_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AlertHistory represents an alert event
type AlertHistory struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	MetricValue float64   `json:"metric_value"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"` // firing, resolved
	TriggeredAt time.Time `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AlertRuleListResponse represents paginated alert rules response
type AlertRuleListResponse struct {
	Rules []AlertRule `json:"rules"`
	Total int         `json:"total"`
}

// AlertHistoryListResponse represents paginated alert history response
type AlertHistoryListResponse struct {
	History []AlertHistory `json:"history"`
	Total   int            `json:"total"`
}

// AlertService handles alert rules and notifications
type AlertService struct {
	mu           sync.RWMutex
	rules        map[string]*AlertRule
	history      []AlertHistory
	statsService *StatsService
	stopCh       chan struct{}
	// Deduplication: track last fire time per rule to avoid alert flooding
	lastFired    map[string]time.Time
	// Default silence period: 5 minutes between same rule alerts
	silencePeriod time.Duration
}

// NewAlertService creates a new alert service
func NewAlertService(statsService *StatsService) *AlertService {
	s := &AlertService{
		rules:         make(map[string]*AlertRule),
		history:       make([]AlertHistory, 0),
		statsService:  statsService,
		stopCh:        make(chan struct{}),
		lastFired:     make(map[string]time.Time),
		silencePeriod: 5 * time.Minute, // Default 5 minutes silence between same alerts
	}

	// Start the evaluation loop
	go s.evaluationLoop()

	return s
}

// Stop stops the alert service
func (s *AlertService) Stop() {
	close(s.stopCh)
}

// SetSilencePeriod sets the default silence period for alert deduplication
func (s *AlertService) SetSilencePeriod(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.silencePeriod = d
}

// ClearFiredHistory clears the deduplication cache for a specific rule or all rules
func (s *AlertService) ClearFiredHistory(ruleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ruleID == "" {
		s.lastFired = make(map[string]time.Time)
	} else {
		delete(s.lastFired, ruleID)
	}
}

// GetFiringStatus returns the current firing status of all rules
func (s *AlertService) GetFiringStatus() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]time.Time)
	for k, v := range s.lastFired {
		result[k] = v
	}
	return result
}

// CreateRule creates a new alert rule
func (s *AlertService) CreateRule(rule AlertRule) (*AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateAlertID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	s.rules[rule.ID] = &rule
	return &rule, nil
}

// UpdateRule updates an existing alert rule
func (s *AlertService) UpdateRule(id string, updates AlertRule) (*AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	// Update fields
	if updates.Name != "" {
		rule.Name = updates.Name
	}
	rule.Description = updates.Description
	if updates.Metric != "" {
		rule.Metric = updates.Metric
	}
	if updates.Condition != "" {
		rule.Condition = updates.Condition
	}
	rule.Threshold = updates.Threshold
	if updates.WindowSeconds > 0 {
		rule.WindowSeconds = updates.WindowSeconds
	}
	if updates.Severity != "" {
		rule.Severity = updates.Severity
	}
	rule.Enabled = updates.Enabled
	rule.NotifyChannels = updates.NotifyChannels
	rule.WebhookURL = updates.WebhookURL
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// DeleteRule deletes an alert rule
func (s *AlertService) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(s.rules, id)
	return nil
}

// GetRule gets a single rule by ID
func (s *AlertService) GetRule(id string) *AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if rule, ok := s.rules[id]; ok {
		return rule
	}
	return nil
}

// ListRules returns all alert rules
func (s *AlertService) ListRules() AlertRuleListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules := make([]AlertRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, *rule)
	}

	return AlertRuleListResponse{
		Rules: rules,
		Total: len(rules),
	}
}

// ListHistory returns alert history
func (s *AlertService) ListHistory(severity, status string, limit int) AlertHistoryListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var filtered []AlertHistory
	for _, h := range s.history {
		if severity != "" && h.Severity != severity {
			continue
		}
		if status != "" && h.Status != status {
			continue
		}
		filtered = append(filtered, h)
		if len(filtered) >= limit {
			break
		}
	}

	return AlertHistoryListResponse{
		History: filtered,
		Total:   len(s.history),
	}
}

// evaluationLoop periodically evaluates all enabled rules
func (s *AlertService) evaluationLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.evaluateRules()
		}
	}
}

// evaluateRules evaluates all enabled rules
func (s *AlertService) evaluateRules() {
	s.mu.RLock()
	rules := make([]*AlertRule, 0)
	for _, rule := range s.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	s.mu.RUnlock()

	for _, rule := range rules {
		s.evaluateRule(rule)
	}
}

// evaluateRule evaluates a single rule
func (s *AlertService) evaluateRule(rule *AlertRule) {
	value := s.getMetricValue(rule.Metric)
	triggered := s.checkCondition(rule.Condition, value, rule.Threshold)

	if triggered {
		s.fireAlert(rule, value)
	}
}

// getMetricValue gets the current value of a metric
func (s *AlertService) getMetricValue(metric string) float64 {
	if s.statsService == nil {
		return 0
	}

	overview := s.statsService.GetOverview()

	switch metric {
	case "error_rate":
		successRate, ok := overview["success_rate"].(float64)
		if ok {
			return 100 - successRate // Convert to error rate
		}
	case "requests_per_minute":
		// Calculate from hourly stats
		hourly := s.statsService.GetHourlyStats()
		if len(hourly) > 0 {
			latest := hourly[len(hourly)-1]
			return float64(latest.Requests) / 60.0
		}
	case "cost_daily":
		daily := s.statsService.GetDailyStats()
		if len(daily) > 0 {
			latest := daily[len(daily)-1]
			return latest.Cost
		}
	case "latency_avg":
		hourly := s.statsService.GetHourlyStats()
		if len(hourly) > 0 {
			latest := hourly[len(hourly)-1]
			return latest.AvgLatencyMs
		}
	case "latency_p99":
		// For simplicity, use avg * 2 as p99 estimate
		hourly := s.statsService.GetHourlyStats()
		if len(hourly) > 0 {
			latest := hourly[len(hourly)-1]
			return latest.AvgLatencyMs * 2
		}
	}

	return 0
}

// checkCondition checks if the condition is met
func (s *AlertService) checkCondition(condition string, value, threshold float64) bool {
	switch condition {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

// fireAlert fires an alert with deduplication
func (s *AlertService) fireAlert(rule *AlertRule, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplication: check if we've fired this rule recently
	if lastTime, exists := s.lastFired[rule.ID]; exists {
		if time.Since(lastTime) < s.silencePeriod {
			// Still in silence period, skip this alert
			return
		}
	}

	// Update last fired time
	s.lastFired[rule.ID] = time.Now()

	// Create alert history entry
	alert := AlertHistory{
		ID:          generateAlertID(),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		MetricValue: value,
		Threshold:   rule.Threshold,
		Severity:    rule.Severity,
		Status:      "firing",
		TriggeredAt: time.Now(),
	}

	// Prepend to history (most recent first)
	s.history = append([]AlertHistory{alert}, s.history...)

	// Keep only last 1000 entries
	if len(s.history) > 1000 {
		s.history = s.history[:1000]
	}

	// Send notifications
	go s.sendNotifications(rule, alert)
}

// sendNotifications sends notifications for an alert
func (s *AlertService) sendNotifications(rule *AlertRule, alert AlertHistory) {
	for _, channel := range rule.NotifyChannels {
		switch channel {
		case "webhook":
			if rule.WebhookURL != "" {
				s.sendWebhook(rule.WebhookURL, alert)
			}
		}
	}
}

// sendWebhook sends a webhook notification
func (s *AlertService) sendWebhook(url string, alert AlertHistory) {
	payload := map[string]interface{}{
		"alert_id":     alert.ID,
		"rule_id":      alert.RuleID,
		"rule_name":    alert.RuleName,
		"metric_value": alert.MetricValue,
		"threshold":    alert.Threshold,
		"severity":     alert.Severity,
		"status":       alert.Status,
		"triggered_at": alert.TriggeredAt.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// generateAlertID generates a unique ID for alerts
func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}
