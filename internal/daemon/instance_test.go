package daemon

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquireInstanceLockExclusive(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "hetud.lock")

	// First acquire succeeds and records our pid.
	f1, err := AcquireInstanceLock(lockPath)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	t.Cleanup(func() { f1.Close() })

	// Second acquire on the same file must fail while the first is held.
	if _, err := AcquireInstanceLock(lockPath); err == nil {
		t.Fatal("expected second acquire to fail while the lock is held")
	} else if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
}

func TestAcquireInstanceLockReleasedOnClose(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "hetud.lock")

	f1, err := AcquireInstanceLock(lockPath)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := f1.Close(); err != nil {
		t.Fatal(err)
	}

	// After the holder closes, the lock is free again (e.g. a crashed daemon
	// would leave the lock released so a later hetud can clean up).
	if f2, err := AcquireInstanceLock(lockPath); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	} else {
		f2.Close()
	}
}

func TestAcquireInstanceLockRecordsPid(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "hetud.lock")

	f1, err := AcquireInstanceLock(lockPath)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { f1.Close() })
	if got := readPidLocked(lockPath); got == 0 {
		t.Fatal("expected a non-zero pid recorded in the lock file")
	}
}
