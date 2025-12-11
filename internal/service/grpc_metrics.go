package service

import (
	"context"
	"net"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/proto"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetricsServer реализует gRPC-сервис Metrics.
type MetricsServer struct {
	proto.UnimplementedMetricsServer
	storage repository.Storage
}

// NewMetricsServer создаёт сервер с переданным хранилищем.
func NewMetricsServer(storage repository.Storage) *MetricsServer {
	return &MetricsServer{storage: storage}
}

// UpdateMetrics принимает батч метрик и сохраняет их в storage.
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil || len(req.Metrics) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty metrics batch")
	}

	// Если хранилище умеет батчи — используем их.
	if bu, ok := s.storage.(repository.BatchUpdater); ok {
		batch := make([]models.Metrics, 0, len(req.Metrics))
		for _, m := range req.Metrics {
			var mt string
			switch m.Type {
			case proto.Metric_GAUGE:
				mt = models.Gauge
			case proto.Metric_COUNTER:
				mt = models.Counter
			default:
				return nil, status.Error(codes.InvalidArgument, "unknown metric type")
			}

			var delta *int64
			var value *float64
			// Для простоты: если тип gauge — используем value; если counter — delta.
			if mt == models.Gauge {
				v := m.Value
				value = &v
			} else {
				d := m.Delta
				delta = &d
			}

			batch = append(batch, models.Metrics{
				ID:    m.Id,
				MType: mt,
				Delta: delta,
				Value: value,
			})
		}

		if err := bu.UpdateBatch(ctx, batch); err != nil {
			return nil, status.Error(codes.Internal, "storage batch error")
		}
		return &proto.UpdateMetricsResponse{}, nil
	}

	// Фолбэк: поштучно.
	for _, m := range req.Metrics {
		switch m.Type {
		case proto.Metric_GAUGE:
			s.storage.UpdateGauge(ctx, m.Id, m.Value)
		case proto.Metric_COUNTER:
			s.storage.UpdateCounter(ctx, m.Id, m.Delta)
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown metric type")
		}
	}

	return &proto.UpdateMetricsResponse{}, nil
}

// trustedSubnetInterceptor проверяет, что x-real-ip ∈ trustedNet.
func trustedSubnetInterceptor(trustedNet *net.IPNet) grpc.UnaryServerInterceptor {
	// Если подсеть не задана — просто пропускаем запросы.
	if trustedNet == nil {
		return func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}

		ips := md.Get("x-real-ip")
		if len(ips) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}

		ip := net.ParseIP(ips[0])
		if ip == nil || !trustedNet.Contains(ip) {
			return nil, status.Error(codes.PermissionDenied, "forbidden subnet")
		}

		return handler(ctx, req)
	}
}

// NewGRPCServer создаёт *grpc.Server с нужным интерцептором и регистрирует сервис.
func NewGRPCServer(storage repository.Storage, trustedNet *net.IPNet) *grpc.Server {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(trustedSubnetInterceptor(trustedNet)),
	)

	proto.RegisterMetricsServer(s, NewMetricsServer(storage))

	return s
}
