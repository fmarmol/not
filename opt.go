package not

import (
	"strings"
)

type WatchOpt func(w *Watcher)

func CmdOpt(cmd Cmd) WatchOpt {
	return func(w *Watcher) {
		w.Cmds = append(w.Cmds, cmd)
	}
}

func ProxyOpt(portApp, portNot int) WatchOpt {
	return func(w *Watcher) {
		w.Proxy.PortApp = portApp
		w.Proxy.PortNot = portNot
		w.Proxy.Activated = true
	}
}

func DirOpt(dir Dir) WatchOpt {
	return func(w *Watcher) {
		extensions := make([]string, 0, len(dir.Exts))
		for _, ext := range dir.Exts {
			if !strings.HasPrefix(ext, ".") {
				extensions = append(extensions, "."+ext)
			} else {
				extensions = append(extensions, ext)
			}
		}
		dir.Exts = extensions
		w.Dirs = append(w.Dirs, dir)
	}
}

func ExtOpt(ext string) WatchOpt {
	return func(w *Watcher) {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		w.Exts = append(w.Exts, ext)
	}
}

func ExcludeFile(filename string) WatchOpt {
	return func(w *Watcher) {
		w.ExcludeFiles = append(w.ExcludeFiles, filename)
	}
}
