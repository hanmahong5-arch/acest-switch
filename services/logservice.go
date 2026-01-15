package services

import (
	"database/sql"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const timeLayout = "2006-01-02 15:04:05"

type LogService struct {
	pricing *modelpricing.Service
}

func NewLogService() *LogService {
	svc, err := modelpricing.DefaultService()
	if err != nil {
		log.Printf("pricing service init failed: %v", err)
	}
	return &LogService{pricing: svc}
}

func (ls *LogService) ListRequestLogs(platform string, provider string, limit int) ([]ReqeustLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.OrderByDesc("id"),
		xdb.Limit(limit),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	if provider != "" {
		options = append(options, xdb.WhereEq("provider", provider))
	}
	records, err := model.Selects(options...)
	if err != nil {
		return nil, err
	}
	logs := make([]ReqeustLog, 0, len(records))
	for _, record := range records {
		logEntry := ReqeustLog{
			ID:                record.GetInt64("id"),
			Platform:          record.GetString("platform"),
			Model:             record.GetString("model"),
			Provider:          record.GetString("provider"),
			HttpCode:          record.GetInt("http_code"),
			InputTokens:       record.GetInt("input_tokens"),
			OutputTokens:      record.GetInt("output_tokens"),
			CacheCreateTokens: record.GetInt("cache_create_tokens"),
			CacheReadTokens:   record.GetInt("cache_read_tokens"),
			ReasoningTokens:   record.GetInt("reasoning_tokens"),
			CreatedAt:         record.GetString("created_at"),
			IsStream:          record.GetBool("is_stream"),
			DurationSec:       record.GetFloat64("duration_sec"),
		}
		ls.decorateCost(&logEntry)
		logs = append(logs, logEntry)
	}
	return logs, nil
}

func (ls *LogService) ListProviders(platform string) ([]string, error) {
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.Field("DISTINCT provider as provider"),
		xdb.WhereNotEq("provider", ""),
		xdb.OrderByAsc("provider"),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	records, err := model.Selects(options...)
	if err != nil {
		return nil, err
	}
	providers := make([]string, 0, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.GetString("provider"))
		if name != "" {
			providers = append(providers, name)
		}
	}
	return providers, nil
}

