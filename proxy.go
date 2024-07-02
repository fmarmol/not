package not

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"
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

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println("OLD LENGTH:", len(content))
	n := bytes.LastIndex(content, []byte("</body>"))
	if n == -1 {
		writer.WriteHeader(resp.StatusCode)
		writer.Write(content)
		return
	}
	// strings.LastIndex()

	// _, err = io.Copy(writer, resp.Body)
	// if err != nil {
	// 	panic(err)
	// }
	// script := `let souce = "e" `

	script := new(bytes.Buffer)
	err = Sse().Render(r.Context(), script)
	if err != nil {
		panic(err)
	}

	newContent := slices.Concat(content[:n], script.Bytes(), content[n:])
	fmt.Println("NEW LENGTH:", len(newContent))
	writer.Header().Set("Content-Length", strconv.Itoa(len(newContent)))
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	// writer.Header().Set("Content-Security-Policy", "default-src 'self';script-src 'self' https://*; style-src 'self' https://*")
	writer.WriteHeader(resp.StatusCode)
	// newContent := content
	writer.Write(newContent)
}

func (watcher *Watcher) runProxy(ctx context.Context, waitProxy chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inject", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for range watcher.events {
			log.Println("DEBUG TRY TO SEND an event data")
			// w.Write([]byte("event: ping\n"))
			w.Write([]byte("data: 1\n\n"))
			flusher, ok := w.(http.Flusher)
			if !ok {
				panic("here")
			}
			flusher.Flush()
			log.Println("DEBUG END TRY TO SEND an event data")
		}
	})
	mux.HandleFunc("/", watcher.forward)

	server := http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", watcher.Proxy.PortNot),
	}
	done := make(chan struct{})
	go func() {
		<-watcher.ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		log.Println("shutdown:", err)
		done <- struct{}{}
	}()
	go func() {
		watcher.logger.Info("proxy", "not", watcher.Proxy.PortNot, "target", watcher.Proxy.PortApp)
		err := server.ListenAndServe()
		log.Println("listen:", err)
	}()
	<-done
	waitProxy <- struct{}{}
}
