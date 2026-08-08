package store

import (
	"context"
	"database/sql"
	"time"
)

type Project struct {
	ID        int64
	Name      string
	Path      string
	CreatedAt time.Time
}

// UpsertProject creates a project for a path, or renames if the path already exists.
func (s *Store) UpsertProject(ctx context.Context, name, path string) (Project, error) {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects(name, path, created_at) VALUES(?,?,?)
		 ON CONFLICT(path) DO UPDATE SET name=excluded.name`,
		name, path, now)
	if err != nil {
		return Project{}, err
	}
	p, ok, err := s.GetProjectByPath(ctx, path)
	if err != nil || !ok {
		return Project{}, err
	}
	return p, nil
}

func (s *Store) GetProjectByPath(ctx context.Context, path string) (Project, bool, error) {
	var p Project
	var ct int64
	err := s.db.QueryRowContext(ctx, `SELECT id, name, path, created_at FROM projects WHERE path=?`, path).
		Scan(&p.ID, &p.Name, &p.Path, &ct)
	if err != nil {
		if err == sql.ErrNoRows {
			return Project{}, false, nil // genuine not found -> ok=false, nil error
		}
		return Project{}, false, err // real database error -> propagate
	}
	p.CreatedAt = time.Unix(ct, 0)
	return p, true, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, path, created_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var ct int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &ct); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(ct, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) ProjectPathByID(ctx context.Context, id int64) (string, bool) {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path FROM projects WHERE id=?`, id).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false
		}
		// For non-ErrNoRows errors, we also return not-found since this is best-effort for toDTO
		return "", false
	}
	return path, true
}