func (ls *LogService) HeatmapStats(days int) ([]HeatmapStat, error) {
	if days <= 0 {
		days = 30
	}
	totalHours := days * 24
	if totalHours <= 0 {
		totalHours = 24
	}
	rangeStart := startOfHour(time.Now())
	if totalHours > 1 {
		rangeStart = rangeStart.Add(-time.Duration(totalHours-1) * time.Hour)
	}

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	// 使用 SQL GROUP BY 按小时聚合，替代 Go 手动聚合
	// strftime('%m-%d %H', created_at) 将时间格式化为 "月-日 时" 格式
	query := `
		SELECT
			strftime('%m-%d %H', created_at) as hour_bucket,
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(reasoning_tokens), 0) as reasoning_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM request_log
		WHERE created_at >= ?
		GROUP BY strftime('%m-%d %H', created_at)
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, rangeStart.Format(timeLayout), totalHours)
	if err != nil {
		if isNoSuchTableErr(err) {
			return []HeatmapStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	stats := make([]HeatmapStat, 0)
	for rows.Next() {
		var stat HeatmapStat
		err := rows.Scan(
			&stat.Day,
			&stat.TotalRequests,
			&stat.InputTokens,
			&stat.OutputTokens,
			&stat.ReasoningTokens,
			&stat.TotalCost,
		)
		if err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (ls *LogService) StatsSince(platform string) (LogStats, error) {
	const seriesHours = 24

	stats := LogStats{
		Series: make([]LogStatsSeries, 0, seriesHours),
	}
	now := time.Now()
	model := xdb.New("request_log")
	seriesStart := startOfDay(now)
	seriesEnd := seriesStart.Add(seriesHours * time.Hour)
	queryStart := seriesStart.Add(-24 * time.Hour)
	summaryStart := seriesStart
	options := []xdb.Option{
		xdb.WhereGte("created_at", queryStart.Format(timeLayout)),
		xdb.Field(
			"input_tokens",
			"output_tokens",
			"reasoning_tokens",
			"cache_create_tokens",
			"cache_read_tokens",
			"input_cost",
			"output_cost",
			"cache_create_cost",
			"cache_read_cost",
			"total_cost",
			"created_at",
		),
		xdb.OrderByAsc("created_at"),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	records, err := model.Selects(options...)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return stats, nil
		}
		return stats, err
	}

	seriesBuckets := make([]*LogStatsSeries, seriesHours)
	for i := 0; i < seriesHours; i++ {
		bucketTime := seriesStart.Add(time.Duration(i) * time.Hour)
		seriesBuckets[i] = &LogStatsSeries{
			Day: bucketTime.Format(timeLayout),
		}
	}

	for _, record := range records {
		createdAt, hasTime := parseCreatedAt(record)
		dayKey := dayFromTimestamp(record.GetString("created_at"))
		isToday := dayKey == seriesStart.Format("2006-01-02")

		if hasTime {
			if createdAt.Before(seriesStart) || !createdAt.Before(seriesEnd) {
				continue
			}
		} else {
			if !isToday {
				continue
			}
			createdAt = seriesStart
		}

		bucketIndex := 0
		if hasTime {
			bucketIndex = int(createdAt.Sub(seriesStart) / time.Hour)
			if bucketIndex < 0 {
				bucketIndex = 0
			}
			if bucketIndex >= seriesHours {
				bucketIndex = seriesHours - 1
			}
		}
		bucket := seriesBuckets[bucketIndex]
		input := record.GetInt("input_tokens")
		output := record.GetInt("output_tokens")
		reasoning := record.GetInt("reasoning_tokens")
		cacheCreate := record.GetInt("cache_create_tokens")
		cacheRead := record.GetInt("cache_read_tokens")
		// 直接读取数据库中已存储的价格，避免重复计算
		inputCost := record.GetFloat64("input_cost")
		outputCost := record.GetFloat64("output_cost")
		cacheCreateCost := record.GetFloat64("cache_create_cost")
		cacheReadCost := record.GetFloat64("cache_read_cost")
		totalCost := record.GetFloat64("total_cost")

		bucket.TotalRequests++
		bucket.InputTokens += int64(input)
		bucket.OutputTokens += int64(output)
		bucket.ReasoningTokens += int64(reasoning)
		bucket.CacheCreateTokens += int64(cacheCreate)
		bucket.CacheReadTokens += int64(cacheRead)
		bucket.TotalCost += totalCost

		if createdAt.IsZero() || createdAt.Before(summaryStart) {
			continue
		}
		stats.TotalRequests++
		stats.InputTokens += int64(input)
		stats.OutputTokens += int64(output)
		stats.ReasoningTokens += int64(reasoning)
		stats.CacheCreateTokens += int64(cacheCreate)
		stats.CacheReadTokens += int64(cacheRead)
		stats.CostInput += inputCost
		stats.CostOutput += outputCost
		stats.CostCacheCreate += cacheCreateCost
		stats.CostCacheRead += cacheReadCost
		stats.CostTotal += totalCost
	}

	for i := 0; i < seriesHours; i++ {
		if bucket := seriesBuckets[i]; bucket != nil {
			stats.Series = append(stats.Series, *bucket)
		} else {
			bucketTime := seriesStart.Add(time.Duration(i) * time.Hour)
			stats.Series = append(stats.Series, LogStatsSeries{
				Day: bucketTime.Format(timeLayout),
			})
		}
	}

	return stats, nil
}

func (ls *LogService) ProviderDailyStats(platform string) ([]ProviderDailyStat, error) {
	start := startOfDay(time.Now())
	end := start.Add(24 * time.Hour)

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	// 使用 SQL GROUP BY 聚合，替代 Go 手动聚合，提升性能
	query := `
		SELECT
			COALESCE(NULLIF(TRIM(provider), ''), '(unknown)') as provider,
			COUNT(*) as total_requests,
			SUM(CASE WHEN http_code >= 200 AND http_code < 300 THEN 1 ELSE 0 END) as successful_requests,
			SUM(CASE WHEN http_code < 200 OR http_code >= 300 THEN 1 ELSE 0 END) as failed_requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(reasoning_tokens), 0) as reasoning_tokens,
			COALESCE(SUM(cache_create_tokens), 0) as cache_create_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as cost_total
		FROM request_log
		WHERE created_at >= ? AND created_at < ?
	`
	args := []interface{}{start.Format(timeLayout), end.Format(timeLayout)}

	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}

	query += `
		GROUP BY COALESCE(NULLIF(TRIM(provider), ''), '(unknown)')
		ORDER BY total_requests DESC, provider ASC
	`

	rows, err := db.Query(query, args...)
	if err != nil {
		if isNoSuchTableErr(err) {
			return []ProviderDailyStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	stats := make([]ProviderDailyStat, 0)
	for rows.Next() {
		var stat ProviderDailyStat
		err := rows.Scan(
			&stat.Provider,
			&stat.TotalRequests,
			&stat.SuccessfulRequests,
			&stat.FailedRequests,
			&stat.InputTokens,
			&stat.OutputTokens,
			&stat.ReasoningTokens,
			&stat.CacheCreateTokens,
			&stat.CacheReadTokens,
			&stat.CostTotal,
		)
		if err != nil {
			return nil, err
		}
		if stat.TotalRequests > 0 {
			stat.SuccessRate = float64(stat.SuccessfulRequests) / float64(stat.TotalRequests)
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (ls *LogService) decorateCost(logEntry *ReqeustLog) {
	if ls == nil || ls.pricing == nil || logEntry == nil {
		return
	}
	usage := modelpricing.UsageSnapshot{
		InputTokens:       logEntry.InputTokens,
		OutputTokens:      logEntry.OutputTokens,
		CacheCreateTokens: logEntry.CacheCreateTokens,
		CacheReadTokens:   logEntry.CacheReadTokens,
	}
	cost := ls.pricing.CalculateCost(logEntry.Model, usage)
	logEntry.HasPricing = cost.HasPricing
	logEntry.InputCost = cost.InputCost
	logEntry.OutputCost = cost.OutputCost
	logEntry.CacheCreateCost = cost.CacheCreateCost
	logEntry.CacheReadCost = cost.CacheReadCost
	logEntry.Ephemeral5mCost = cost.Ephemeral5mCost
	logEntry.Ephemeral1hCost = cost.Ephemeral1hCost
	logEntry.TotalCost = cost.TotalCost
}

func (ls *LogService) calculateCost(model string, usage modelpricing.UsageSnapshot) modelpricing.CostBreakdown {
	if ls == nil || ls.pricing == nil {
		return modelpricing.CostBreakdown{}
	}
	return ls.pricing.CalculateCost(model, usage)
}

func parseCreatedAt(record xdb.Record) (time.Time, bool) {
	if t := record.GetTime("created_at"); t != nil {
		return t.In(time.Local), true
	}
	raw := strings.TrimSpace(record.GetString("created_at"))
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		timeLayout,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05-0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.In(time.Local), true
		}
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed.In(time.Local), true
		}
	}

	if normalized := strings.Replace(raw, " ", "T", 1); normalized != raw {
		if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
			return parsed.In(time.Local), true
		}
	}

	if len(raw) >= len("2006-01-02") {
		if parsed, err := time.ParseInLocation("2006-01-02", raw[:10], time.Local); err == nil {
			return parsed, false
		}
	}

	return time.Time{}, false
}

func dayFromTimestamp(value string) string {
	if len(value) >= len("2006-01-02") {
		if t, err := time.ParseInLocation(timeLayout, value, time.Local); err == nil {
			return t.Format("2006-01-02")
		}
		return value[:10]
	}
	return value
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfHour(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isNoSuchTableErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such table")
}

type HeatmapStat struct {
	Day             string  `json:"day"`
	TotalRequests   int64   `json:"total_requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	TotalCost       float64 `json:"total_cost"`
}

