package admin

import (
	"sync"
	"time"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id,omitempty"`
	Username     string                 `json:"username,omitempty"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	Result       string                 `json:"result"` // success, failure, blocked
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// AuditLogQuery represents query parameters for audit logs
type AuditLogQuery struct {
	UserID    string
	Action    string
	Result    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

// AuditLogListResponse represents paginated audit logs response
type AuditLogListResponse struct {
	Logs  []AuditLog `json:"logs"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"page_size"`
}

// AuditService handles audit logging
type AuditService struct {
	mu   sync.RWMutex
	logs []AuditLog
}

// NewAuditService creates a new audit service
func NewAuditService() *AuditService {
	return &AuditService{
		logs: make([]AuditLog, 0),
	}
}

// Log records an audit log entry
func (s *AuditService) Log(log AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if log.ID == "" {
		log.ID = generateID()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	// Prepend to keep most recent first
	s.logs = append([]AuditLog{log}, s.logs...)

	// Keep only the last 10000 logs in memory
	if len(s.logs) > 10000 {
		s.logs = s.logs[:10000]
	}
}

// LogAction is a convenience method for logging actions
func (s *AuditService) LogAction(userID, username, action, resourceType, resourceID, result, ipAddress string, details map[string]interface{}) {
	s.Log(AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		IPAddress:    ipAddress,
		Details:      details,
		CreatedAt:    time.Now(),
	})
}

// Query returns audit logs based on query parameters
func (s *AuditService) Query(q AuditLogQuery) AuditLogListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter logs
	var filtered []AuditLog
	for _, log := range s.logs {
		if q.UserID != "" && log.UserID != q.UserID {
			continue
		}
		if q.Action != "" && log.Action != q.Action {
			continue
		}
		if q.Result != "" && log.Result != q.Result {
			continue
		}
		if q.StartTime != nil && log.CreatedAt.Before(*q.StartTime) {
			continue
		}
		if q.EndTime != nil && log.CreatedAt.After(*q.EndTime) {
			continue
		}
		filtered = append(filtered, log)
	}

	total := len(filtered)

	// Pagination
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}

	start := (q.Page - 1) * q.PageSize
	end := start + q.PageSize

	if start >= len(filtered) {
		return AuditLogListResponse{
			Logs:  []AuditLog{},
			Total: total,
			Page:  q.Page,
			Size:  q.PageSize,
		}
	}

	if end > len(filtered) {
		end = len(filtered)
	}

	return AuditLogListResponse{
		Logs:  filtered[start:end],
		Total: total,
		Page:  q.Page,
		Size:  q.PageSize,
	}
}

// GetRecent returns the most recent N audit logs
func (s *AuditService) GetRecent(n int) []AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n > len(s.logs) {
		n = len(s.logs)
	}
	if n <= 0 {
		return []AuditLog{}
	}

	result := make([]AuditLog, n)
	copy(result, s.logs[:n])
	return result
}

// generateID generates a unique ID for audit logs
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of length n
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1 * time.Nanosecond)
	}
	return string(b)
}
