//go:build windows

package agentworker

import (
	"fmt"
	"os/exec"
	"syscall"
)

const (
	processSetQuota  = 0x0100
	processTerminate = 0x0001
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	openProcess              = kernel32.NewProc("OpenProcess")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

type processTree struct{ job syscall.Handle }

func prepareProcess(command *exec.Cmd) {}

func attachProcess(command *exec.Cmd) (*processTree, error) {
	job, _, createErr := createJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create job object: %v", createErr)
	}
	process, _, openErr := openProcess.Call(processSetQuota|processTerminate, 0, uintptr(uint32(command.Process.Pid)))
	if process == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return nil, fmt.Errorf("open process: %v", openErr)
	}
	assigned, _, assignErr := assignProcessToJobObject.Call(job, process)
	syscall.CloseHandle(syscall.Handle(process))
	if assigned == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return nil, fmt.Errorf("assign process to job: %v", assignErr)
	}
	return &processTree{job: syscall.Handle(job)}, nil
}

func (p *processTree) terminate() {
	if p.job != 0 {
		terminateJobObject.Call(uintptr(p.job), 1)
	}
}

func (p *processTree) close() {
	if p.job != 0 {
		syscall.CloseHandle(p.job)
		p.job = 0
	}
}