type LogStats struct {
	TotalRequests     int64            `json:"total_requests"`
	InputTokens       int64            `json:"input_tokens"`
	OutputTokens      int64            `json:"output_tokens"`
	ReasoningTokens   int64            `json:"reasoning_tokens"`
	CacheCreateTokens int64            `json:"cache_create_tokens"`
	CacheReadTokens   int64            `json:"cache_read_tokens"`
	CostTotal         float64          `json:"cost_total"`
	CostInput         float64          `json:"cost_input"`
	CostOutput        float64          `json:"cost_output"`
	CostCacheCreate   float64          `json:"cost_cache_create"`
	CostCacheRead     float64          `json:"cost_cache_read"`
	Series            []LogStatsSeries `json:"series"`
}

type ProviderDailyStat struct {
	Provider          string  `json:"provider"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	ReasoningTokens   int64   `json:"reasoning_tokens"`
	CacheCreateTokens int64   `json:"cache_create_tokens"`
	CacheReadTokens   int64   `json:"cache_read_tokens"`
	CostTotal         float64 `json:"cost_total"`
}

type LogStatsSeries struct {
	Day               string  `json:"day"`
	TotalRequests     int64   `json:"total_requests"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	ReasoningTokens   int64   `json:"reasoning_tokens"`
	CacheCreateTokens int64   `json:"cache_create_tokens"`
	CacheReadTokens   int64   `json:"cache_read_tokens"`
	TotalCost         float64 `json:"total_cost"`
}

