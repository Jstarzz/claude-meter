package otel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ExportLogsServiceRequest struct {
	ResourceLogs []ResourceLogs `json:"resourceLogs"`
}

type ResourceLogs struct {
	Resource  Resource    `json:"resource"`
	ScopeLogs []ScopeLogs `json:"scopeLogs"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeLogs struct {
	LogRecords []LogRecord `json:"logRecords"`
}

type LogRecord struct {
	TimeUnixNano         string          `json:"timeUnixNano"`
	ObservedTimeUnixNano string          `json:"observedTimeUnixNano"`
	Body                 json.RawMessage `json:"body"`
	Attributes           []KeyValue      `json:"attributes"`
}

type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

type AnyValue struct {
	StringValue *string         `json:"stringValue,omitempty"`
	IntValue    json.RawMessage `json:"intValue,omitempty"`
	DoubleValue *float64        `json:"doubleValue,omitempty"`
	BoolValue   *bool           `json:"boolValue,omitempty"`
	ArrayValue  *ArrayValue     `json:"arrayValue,omitempty"`
	KVListValue *KeyValueList   `json:"kvlistValue,omitempty"`
	BytesValue  *string         `json:"bytesValue,omitempty"`
}

type ArrayValue struct {
	Values []AnyValue `json:"values"`
}

type KeyValueList struct {
	Values []KeyValue `json:"values"`
}

type Event struct {
	EventKey            string
	EventName           string
	EventTime           time.Time
	EventSequence       int64
	PersonID            string
	DeviceName          string
	DeviceNodeID        string
	SourceIP            string
	AccountUUID         string
	AccountID           string
	AccountEmail        string
	OrganizationID      string
	UserID              string
	SessionID           string
	RequestID           string
	ClientRequestID     string
	Model               string
	QuerySource         string
	Effort              string
	Speed               string
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	CostUSDMicros       int64
	DurationMS          int64
	StatusCode          int64
	Error               string
}

func Decode(data []byte, receivedAt time.Time) ([]Event, error) {
	var req ExportLogsServiceRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("decode OTLP JSON: %w", err)
	}
	events := make([]Event, 0, 16)
	for _, rl := range req.ResourceLogs {
		resourceAttrs := attrsToMap(rl.Resource.Attributes)
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				attrs := cloneMap(resourceAttrs)
				for k, v := range attrsToMap(lr.Attributes) {
					attrs[k] = v
				}
				name := asString(attrs["event.name"])
				if name == "" {
					name = bodyEventName(lr.Body)
				}
				if name != "api_request" && name != "api_error" {
					continue
				}
				e := Event{
					EventName:           name,
					EventTime:           eventTime(attrs, lr, receivedAt),
					EventSequence:       asInt64(attrs["event.sequence"]),
					PersonID:            asString(attrs["meter.person"]),
					DeviceName:          asString(attrs["meter.device"]),
					DeviceNodeID:        asString(attrs["meter.node_id"]),
					SourceIP:            asString(attrs["meter.source_ip"]),
					AccountUUID:         asString(attrs["user.account_uuid"]),
					AccountID:           asString(attrs["user.account_id"]),
					AccountEmail:        asString(attrs["user.email"]),
					OrganizationID:      asString(attrs["organization.id"]),
					UserID:              asString(attrs["user.id"]),
					SessionID:           asString(attrs["session.id"]),
					RequestID:           asString(attrs["request_id"]),
					ClientRequestID:     asString(attrs["client_request_id"]),
					Model:               asString(attrs["model"]),
					QuerySource:         asString(attrs["query_source"]),
					Effort:              asString(attrs["effort"]),
					Speed:               asString(attrs["speed"]),
					InputTokens:         asInt64(attrs["input_tokens"]),
					OutputTokens:        asInt64(attrs["output_tokens"]),
					CacheReadTokens:     asInt64(attrs["cache_read_tokens"]),
					CacheCreationTokens: asInt64(attrs["cache_creation_tokens"]),
					CostUSDMicros:       costMicros(attrs),
					DurationMS:          asInt64(attrs["duration_ms"]),
					StatusCode:          asInt64(attrs["status_code"]),
					Error:               asString(attrs["error"]),
				}
				e.EventKey = makeEventKey(e)
				events = append(events, e)
			}
		}
	}
	return events, nil
}

func attrsToMap(kvs []KeyValue) map[string]any {
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyValue(kv.Value)
	}
	return m
}

func anyValue(v AnyValue) any {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if len(v.IntValue) > 0 {
		var s string
		if json.Unmarshal(v.IntValue, &s) == nil {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n
			}
			return s
		}
		var n int64
		if json.Unmarshal(v.IntValue, &n) == nil {
			return n
		}
	}
	if v.DoubleValue != nil {
		return *v.DoubleValue
	}
	if v.BoolValue != nil {
		return *v.BoolValue
	}
	if v.ArrayValue != nil {
		out := make([]any, 0, len(v.ArrayValue.Values))
		for _, item := range v.ArrayValue.Values {
			out = append(out, anyValue(item))
		}
		return out
	}
	if v.KVListValue != nil {
		return attrsToMap(v.KVListValue.Values)
	}
	if v.BytesValue != nil {
		return *v.BytesValue
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+16)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func costMicros(attrs map[string]any) int64 {
	if n := asInt64(attrs["cost_usd_micros"]); n != 0 {
		return n
	}
	switch x := attrs["cost_usd"].(type) {
	case float64:
		return int64(x*1_000_000 + 0.5)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return int64(f*1_000_000 + 0.5)
	}
	return 0
}

func eventTime(attrs map[string]any, lr LogRecord, fallback time.Time) time.Time {
	if s := asString(attrs["event.timestamp"]); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC()
		}
	}
	for _, raw := range []string{lr.TimeUnixNano, lr.ObservedTimeUnixNano} {
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && n > 0 {
			return time.Unix(0, n).UTC()
		}
	}
	return fallback.UTC()
}

func bodyEventName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var av AnyValue
	if json.Unmarshal(raw, &av) != nil {
		return ""
	}
	s := asString(anyValue(av))
	s = strings.TrimSpace(strings.TrimPrefix(s, "claude_code."))
	return s
}

func makeEventKey(e Event) string {
	raw := strings.Join([]string{
		e.EventName,
		e.SessionID,
		strconv.FormatInt(e.EventSequence, 10),
		e.RequestID,
		e.ClientRequestID,
		e.EventTime.Format(time.RFC3339Nano),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
