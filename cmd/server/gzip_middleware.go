package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer       *gzip.Writer
	wroteHeaders bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if !w.wroteHeaders {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.wroteHeaders = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeaders {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(b)
}

// Реализуем http.Flusher
func (w *gzipResponseWriter) Flush() {
	// Сбрасываем буфер самого gzip.Writer
	_ = w.writer.Flush()
	// Если базовый RW поддерживает Flusher — тоже пробросим
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Проксируем Hijacker (если нужен апгрейд соединения)
func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hj.Hijack()
}

// Проксируем HTTP/2 Pusher (если доступен)
func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func gzipResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Если клиент не просит gzip — отдаём как есть
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		// Не сжимаем повторно
		if w.Header().Get("Content-Encoding") != "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")

		gzw := gzipPool.Get().(*gzip.Writer)
		gzw.Reset(w)
		defer func() {
			_ = gzw.Close()
			gzw.Reset(io.Discard)
			gzipPool.Put(gzw)
		}()

		grw := &gzipResponseWriter{
			ResponseWriter: w,
			writer:         gzw,
		}
		next.ServeHTTP(grw, r)
	})
}

// Распаковка входящих gzip-запросов
func gzipRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "failed to read gzip body", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = io.NopCloser(gr)
		}
		next.ServeHTTP(w, r)
	})
}
