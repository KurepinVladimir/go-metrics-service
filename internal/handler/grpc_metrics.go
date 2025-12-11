package handler

import (
	"context"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/audit"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/proto"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCMetricsServer implements proto.MetricsServer and applies updates to Storage.
type GRPCMetricsServer struct {
	proto.UnimplementedMetricsServer
	Storage repository.Storage
	Auditor *audit.Auditor
}

func (s *GRPCMetricsServer) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil || len(req.Metrics) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	batch := make([]models.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		if m == nil {
			continue
		}
		switch m.Type {
		case proto.Metric_GAUGE:
			val := m.Value
			batch = append(batch, models.Metrics{ID: m.Id, MType: models.Gauge, Value: &val})
		case proto.Metric_COUNTER:
			delta := m.Delta
			batch = append(batch, models.Metrics{ID: m.Id, MType: models.Counter, Delta: &delta})
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown metric type")
		}
	}
	if len(batch) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty batch")
	}

	if bu, ok := s.Storage.(repository.BatchUpdater); ok {
		if err := bu.UpdateBatch(ctx, batch); err != nil {
			return nil, status.Error(codes.Internal, "storage error")
		}
	} else {
		for _, m := range batch {
			switch m.MType {
			case models.Gauge:
				if m.Value == nil {
					return nil, status.Error(codes.InvalidArgument, "missing gauge value")
				}
				s.Storage.UpdateGauge(ctx, m.ID, *m.Value)
			case models.Counter:
				if m.Delta == nil {
					return nil, status.Error(codes.InvalidArgument, "missing counter delta")
				}
				s.Storage.UpdateCounter(ctx, m.ID, *m.Delta)
			default:
				return nil, status.Error(codes.InvalidArgument, "unknown metric type")
			}
		}
	}

	names := make([]string, 0, len(batch))
	for _, m := range batch {
		names = append(names, m.ID)
	}

	if s.Auditor != nil && s.Auditor.Enabled() {
		ip := metadataIP(ctx)
		s.Auditor.Notify(ctx, names, ip)
	}

	return &proto.UpdateMetricsResponse{}, nil
}

func metadataIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("x-real-ip")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
