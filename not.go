package not

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Proxy struct {
	Activated bool `toml:"activated"`
	PortApp   int  `toml:"port_app"`
	PortNot   int  `toml:"port_not"`
}

type Cmd struct {
	Args   []string
	Deamon bool
}

type Watcher struct {
	Dirs         []Dir
	Exts         []string
	ExcludeFiles []string
	ExcludedDirs []string
	IncludedExt  []string
	Cmds         []Cmd
	Proxy        Proxy
	waitProxy    chan struct{}
	ctx          context.Context
	stop         context.CancelFunc
	close        chan struct{}
	success      chan struct{}
	logger       *slog.Logger
	onGoingCmds  map[int]*os.Process
	events       chan struct{}
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
	if w.events == nil {
		w.events = make(chan struct{})
	}
	if w.Proxy.Activated {
		w.waitProxy = make(chan struct{})
		go w.runProxy(w.ctx, w.waitProxy)
	}
	return w
}

func (w *Watcher) Run() error {
	w.logger.Info("starting...", "pid", os.Getpid())
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	go func() {
		for _, cmd := range w.Cmds {
			w.newCmd(w.ctx, cmd)
		}
	}()
	for _, dir := range w.Dirs {
		err = watcher.Add(dir.Name)
		if err != nil {
			log.Fatal(err)
		}
	}
	w.EventLoop(watcher)

	var done sync.WaitGroup
	go func() {
		done.Add(1)
		defer done.Done()
		for range w.success {
			w.CloseOngoingProcesses()
			go func() {
				for _, cmd := range w.Cmds {
					w.newCmd(w.ctx, cmd)
				}
				w.HealthCheck()
				if w.Proxy.Activated {
					log.Println("DEBUG: sending event")
					select {
					case w.events <- struct{}{}:
						log.Println("DEBUG: sent event")
					case <-time.After(time.Second):
						log.Println("DEBUG: timeout event")
					}
				}
			}()
		}
	}()
	<-w.close
	err = watcher.Close()
	if err != nil {
		w.logger.Error("failed to close fsnotify watcher:", "error", err)
	}
	done.Wait()

	if w.waitProxy != nil {
		<-w.waitProxy
	}

	return nil
}
