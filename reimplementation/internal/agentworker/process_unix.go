//go:build !windows

package agentworker

import (
	"os/exec"
	"syscall"
)

type processTree struct{ command *exec.Cmd }

func prepareProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcess(command *exec.Cmd) (*processTree, error) {
	return &processTree{command: command}, nil
}

func (p *processTree) terminate() {
	if p.command.Process != nil {
		_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)
	}
}

func (p *processTree) close() {}