// CostAnalysis 成本深度分析结构
type CostAnalysis struct {
	InputCostRatio       float64          `json:"input_cost_ratio"`
	OutputCostRatio      float64          `json:"output_cost_ratio"`
	CacheCreateCostRatio float64          `json:"cache_create_cost_ratio"`
	CacheReadCostRatio   float64          `json:"cache_read_cost_ratio"`
	CacheReadTokens      int64            `json:"cache_read_tokens"`
	CacheSavedCost       float64          `json:"cache_saved_cost"`
	CacheSavingPercent   float64          `json:"cache_saving_percent"`
	DailyAvgCost         float64          `json:"daily_avg_cost"`
	CostTrend            []DailyCostPoint `json:"cost_trend"`
	TrendDirection       string           `json:"trend_direction"`
	TrendPercentage      float64          `json:"trend_percentage"`
}

type DailyCostPoint struct {
	Day       string  `json:"day"`
	TotalCost float64 `json:"total_cost"`
	Requests  int64   `json:"requests"`
}

// PerformanceAnalysis 性能与可靠性分析结构
type PerformanceAnalysis struct {
	DurationP50         float64                   `json:"duration_p50"`
	DurationP95         float64                   `json:"duration_p95"`
	DurationP99         float64                   `json:"duration_p99"`
	DurationAvg         float64                   `json:"duration_avg"`
	DurationMin         float64                   `json:"duration_min"`
	DurationMax         float64                   `json:"duration_max"`
	ErrorDistribution   map[string]int64          `json:"error_distribution"`
	TotalErrors         int64                     `json:"total_errors"`
	ErrorRate           float64                   `json:"error_rate"`
	ProviderReliability []ProviderReliabilityStat `json:"provider_reliability"`
}

type ProviderReliabilityStat struct {
	Provider      string           `json:"provider"`
	TotalRequests int64            `json:"total_requests"`
	SuccessCount  int64            `json:"success_count"`
	FailCount     int64            `json:"fail_count"`
	SuccessRate   float64          `json:"success_rate"`
	AvgDuration   float64          `json:"avg_duration"`
	ErrorTypes    map[string]int64 `json:"error_types"`
}

