package not

import "time"

func (w *Watcher) HealthCheck() {
	time.Sleep(time.Second)
}
