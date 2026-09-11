package manager

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type process struct {
	command       *exec.Cmd
	file          *os.File
	done          chan struct{}
	err           error
	stopRequested bool
}

func newProcess(command *exec.Cmd, path string) (*process, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	command.Stdout = file
	if command.Stderr == nil {
		command.Stderr = file
	}
	if err := command.Start(); err != nil {
		file.Close()
		return nil, err
	}
	started := &process{command: command, file: file, done: make(chan struct{})}
	go func() { started.err = command.Wait(); close(started.done) }()
	return started, nil
}

func (p *process) signal() error {
	if p.stopRequested {
		return nil
	}
	select {
	case <-p.done:
		return nil
	default:
	}
	p.stopRequested = true
	err := p.command.Process.Signal(syscall.SIGTERM)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func (p *process) stop() error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := p.signal(); err != nil {
		return err
	}
	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-timer.C:
		err := p.command.Process.Kill()
		<-p.done
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
}