// CostAnalysis 返回成本深度分析数据
func (ls *LogService) CostAnalysis(platform string, days int) (CostAnalysis, error) {
	if days <= 0 {
		days = 7
	}

	result := CostAnalysis{
		CostTrend: make([]DailyCostPoint, 0, days),
	}

	now := time.Now()
	startDate := startOfDay(now).Add(-time.Duration(days-1) * 24 * time.Hour)

	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.WhereGte("created_at", startDate.Format(timeLayout)),
		xdb.Field(
			"input_cost",
			"output_cost",
			"cache_create_cost",
			"cache_read_cost",
			"total_cost",
			"cache_read_tokens",
			"input_tokens",
			"created_at",
		),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}

	records, err := model.Selects(options...)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}

	var totalInputCost, totalOutputCost, totalCacheCreateCost, totalCacheReadCost float64
	var totalCacheReadTokens, totalInputTokens int64
	dailyMap := make(map[string]*DailyCostPoint)

	for _, record := range records {
		totalInputCost += record.GetFloat64("input_cost")
		totalOutputCost += record.GetFloat64("output_cost")
		totalCacheCreateCost += record.GetFloat64("cache_create_cost")
		totalCacheReadCost += record.GetFloat64("cache_read_cost")
		totalCacheReadTokens += int64(record.GetInt("cache_read_tokens"))
		totalInputTokens += int64(record.GetInt("input_tokens"))

		dayKey := dayFromTimestamp(record.GetString("created_at"))
		if dailyMap[dayKey] == nil {
			dailyMap[dayKey] = &DailyCostPoint{Day: dayKey}
		}
		dailyMap[dayKey].TotalCost += record.GetFloat64("total_cost")
		dailyMap[dayKey].Requests++
	}

	totalCost := totalInputCost + totalOutputCost + totalCacheCreateCost + totalCacheReadCost
	if totalCost > 0 {
		result.InputCostRatio = totalInputCost / totalCost
		result.OutputCostRatio = totalOutputCost / totalCost
		result.CacheCreateCostRatio = totalCacheCreateCost / totalCost
		result.CacheReadCostRatio = totalCacheReadCost / totalCost
	}

	result.CacheReadTokens = totalCacheReadTokens
	if totalInputTokens > 0 && totalCacheReadTokens > 0 {
		avgInputCostPerToken := totalInputCost / float64(totalInputTokens)
		fullCostWithoutCache := float64(totalCacheReadTokens) * avgInputCostPerToken
		result.CacheSavedCost = fullCostWithoutCache - totalCacheReadCost
		if fullCostWithoutCache > 0 {
			result.CacheSavingPercent = result.CacheSavedCost / fullCostWithoutCache
		}
	}

	for i := 0; i < days; i++ {
		dayTime := startDate.Add(time.Duration(i) * 24 * time.Hour)
		dayKey := dayTime.Format("2006-01-02")
		point := dailyMap[dayKey]
		if point == nil {
			point = &DailyCostPoint{Day: dayKey}
		}
		result.CostTrend = append(result.CostTrend, *point)
	}

	if len(result.CostTrend) > 0 {
		var sum float64
		for _, p := range result.CostTrend {
			sum += p.TotalCost
		}
		result.DailyAvgCost = sum / float64(len(result.CostTrend))

		if len(result.CostTrend) >= 2 {
			mid := len(result.CostTrend) / 2
			var firstHalf, secondHalf float64
			for i := 0; i < mid; i++ {
				firstHalf += result.CostTrend[i].TotalCost
			}
			for i := mid; i < len(result.CostTrend); i++ {
				secondHalf += result.CostTrend[i].TotalCost
			}
			if firstHalf > 0 {
				change := (secondHalf - firstHalf) / firstHalf
				result.TrendPercentage = change * 100
				if change > 0.1 {
					result.TrendDirection = "up"
				} else if change < -0.1 {
					result.TrendDirection = "down"
				} else {
					result.TrendDirection = "stable"
				}
			} else {
				result.TrendDirection = "stable"
			}
		}
	}

	return result, nil
}

