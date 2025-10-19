package audit

import (
	"context"
	"encoding/json"
	"os"
	"sync"
)

type FileSink struct {
	path string
	mu   sync.Mutex
}

func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

func (s *FileSink) Send(_ context.Context, ev Event) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(b, '\n'))
	return err
}
