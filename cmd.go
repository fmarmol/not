package not

import (
	"bufio"
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

func (w *Watcher) newCmd(ctx context.Context, args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		errKill := cmd.Process.Kill()
		log.Println("STDERR Killing process:", err, errKill)
	}
	_ = stderr
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
	cmd.Wait()
	wg.Wait()
	w.Lock()
	delete(w.onGoingCmds, cmd.Process.Pid)
	w.Unlock()
}
