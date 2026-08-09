package daemon

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ErrAlreadyRunning is returned by AcquireInstanceLock when another hetud
// already holds the lock.
var ErrAlreadyRunning = errors.New("hetud already running")

// AcquireInstanceLock takes an exclusive advisory lock on lockPath so that only
// one hetud can run at a time. The returned file must be kept open for the
// lifetime of the process: the lock is released automatically when it is closed
// or the process exits (including crashes), which also lets a later hetud clean
// up a stale socket safely.
//
// If another hetud holds the lock, the returned error wraps ErrAlreadyRunning
// and names the running pid when it can be read from the file.
func AcquireInstanceLock(lockPath string) (*os.File, error) {
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		pid := readPidLocked(lockPath)
		f.Close()
		if pid != 0 {
			return nil, fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, pid)
		}
		return nil, ErrAlreadyRunning
	}
	// Record our pid for the benefit of the next instance's error message.
	if err := f.Truncate(0); err != nil {
		// non-fatal; the lock itself is what guarantees single instance
	}
	if _, err := f.Seek(0, 0); err != nil {
		// non-fatal
	}
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		// non-fatal
	}
	return f, nil
}

// readPidLocked reads the pid recorded in the lock file. Best-effort: a missing
// or malformed value just yields 0.
func readPidLocked(lockPath string) int {
	b, err := os.ReadFile(lockPath)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return pid
}
