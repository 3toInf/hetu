package store

import (
	"path/filepath"
	"testing"
)

func filepathInTemp(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "t.sqlite")
}
