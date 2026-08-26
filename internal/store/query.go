package store

import (
	"context"
	"time"
)

func (s *Store) Totals(ctx context.Context, since *time.Time, person string) (Totals, error) {
	where, args := whereClause(since, person, true)
	stmt, err := s.query(ctx, `SELECT COUNT(*), COUNT(DISTINCT NULLIF(session_id,'')), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0), COALESCE(SUM(cost_usd_micros),0) FROM usage_events `+where, args...)
	if err != nil {
		return Totals{}, err
	}
	defer stmt.close()
	row, _, err := stmt.step()
	if err != nil || !row {
		return Totals{}, err
	}
	return Totals{Requests: stmt.int64(0), Sessions: stmt.int64(1), InputTokens: stmt.int64(2), OutputTokens: stmt.int64(3), CacheReadTokens: stmt.int64(4), CacheCreationTokens: stmt.int64(5), CostUSDMicros: stmt.int64(6)}, nil
}

func (s *Store) ByPerson(ctx context.Context, since *time.Time) ([]PersonUsage, error) {
	where, args := whereClause(since, "", true)
	stmt, err := s.query(ctx, `SELECT COALESCE(NULLIF(person_id,''),'unknown'), COUNT(*), COUNT(DISTINCT NULLIF(session_id,'')), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0), COALESCE(SUM(cost_usd_micros),0) FROM usage_events `+where+` GROUP BY person_id ORDER BY SUM(cost_usd_micros) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer stmt.close()
	out := make([]PersonUsage, 0, 8)
	for {
		row, done, err := stmt.step()
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
		if row {
			out = append(out, PersonUsage{Person: stmt.text(0), Totals: Totals{Requests: stmt.int64(1), Sessions: stmt.int64(2), InputTokens: stmt.int64(3), OutputTokens: stmt.int64(4), CacheReadTokens: stmt.int64(5), CacheCreationTokens: stmt.int64(6), CostUSDMicros: stmt.int64(7)}})
		}
	}
}

func (s *Store) ByAccount(ctx context.Context, since *time.Time, person string) ([]AccountUsage, error) {
	where, args := whereClause(since, person, true)
	stmt, err := s.query(ctx, `SELECT account_uuid, MAX(account_email), COUNT(*), COUNT(DISTINCT NULLIF(session_id,'')), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0), COALESCE(SUM(cost_usd_micros),0) FROM usage_events `+where+` GROUP BY account_uuid ORDER BY SUM(cost_usd_micros) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer stmt.close()
	out := make([]AccountUsage, 0, 8)
	for {
		row, done, err := stmt.step()
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
		if row {
			out = append(out, AccountUsage{AccountUUID: stmt.text(0), AccountEmail: stmt.text(1), Totals: Totals{Requests: stmt.int64(2), Sessions: stmt.int64(3), InputTokens: stmt.int64(4), OutputTokens: stmt.int64(5), CacheReadTokens: stmt.int64(6), CacheCreationTokens: stmt.int64(7), CostUSDMicros: stmt.int64(8)}})
		}
	}
}

func (s *Store) History(ctx context.Context, since *time.Time, person string, limit int) ([]HistoryRow, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	where, args := whereClause(since, person, false)
	args = append(args, int64(limit))
	stmt, err := s.query(ctx, `SELECT event_time,event_name,COALESCE(NULLIF(person_id,''),'unknown'),device_name,account_uuid,account_email,session_id,model,query_source,effort,input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cost_usd_micros,duration_ms,status_code,error FROM usage_events `+where+` ORDER BY event_time DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer stmt.close()
	out := make([]HistoryRow, 0, limit)
	for {
		row, done, err := stmt.step()
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
		if row {
			ts, _ := time.Parse(time.RFC3339Nano, stmt.text(0))
			out = append(out, HistoryRow{Time: ts, EventName: stmt.text(1), Person: stmt.text(2), Device: stmt.text(3), AccountUUID: stmt.text(4), AccountEmail: stmt.text(5), SessionID: stmt.text(6), Model: stmt.text(7), QuerySource: stmt.text(8), Effort: stmt.text(9), InputTokens: stmt.int64(10), OutputTokens: stmt.int64(11), CacheReadTokens: stmt.int64(12), CacheCreationTokens: stmt.int64(13), CostUSDMicros: stmt.int64(14), DurationMS: stmt.int64(15), StatusCode: stmt.int64(16), Error: stmt.text(17)})
		}
	}
}

func (s *Store) Prune(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour).Format(time.RFC3339Nano)
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.db.prepare(`DELETE FROM usage_events WHERE event_time < ?`)
	if err != nil {
		return 0, err
	}
	defer stmt.close()
	if err := stmt.bind(cutoff); err != nil {
		return 0, err
	}
	_, done, err := stmt.step()
	if err != nil || !done {
		return 0, err
	}
	return int64(s.db.changes()), nil
}

func (s *Store) Ping(ctx context.Context) error {
	stmt, err := s.query(ctx, "SELECT 1")
	if err != nil {
		return err
	}
	defer stmt.close()
	row, _, err := stmt.step()
	if err != nil {
		return err
	}
	if !row {
		return context.Canceled
	}
	return nil
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sqliteStmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	stmt, err := s.db.prepare(query)
	if err == nil {
		err = stmt.bind(args...)
	}
	s.mu.Unlock()
	if err != nil {
		if stmt != nil {
			stmt.close()
		}
		return nil, err
	}
	return stmt, nil
}
