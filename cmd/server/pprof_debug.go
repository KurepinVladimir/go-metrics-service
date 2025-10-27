//go:build !release
// +build !release

package main

import (
	"net/http"
	_ "net/http/pprof" // регистрирует /debug/pprof/*
)

// поднимем pprof на отдельном порту
func init() {
	go func() {
		_ = http.ListenAndServe("127.0.0.1:6060", nil)
	}()
}
