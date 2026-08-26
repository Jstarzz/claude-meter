package store

import (
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu sync.Mutex
	db *sqliteDB
}

type Totals struct {
	Requests            int64 `json:"requests"`
	Sessions            int64 `json:"sessions"`
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CostUSDMicros       int64 `json:"cost_usd_micros"`
}

type PersonUsage struct {
	Person string `json:"person"`
	Totals
}

type AccountUsage struct {
	AccountUUID  string `json:"account_uuid"`
	AccountEmail string `json:"account_email"`
	Totals
}

type HistoryRow struct {
	Time                time.Time `json:"time"`
	EventName           string    `json:"event_name"`
	Person              string    `json:"person"`
	Device              string    `json:"device"`
	AccountUUID         string    `json:"account_uuid"`
	AccountEmail        string    `json:"account_email"`
	SessionID           string    `json:"session_id"`
	Model               string    `json:"model"`
	QuerySource         string    `json:"query_source"`
	Effort              string    `json:"effort"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CostUSDMicros       int64     `json:"cost_usd_micros"`
	DurationMS          int64     `json:"duration_ms"`
	StatusCode          int64     `json:"status_code"`
	Error               string    `json:"error,omitempty"`
}

func whereClause(since *time.Time, person string, requestsOnly bool) (string, []any) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 2)
	if requestsOnly {
		conditions = append(conditions, "event_name='api_request'")
	}
	if since != nil {
		conditions = append(conditions, "event_time >= ?")
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	if strings.TrimSpace(person) != "" {
		conditions = append(conditions, "person_id = ?")
		args = append(args, person)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}
