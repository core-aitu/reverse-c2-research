package domain

import "testing"

func TestTaskStateTransitions(t *testing.T) {
	allowed := map[[2]string]bool{
		{TaskQueued, TaskLeased}:       true,
		{TaskQueued, TaskCancelled}:    true,
		{TaskLeased, TaskRunning}:      true,
		{TaskLeased, TaskLeased}:       true,
		{TaskRunning, TaskCompleted}:   true,
		{TaskRunning, TaskFailed}:      true,
		{TaskRunning, TaskCancelled}:   true,
		{TaskRunning, TaskLeased}:      true,
		{TaskCompleted, TaskRunning}:   false,
		{TaskFailed, TaskLeased}:       false,
		{TaskCancelled, TaskCompleted}: false,
		{TaskQueued, TaskCompleted}:    false,
	}
	for pair, want := range allowed {
		if got := ValidTaskTransition(pair[0], pair[1]); got != want {
			t.Errorf("%s -> %s: got %v want %v", pair[0], pair[1], got, want)
		}
	}
	for _, state := range []string{TaskCompleted, TaskFailed, TaskCancelled} {
		if !TerminalTaskState(state) {
			t.Errorf("%s should be terminal", state)
		}
	}
}
