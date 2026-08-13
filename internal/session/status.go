package session

type Status string

const (
	StatusRunning             Status = "Running"
	StatusCompleted           Status = "Completed"
	StatusError               Status = "Error"
	StatusIdle                Status = "Idle"
	StatusUnknown             Status = "Unknown"
	StatusWaitingForApproval  Status = "WaitingForApproval"
	StatusWaitingForInput     Status = "WaitingForInput"
)

func (s Status) String() string { return string(s) }

// ParseStatus parses a status string; ok=false if unknown.
func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusRunning, StatusCompleted, StatusError, StatusIdle, StatusUnknown, StatusWaitingForApproval, StatusWaitingForInput:
		return Status(s), true
	}
	return StatusUnknown, false
}
