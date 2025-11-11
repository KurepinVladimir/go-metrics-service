package pool_test

import (
	"bytes"
	"testing"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/pool"
)

func TestPoolBytesBuffer(t *testing.T) {
	p := pool.New(func() *bytes.Buffer { return &bytes.Buffer{} })

	// берём, используем
	b := p.Get()
	b.WriteString("hello")
	if b.Len() == 0 {
		t.Fatalf("expected buffer to have data")
	}

	// кладём назад — Pool вызывает Reset()
	p.Put(b)

	// берём снова — тот же буфер, но пустой
	b2 := p.Get()
	if b2.Len() != 0 {
		t.Fatalf("expected empty buffer after Reset, got len=%d", b2.Len())
	}
	// и вернули
	p.Put(b2)
}
