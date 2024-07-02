package not

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
)

func readOutput(ctx context.Context, prefix string, r io.ReadCloser) {
	scan := bufio.NewScanner(r)
LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		default:
			ok := scan.Scan()
			if !ok {
				break LOOP
			}
			line := scan.Text()
			fmt.Println(prefix, ":", line)
		}
	}
	r.Close()
}

func (w *Watcher) newCmd(ctx context.Context, _cmd Cmd) {
	args := _cmd.Args
	cmd := exec.Command(args[0], args[1:]...)

	if !_cmd.Deamon {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.Stderr = stderr
		cmd.Stdout = stdout
		err := cmd.Run()
		if err != nil {
			w.logger.Error("process stopped", "cmd", cmd.String(), "error", err)
			return
		} else {
			w.logger.Info("process ran", "cmd", cmd.String(), "pid", cmd.Process.Pid, "code", cmd.ProcessState.ExitCode())
			stdoutContent := stdout.String()
			stderrContent := stderr.String()
			if len(stdoutContent) > 0 {
				fmt.Println("STDOUT:", stdout.String())
			}
			if len(stderrContent) > 0 {
				fmt.Println("STDERR:", stderr.String())
			}
		}
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		errKill := cmd.Process.Kill()
		log.Println("STDERR Killing process:", err, errKill)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errKill := cmd.Process.Kill()
		log.Println("STDOUT Killing process:", err, errKill)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readOutput(ctx, "STDOUT", stdout)
	}()
	go func() {
		defer wg.Done()
		readOutput(ctx, "STDERR", stderr)
	}()

	err = cmd.Start()
	if err != nil {
		w.logger.Error("process stopped", "cmd", cmd.String(), "error", err)
		return
	}
	w.logger.Info("process running", "cmd", cmd.String(), "pid", cmd.Process.Pid)
	w.Lock()
	w.onGoingCmds[cmd.Process.Pid] = cmd.Process
	w.Unlock()
	if !_cmd.Deamon {
		cmd.Wait()
	}
	wg.Wait()
	w.Lock()
	delete(w.onGoingCmds, cmd.Process.Pid)
	w.Unlock()
}
