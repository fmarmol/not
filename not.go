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
	Dirs         []Dir
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
}

func NewWatcher(opts ...WatchOpt) *Watcher {
	w := new(Watcher)
	for _, opt := range opts {
		opt(w)
	}
	if len(w.Dirs) == 0 {
		w.Dirs = []Dir{{Name: "."}}
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
		err = watcher.Add(dir.Name)
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
					fileName, err := filepath.Abs(event.Name)
					if err != nil {
						panic(err)
					}
					dirFile := filepath.Dir(fileName)

					for _, ex := range w.ExcludeFiles {
						// NAIVE IMPLEMENTATION TODO: CHANGE
						if strings.Contains(fileName, ex) {
							continue EVENTS_LOOP
						}
					}
					// find the dir and check exts
					var checkDir bool
					for _, dir := range w.Dirs {
						// NAIVE IMPLEMENTATION
						if dir.Name == dirFile && len(dir.Exts) > 0 {
							var found bool
							for _, ext := range dir.Exts {
								if filepath.Ext(fileName) == ext {
									found = true
									checkDir = true
									break
								}
							}
							if !found {
								continue EVENTS_LOOP
							}
						}
					}

					if !checkDir && len(w.Exts) > 0 {
						var found bool
						for _, ext := range w.Exts {
							if filepath.Ext(fileName) == ext {
								found = true
								break
							}
						}
						if !found {
							continue EVENTS_LOOP
						}
					}
					w.logger.Info("modified file:", "file", fileName)
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
