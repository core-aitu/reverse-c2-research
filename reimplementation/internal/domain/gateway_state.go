package domain

const (
	TaskQueued    = "queued"
	TaskLeased    = "leased"
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

func ValidTaskTransition(from, to string) bool {
	switch from {
	case TaskQueued:
		return to == TaskLeased || to == TaskCancelled || to == TaskFailed
	case TaskLeased:
		return to == TaskRunning || to == TaskLeased || to == TaskCancelled || to == TaskFailed
	case TaskRunning:
		return to == TaskCompleted || to == TaskFailed || to == TaskCancelled || to == TaskLeased
	default:
		return false
	}
}

func TerminalTaskState(state string) bool {
	return state == TaskCompleted || state == TaskFailed || state == TaskCancelled
}
