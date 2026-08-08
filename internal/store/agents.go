package store

import "context"

type AgentRow struct {
	ID      int64
	Name    string
	Enabled bool
	Binary  string
}

func (s *Store) UpsertAgent(ctx context.Context, name, binary string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agents(name, enabled, binary, created_at) VALUES(?,?,?,strftime('%s','now'))
		 ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled, binary=excluded.binary`,
		name, en, binary)
	return err
}

func (s *Store) ListAgents(ctx context.Context) ([]AgentRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, enabled, COALESCE(binary,'') FROM agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentRow
	for rows.Next() {
		var a AgentRow
		var en int
		if err := rows.Scan(&a.ID, &a.Name, &en, &a.Binary); err != nil {
			return nil, err
		}
		a.Enabled = en == 1
		out = append(out, a)
	}
	return out, rows.Err()
}
