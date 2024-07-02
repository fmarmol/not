package not

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func (w *Watcher) EventLoop(watcher *fsnotify.Watcher) {
	go func() {
		defer close(w.success)
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
				if !event.Has(fsnotify.Write) {
					continue EVENTS_LOOP
				}
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
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
}
