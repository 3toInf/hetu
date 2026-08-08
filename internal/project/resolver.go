package project

import (
	"context"
	"path/filepath"

	"github.com/3toInf/hetu/internal/store"
)

type Resolver struct{ store *store.Store }

func NewResolver(s *store.Store) *Resolver { return &Resolver{store: s} }

// ResolveByCWD returns the project for cwd, creating it (named after the base dir) if absent.
func (r *Resolver) ResolveByCWD(ctx context.Context, cwd string) (store.Project, error) {
	name := filepath.Base(cwd)
	if name == "" || name == "." || name == "/" {
		name = "project"
	}
	return r.store.UpsertProject(ctx, name, cwd)
}
