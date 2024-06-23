package not

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func (w *Watcher) forward(writer http.ResponseWriter, r *http.Request) {
	newUrl := r.URL
	newUrl.Scheme = "http"
	newUrl.Host = fmt.Sprintf("localhost:%v", w.Proxy.PortApp)

	defer r.Body.Close()
	request, err := http.NewRequest(r.Method, newUrl.String(), r.Body)
	for k, values := range r.Header {
		for _, value := range values {
			request.Header.Add(k, value)
		}
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		w.logger.Error("request forward", "error", err, "url", newUrl.String())
		return
	}
	defer resp.Body.Close()
	for k, values := range resp.Header {
		for _, value := range values {
			writer.Header().Add(k, value)
		}
	}
	writer.WriteHeader(resp.StatusCode)
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		panic(err)
	}
}

func (w *Watcher) runProxy(ctx context.Context, waitProxy chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", w.forward)

	server := http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", w.Proxy.PortNot),
	}
	done := make(chan struct{})
	go func() {
		<-w.ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		log.Println("shutdown:", err)
		done <- struct{}{}
	}()
	go func() {
		w.logger.Info("proxy", "not", w.Proxy.PortNot, "target", w.Proxy.PortApp)
		err := server.ListenAndServe()
		log.Println("listen:", err)
	}()
	<-done
	waitProxy <- struct{}{}
}
