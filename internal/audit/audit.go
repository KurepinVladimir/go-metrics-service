package audit

import (
	"context"
	"time"
)

type Event struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

type Sink interface {
	Send(ctx context.Context, ev Event) error
}

type Auditor struct {
	sinks []Sink
}

func New(sinks ...Sink) *Auditor {
	return &Auditor{sinks: sinks}
}

func (a *Auditor) Enabled() bool {
	return a != nil && len(a.sinks) > 0
}

func (a *Auditor) Notify(ctx context.Context, metrics []string, ip string, now func() time.Time) {
	if !a.Enabled() {
		return
	}
	ev := Event{
		TS:        now().Unix(),
		Metrics:   metrics,
		IPAddress: ip,
	}
	for _, s := range a.sinks {
		// best-effort: ошибки не роняют обработку
		_ = s.Send(ctx, ev)
	}
}
