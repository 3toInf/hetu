package claude

import (
	"io"
	"os/exec"
)

// process is the seam that lets us test session.go without a real claude binary.
type process interface {
	Start() error
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Wait() error
	Kill() error
}

type execProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
}

func newExecProcess(bin string, args []string, cwd string) (*execProcess, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return &execProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (e *execProcess) Start() error          { return e.cmd.Start() }
func (e *execProcess) Stdin() io.WriteCloser  { return e.stdin }
func (e *execProcess) Stdout() io.Reader      { return e.stdout }
func (e *execProcess) Wait() error            { return e.cmd.Wait() }
func (e *execProcess) Kill() error            { return e.cmd.Process.Kill() }
