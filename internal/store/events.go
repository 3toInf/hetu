package store

import (
	"context"
	"time"
)

type EventRow struct {
	Seq     int
	Kind    string
	Payload string
	TS      time.Time
}

func (s *Store) AppendEvent(ctx context.Context, hetuID string, seq int, kind, payload string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO session_events(session_hetu, seq, kind, payload, ts) VALUES(?,?,?,?,?)`,
		hetuID, seq, kind, payload, time.Now().Unix())
	return err
}

func (s *Store) RecentEvents(ctx context.Context, hetuID string, limit int) ([]EventRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, kind, COALESCE(payload,''), ts FROM session_events WHERE session_hetu=? ORDER BY seq DESC LIMIT ?`, hetuID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		var ts int64
		if err := rows.Scan(&e.Seq, &e.Kind, &e.Payload, &ts); err != nil {
			return nil, err
		}
		e.TS = time.Unix(ts, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}
