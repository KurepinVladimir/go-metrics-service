package pool_test

import (
	"testing"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/pool"
)

type testS struct {
	S []int
	M map[string]int
}

func (s *testS) Reset() {
	if s == nil {
		return
	}
	if s.S != nil {
		s.S = s.S[:0]
	}
	clear(s.M)
}

func TestPoolStruct(t *testing.T) {
	p := pool.New(func() *testS { return &testS{M: make(map[string]int)} })

	obj := p.Get()
	obj.S = append(obj.S, 1, 2, 3)
	obj.M["x"] = 42
	p.Put(obj) // Reset()

	obj2 := p.Get()
	if len(obj2.S) != 0 {
		t.Fatalf("slice not reset, len=%d", len(obj2.S))
	}
	if len(obj2.M) != 0 {
		t.Fatalf("map not cleared, len=%d", len(obj2.M))
	}
	p.Put(obj2)
}
