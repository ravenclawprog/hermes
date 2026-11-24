package api

import (
	"log"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	if w.status == 0 && status >= 200 {
		w.status = status
	}
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("request %s route=%q status=%d duration=%s", r.RemoteAddr, route, status, time.Since(start).Round(time.Microsecond))
	})
}
