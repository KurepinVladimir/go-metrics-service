package middleware

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TrustedSubnetUnaryInterceptor validates that the x-real-ip metadata belongs to the trustedNet.
// If trustedNet is nil, the interceptor lets requests pass without checks.
func TrustedSubnetUnaryInterceptor(trustedNet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if trustedNet == nil {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}
		ips := md.Get("x-real-ip")
		if len(ips) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}
		ipStr := strings.TrimSpace(ips[0])
		ip := net.ParseIP(ipStr)
		if ip == nil || !trustedNet.Contains(ip) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}

		return handler(ctx, req)
	}
}
