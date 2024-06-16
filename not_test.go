package not

import (
	"os/exec"
	"testing"
)

func TestWatcher(t *testing.T) {
	w := NewWatcher(CmdOpt(exec.Command("echo", "hello world")))
	w.Run()
}
