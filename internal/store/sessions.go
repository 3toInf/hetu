package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/3toInf/hetu/internal/session"
)

// ErrAmbiguousID is returned by ResolveSession when more than one session shares
// the given id prefix.
var ErrAmbiguousID = errors.New("ambiguous session id; use more characters")

type Session struct {
	HetuID       string
	Agent        string
	ExternalID   string
	ProjectID    int64
	Host         string
	CWD          string
	Title        string
	Status       session.Status
	Driven       bool
	Unread       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastEventAt  *time.Time
	LastViewedAt *time.Time
}

type ListFilter struct {
	ProjectID int64
	Status    string
	Agent     string
	Limit     int
}

func (s *Store) UpsertSession(ctx context.Context, in Session) (Session, error) {
	if in.Host == "" {
		in.Host = "local"
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}
	in.UpdatedAt = time.Now()
	driven := 0
	if in.Driven {
		driven = 1
	}
	unread := 0
	if in.Unread {
		unread = 1
	}
	var projID any
	if in.ProjectID != 0 {
		projID = in.ProjectID
	}
	var lastEv any
	if in.LastEventAt != nil {
		lastEv = in.LastEventAt.Unix()
	}
	var lastView any
	if in.LastViewedAt != nil {
		lastView = in.LastViewedAt.Unix()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(hetu_id, agent, external_id, project_id, host, cwd, title, status, driven, unread, created_at, updated_at, last_event_at, last_viewed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(agent, external_id) DO UPDATE SET
			hetu_id=excluded.hetu_id,
			project_id=COALESCE(excluded.project_id, sessions.project_id),
			host=excluded.host, cwd=excluded.cwd, title=excluded.title,
			status=excluded.status, driven=excluded.driven, updated_at=excluded.updated_at,
			last_event_at=COALESCE(excluded.last_event_at, sessions.last_event_at)`,
		in.HetuID, in.Agent, in.ExternalID, projID, in.Host, in.CWD, in.Title, in.Status, driven, unread, in.CreatedAt.Unix(), in.UpdatedAt.Unix(), lastEv, lastView)
	if err != nil {
		return Session{}, err
	}
	got, ok, err := s.GetSession(ctx, in.HetuID)
	if err != nil || !ok {
		return Session{}, err
	}
	return got, nil
}

func (s *Store) GetSession(ctx context.Context, hetuID string) (Session, bool, error) {
	row := s.db.QueryRowContext(ctx, sessionCols+` FROM sessions WHERE hetu_id=?`, hetuID)
	out, err := scanSession(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, false, nil
		}
		return Session{}, false, err
	}
	return out, true, nil
}

// GetSessionByExternal looks up a session by its (agent, external_id) pair —
// the natural identity used by the upsert conflict clause. It lets Ensure reuse
// an existing session's hetu_id rather than minting a new one (which would
// invalidate the id users see in `hetu sessions`).
func (s *Store) GetSessionByExternal(ctx context.Context, agentName, externalID string) (Session, bool, error) {
	row := s.db.QueryRowContext(ctx, sessionCols+` FROM sessions WHERE agent=? AND external_id=?`, agentName, externalID)
	out, err := scanSession(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, false, nil
		}
		return Session{}, false, err
	}
	return out, true, nil
}

// ResolveSession looks up a session by its full hetu_id or a unique prefix of
// it. The session table only ever displays truncated ids (see cli.short), so
// callers must resolve a user-supplied id through here rather than GetSession.
// It returns ok=false when nothing matches and an error (ErrAmbiguousID) when
// the prefix matches more than one session.
func (s *Store) ResolveSession(ctx context.Context, idOrPrefix string) (Session, bool, error) {
	// Fast path: exact match covers full ids (and any short id stored verbatim).
	if se, ok, err := s.GetSession(ctx, idOrPrefix); err != nil {
		return Session{}, false, err
	} else if ok {
		return se, true, nil
	}
	rows, err := s.db.QueryContext(ctx, sessionCols+` FROM sessions WHERE hetu_id LIKE ? ESCAPE '\'`, likePrefix(idOrPrefix))
	if err != nil {
		return Session{}, false, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		se, err := scanSession(rows)
		if err != nil {
			return Session{}, false, err
		}
		out = append(out, se)
	}
	if err := rows.Err(); err != nil {
		return Session{}, false, err
	}
	switch len(out) {
	case 0:
		return Session{}, false, nil
	case 1:
		return out[0], true, nil
	default:
		return Session{}, false, ErrAmbiguousID
	}
}

// likePrefix escapes LIKE metacharacters and appends the trailing wildcard,
// turning an id prefix into a safe LIKE pattern.
func likePrefix(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(p) + `%`
}

const sessionCols = `SELECT hetu_id, agent, external_id, COALESCE(project_id,0), host, cwd, COALESCE(title,''), status, driven, unread, created_at, updated_at, last_event_at, last_viewed_at`

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var s Session
	var driven, unread int
	var ct, ut int64
	var lastEv *int64
	var lastView *int64
	if err := row.Scan(&s.HetuID, &s.Agent, &s.ExternalID, &s.ProjectID, &s.Host, &s.CWD, &s.Title, &s.Status, &driven, &unread, &ct, &ut, &lastEv, &lastView); err != nil {
		return Session{}, err
	}
	s.Driven = driven == 1
	s.Unread = unread == 1
	s.CreatedAt = time.Unix(ct, 0)
	s.UpdatedAt = time.Unix(ut, 0)
	if lastEv != nil {
		t := time.Unix(*lastEv, 0)
		s.LastEventAt = &t
	}
	if lastView != nil {
		t := time.Unix(*lastView, 0)
		s.LastViewedAt = &t
	}
	return s, nil
}

func (s *Store) ListSessions(ctx context.Context, f ListFilter) ([]Session, error) {
	q := sessionCols + ` FROM sessions WHERE 1=1`
	var args []any
	if f.ProjectID != 0 {
		q += ` AND project_id=?`
		args = append(args, f.ProjectID)
	}
	if f.Status != "" {
		q += ` AND status=?`
		args = append(args, f.Status)
	}
	if f.Agent != "" {
		q += ` AND agent=?`
		args = append(args, f.Agent)
	}
	q += ` ORDER BY CASE status
		WHEN 'WaitingForApproval' THEN 0 WHEN 'WaitingForInput' THEN 0
		WHEN 'Error' THEN 1 WHEN 'Running' THEN 2 WHEN 'Completed' THEN 3 WHEN 'Idle' THEN 4 ELSE 5 END,
		updated_at DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		se, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, se)
	}
	return out, rows.Err()
}

func (s *Store) UpdateSessionStatus(ctx context.Context, hetuID string, st session.Status, driven bool) error {
	d := 0
	if driven {
		d = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status=?, driven=?, updated_at=strftime('%s','now'), last_event_at=strftime('%s','now') WHERE hetu_id=?`,
		st, d, hetuID)
	return err
}

func (s *Store) MarkRead(ctx context.Context, hetuID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET unread=0, last_viewed_at=strftime('%s','now') WHERE hetu_id=?`, hetuID)
	return err
}

func (s *Store) CountAttention(ctx context.Context, projectID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM sessions WHERE project_id=? AND (status IN ('WaitingForApproval','WaitingForInput','Error') OR unread=1)`,
		projectID).Scan(&n)
	return n, err
}
