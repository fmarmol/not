package main

import (
	"github.com/fmarmol/not"
)

func main() {
	w := not.NewWatcher(
		not.CmdOpt([]string{"touch", "README.io"}),
		not.CmdOpt([]string{"httpserver", "-p", "1234"}),
		not.ProxyOpt(8082, 8083),
	)
	w.Run()

}
