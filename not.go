package not

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Proxy struct {
	Activated bool `toml:"activated"`
	PortApp   int  `toml:"port_app"`
	PortNot   int  `toml:"port_not"`
}

type Watcher struct {
	Dirs         []string
	Exts         []string
	ExcludeFiles []string
	ExcludedDirs []string
	IncludedExt  []string
	Cmds         [][]string
	Proxy        Proxy
	waitProxy    chan struct{}
	ctx          context.Context
	stop         context.CancelFunc
	close        chan struct{}
	success      chan struct{}
	logger       *slog.Logger
	onGoingCmds  map[int]*os.Process
	sync.Mutex
	//Parallel     bool
}

type WatchOpt func(w *Watcher)

func CmdOpt(args []string) WatchOpt {
	return func(w *Watcher) {
		w.Cmds = append(w.Cmds, args)
	}
}

func ProxyOpt(portApp, portNot int) WatchOpt {
	return func(w *Watcher) {
		w.Proxy.PortApp = portApp
		w.Proxy.PortNot = portNot
		w.Proxy.Activated = true
	}
}

func DirOpt(dir string) WatchOpt {
	return func(w *Watcher) {
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

func NewWatcher(opts ...WatchOpt) *Watcher {
	w := new(Watcher)
	for _, opt := range opts {
		opt(w)
	}
	if len(w.Dirs) == 0 {
		w.Dirs = []string{"."}
	}
	if w.ctx == nil {
		w.ctx, w.stop = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	}
	if w.success == nil {
		w.success = make(chan struct{})
	}
	if w.close == nil {
		w.close = make(chan struct{})
	}
	if len(w.Cmds) == 0 {
		panic("no command defined")
	}
	if w.logger == nil {
		w.logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if w.onGoingCmds == nil {
		w.onGoingCmds = make(map[int]*os.Process)
	}
	if w.Proxy.Activated {
		w.waitProxy = make(chan struct{})
		go w.runProxy(w.ctx, w.waitProxy)
	}
	return w
}

func (w *Watcher) Run() error {
	w.logger.Info("starting not...", "pid", os.Getpid())
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	for _, dir := range w.Dirs {
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}
	}

	go func() {
	EVENTS_LOOP:
		for {
			select {
			case <-w.ctx.Done():
				w.logger.Info("received signal to close")
				close(w.close)
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					w.logger.Info("modified file:", "file", event.Name)
					for _, ex := range w.ExcludeFiles {
						if event.Name == ex {
							continue EVENTS_LOOP
						}
					}
					if len(w.Exts) > 0 {
						var found bool
						for _, ext := range w.Exts {
							if filepath.Ext(event.Name) == ext {
								found = true
								break
							}
						}
						if !found {
							continue EVENTS_LOOP
						}
					}
					w.success <- struct{}{}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
	var done sync.WaitGroup
	go func() {
		done.Add(1)
		defer done.Done()
		for range w.success {

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
				err = process.Signal(os.Interrupt)
				if err != nil {
					panic(err)
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

			go func() {
				for _, args := range w.Cmds {
					w.newCmd(w.ctx, args)
				}
			}()
		}
	}()
	<-w.close
	err = watcher.Close()
	if err != nil {
		w.logger.Error("failed to close fsnotify watcher:", "error", err)
	}
	close(w.success)
	done.Wait()

	if w.waitProxy != nil {
		<-w.waitProxy
	}

	return nil
}
