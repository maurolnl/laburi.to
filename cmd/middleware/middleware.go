// Package middleware provides HTTP middleware setup for basic middleware
package middleware

import (
	"log"
	"net/http"
	"time"
)

func CreateStack(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			middleware := middlewares[i]
			next = middleware(next)
		}

		return next
	}
}

func (w *wrapperResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writerWithStatusCode := &wrapperResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(writerWithStatusCode, r)

		log.Println(writerWithStatusCode, r.Method, r.URL.Path, time.Since(start))
	})
}
