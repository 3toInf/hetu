package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

type Message struct {
	Seq     int
	Role    string
	Content string
	TS      int64
}

// SyncMessages replaces all stored messages for a session (REPLACE semantics:
// a rewritten transcript invalidates old seq numbers, so delete-then-insert).
func (s *Store) SyncMessages(ctx context.Context, hetuID string, msgs []Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_messages WHERE session_hetu=?`, hetuID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_messages_fts WHERE session_hetu=?`, hetuID); err != nil {
		return err
	}
	for _, m := range msgs {
		var ts any
		if m.TS != 0 {
			ts = m.TS
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_messages(session_hetu, seq, role, content, ts) VALUES(?,?,?,?,?)`,
			hetuID, m.Seq, m.Role, m.Content, ts); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_messages_fts(session_hetu, role, content) VALUES(?,?,?)`,
			hetuID, m.Role, m.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecentMessages(ctx context.Context, hetuID string, limit int) ([]Message, error) {
	q := `SELECT seq, role, content, COALESCE(ts,0) FROM session_messages WHERE session_hetu=? ORDER BY seq DESC`
	var args []any
	args = append(args, hetuID)
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Seq, &m.Role, &m.Content, &m.TS); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SetContentSynced(ctx context.Context, hetuID string, unix int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET content_synced_at=? WHERE hetu_id=?`, unix, hetuID)
	return err
}

func (s *Store) ContentSyncedAt(ctx context.Context, hetuID string) (int64, bool, error) {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT content_synced_at FROM sessions WHERE hetu_id=?`, hetuID).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v.Int64, v.Valid, nil
}

type SearchHit struct {
	HetuID  string
	Hits    int
	Snippet string
}

// SearchMessages runs a phrase query over the FTS index, aggregated per
// session with the number of matching messages and one snippet.
func (s *Store) SearchMessages(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	// phrase: strip quotes then wrap, so user input is literal
	phrase := `"` + strings.ReplaceAll(query, `"`, ` `) + `"`
	rows, err := s.db.QueryContext(ctx,
		`SELECT session_hetu, snippet(session_messages_fts, 2, '[', ']', '…', 12)
		 FROM session_messages_fts WHERE session_messages_fts MATCH ? LIMIT 500`, phrase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type agg struct {
		hetu string
		hits int
		snip string
	}
	order := []string{}
	m := map[string]*agg{}
	for rows.Next() {
		var hetu, snip string
		if err := rows.Scan(&hetu, &snip); err != nil {
			return nil, err
		}
		a, ok := m[hetu]
		if !ok {
			a = &agg{hetu: hetu, snip: snip}
			m[hetu] = a
			order = append(order, hetu)
		}
		a.hits++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(order))
	for _, h := range order {
		out = append(out, SearchHit{HetuID: h, Hits: m[h].hits, Snippet: m[h].snip})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hits > out[j].Hits })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