// PerformanceAnalysis 返回性能与可靠性分析数据
func (ls *LogService) PerformanceAnalysis(platform string, days int) (PerformanceAnalysis, error) {
	if days <= 0 {
		days = 1
	}

	result := PerformanceAnalysis{
		ErrorDistribution:   make(map[string]int64),
		ProviderReliability: make([]ProviderReliabilityStat, 0),
	}

	startDate := startOfDay(time.Now()).Add(-time.Duration(days-1) * 24 * time.Hour)

	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.WhereGte("created_at", startDate.Format(timeLayout)),
		xdb.Field(
			"provider",
			"http_code",
			"duration_sec",
			"error_type",
		),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}

	records, err := model.Selects(options...)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}

	durations := make([]float64, 0, len(records))
	providerMap := make(map[string]*ProviderReliabilityStat)
	var totalRequests int64

	for _, record := range records {
		totalRequests++

		duration := record.GetFloat64("duration_sec")
		if duration > 0 {
			durations = append(durations, duration)
		}

		httpCode := record.GetInt("http_code")
		errorType := record.GetString("error_type")
		provider := strings.TrimSpace(record.GetString("provider"))
		if provider == "" {
			provider = "(unknown)"
		}

		if errorType != "" {
			result.ErrorDistribution[errorType]++
			result.TotalErrors++
		}

		stat := providerMap[provider]
		if stat == nil {
			stat = &ProviderReliabilityStat{
				Provider:   provider,
				ErrorTypes: make(map[string]int64),
			}
			providerMap[provider] = stat
		}
		stat.TotalRequests++
		if httpCode >= 200 && httpCode < 300 {
			stat.SuccessCount++
		} else {
			stat.FailCount++
			if errorType != "" {
				stat.ErrorTypes[errorType]++
			}
		}
		if duration > 0 {
			stat.AvgDuration = (stat.AvgDuration*float64(stat.TotalRequests-1) + duration) / float64(stat.TotalRequests)
		}
	}

	if len(durations) > 0 {
		sort.Float64s(durations)
		n := len(durations)
		result.DurationP50 = percentile(durations, 0.50)
		result.DurationP95 = percentile(durations, 0.95)
		result.DurationP99 = percentile(durations, 0.99)
		result.DurationMin = durations[0]
		result.DurationMax = durations[n-1]

		var sum float64
		for _, d := range durations {
			sum += d
		}
		result.DurationAvg = sum / float64(n)
	}

	if totalRequests > 0 {
		result.ErrorRate = float64(result.TotalErrors) / float64(totalRequests)
	}

	for _, stat := range providerMap {
		if stat.TotalRequests > 0 {
			stat.SuccessRate = float64(stat.SuccessCount) / float64(stat.TotalRequests)
		}
		result.ProviderReliability = append(result.ProviderReliability, *stat)
	}

	sort.Slice(result.ProviderReliability, func(i, j int) bool {
		return result.ProviderReliability[i].TotalRequests > result.ProviderReliability[j].TotalRequests
	})

	return result, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := p * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// RequestLogBodyResult 请求/响应体查询结果
type RequestLogBodyResult struct {
	ID            int64  `json:"id"`
	TraceID       string `json:"trace_id"`
	RequestBody   string `json:"request_body"`
	ResponseBody  string `json:"response_body"`
	BodySizeBytes int64  `json:"body_size_bytes"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
}

// GetRequestLogBody 根据 trace_id 获取请求/响应体
func (ls *LogService) GetRequestLogBody(traceID string) (*RequestLogBodyResult, error) {
	if traceID == "" {
		return nil, errors.New("trace_id is required")
	}

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, trace_id, request_body, response_body, body_size_bytes, created_at, expires_at
		FROM request_log_body
		WHERE trace_id = ?
		LIMIT 1
	`

	row := db.QueryRow(query, traceID)
	var result RequestLogBodyResult
	var requestBody, responseBody, createdAt, expiresAt sql.NullString
	err = row.Scan(
		&result.ID,
		&result.TraceID,
		&requestBody,
		&responseBody,
		&result.BodySizeBytes,
		&createdAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // 未找到记录
		}
		return nil, err
	}

	result.RequestBody = requestBody.String
	result.ResponseBody = responseBody.String
	result.CreatedAt = createdAt.String
	result.ExpiresAt = expiresAt.String

	return &result, nil
}
