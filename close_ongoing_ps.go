package not

import (
	"log"
	"os"
	"reflect"
	"time"
)

func (w *Watcher) CloseOngoingProcesses() {
	w.Lock()
	for pid, process := range w.onGoingCmds {
		done := make(chan struct{})
		go func() {
			w.logger.Info("waiting processs", "pid", pid)
			state, err := process.Wait()
			if err != nil {
				w.logger.Error("wait process", "error", err, "pid", pid)
			}
			w.logger.Info("wait process", "code", state.ExitCode(), "pid", pid)
			done <- struct{}{}
		}()
		w.logger.Info("stopping process", "pid", pid)
		err := process.Signal(os.Interrupt)
		if err != nil {
			w.logger.Error("stop process", "error", err)
		}
		select {
		case <-time.After(10 * time.Second):
			err := process.Kill()
			if err != nil {
				log.Println("KILL WITH ERR:", reflect.TypeOf(err), err)
			}
		case <-done:
			w.logger.Info("process stopped", "pid", pid)
		}
	}
	w.Unlock()
}
