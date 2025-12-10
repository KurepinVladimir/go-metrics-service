package main

import (
	"context"
	"sync"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func startGRPCDispatcher(ctx context.Context, flushInterval time.Duration, jobs <-chan models.Metrics, client proto.MetricsClient, realIP string) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()

		batch := make([]*proto.Metric, 0, 128)
		flush := func() {
			if len(batch) == 0 {
				return
			}
			req := &proto.UpdateMetricsRequest{Metrics: batch}
			md := metadata.Pairs("x-real-ip", realIP)
			_, err := client.UpdateMetrics(metadata.NewOutgoingContext(ctx, md), req)
			if err != nil {
				logger.Log.Error("grpc batch send failed", zap.Error(err))
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case m, ok := <-jobs:
				if !ok {
					flush()
					return
				}
				if pm := toProtoMetric(m); pm != nil {
					batch = append(batch, pm)
				}
			case <-ticker.C:
				flush()
			}
		}
	}()

	return &wg
}

func toProtoMetric(m models.Metrics) *proto.Metric {
	switch m.MType {
	case models.Gauge:
		return &proto.Metric{Id: m.ID, Type: proto.Metric_GAUGE, Value: valueOrZero(m.Value)}
	case models.Counter:
		return &proto.Metric{Id: m.ID, Type: proto.Metric_COUNTER, Delta: deltaOrZero(m.Delta)}
	default:
		return nil
	}
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func deltaOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
