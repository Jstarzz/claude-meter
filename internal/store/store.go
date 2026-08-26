package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Jstarzz/claude-meter/internal/otel"
)

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.db.exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA foreign_keys=ON;`); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.migrate(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.close()
}

func (s *Store) migrate() error {
	return s.db.exec(`
CREATE TABLE IF NOT EXISTS usage_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 event_key TEXT NOT NULL UNIQUE,
 event_name TEXT NOT NULL,
 event_time TEXT NOT NULL,
 received_at TEXT NOT NULL,
 event_sequence INTEGER NOT NULL DEFAULT 0,
 person_id TEXT NOT NULL DEFAULT '',
 device_name TEXT NOT NULL DEFAULT '',
 device_node_id TEXT NOT NULL DEFAULT '',
 source_ip TEXT NOT NULL DEFAULT '',
 account_uuid TEXT NOT NULL DEFAULT '',
 account_id TEXT NOT NULL DEFAULT '',
 account_email TEXT NOT NULL DEFAULT '',
 organization_id TEXT NOT NULL DEFAULT '',
 user_id TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 request_id TEXT NOT NULL DEFAULT '',
 client_request_id TEXT NOT NULL DEFAULT '',
 model TEXT NOT NULL DEFAULT '',
 query_source TEXT NOT NULL DEFAULT '',
 effort TEXT NOT NULL DEFAULT '',
 speed TEXT NOT NULL DEFAULT '',
 input_tokens INTEGER NOT NULL DEFAULT 0,
 output_tokens INTEGER NOT NULL DEFAULT 0,
 cache_read_tokens INTEGER NOT NULL DEFAULT 0,
 cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
 cost_usd_micros INTEGER NOT NULL DEFAULT 0,
 duration_ms INTEGER NOT NULL DEFAULT 0,
 status_code INTEGER NOT NULL DEFAULT 0,
 error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_usage_time ON usage_events(event_time DESC);
CREATE INDEX IF NOT EXISTS idx_usage_person_time ON usage_events(person_id, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_usage_account_time ON usage_events(account_uuid, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_usage_session ON usage_events(session_id);`)
}

func (s *Store) Insert(ctx context.Context, events []otel.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.exec("BEGIN IMMEDIATE"); err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.db.exec("ROLLBACK")
		}
	}()
	stmt, err := s.db.prepare(`INSERT OR IGNORE INTO usage_events (
 event_key,event_name,event_time,received_at,event_sequence,person_id,device_name,device_node_id,source_ip,
 account_uuid,account_id,account_email,organization_id,user_id,session_id,request_id,client_request_id,
 model,query_source,effort,speed,input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,
 cost_usd_micros,duration_ms,status_code,error
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	inserted := 0
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return inserted, err
		}
		if err := stmt.bind(
			e.EventKey, e.EventName, e.EventTime.UTC().Format(time.RFC3339Nano), now, e.EventSequence,
			e.PersonID, e.DeviceName, e.DeviceNodeID, e.SourceIP, e.AccountUUID, e.AccountID, e.AccountEmail,
			e.OrganizationID, e.UserID, e.SessionID, e.RequestID, e.ClientRequestID, e.Model, e.QuerySource, e.Effort, e.Speed,
			e.InputTokens, e.OutputTokens, e.CacheReadTokens, e.CacheCreationTokens, e.CostUSDMicros, e.DurationMS, e.StatusCode, e.Error,
		); err != nil {
			return inserted, err
		}
		_, done, err := stmt.step()
		if err != nil {
			return inserted, err
		}
		if !done {
			return inserted, fmt.Errorf("insert did not complete")
		}
		inserted += s.db.changes()
		stmt.reset()
	}
	if err := s.db.exec("COMMIT"); err != nil {
		return inserted, err
	}
	committed = true
	return inserted, nil
}
