package service

import (
	"context"
	"errors"
	"fmt"

	metrics "github.com/grpc-ecosystem/go-grpc-middleware/providers/openmetrics/v2"
	grpczerolog "github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// NewGRPCService creates a grpc service with various defaults middlewares.
// Notably, the logging and metrics are automatically registered for sane
// defaults of observability.
func NewGRPCService(ctx context.Context, service interface{}, descriptors []*grpc.ServiceDesc, unaryIntercepts []grpc.UnaryServerInterceptor, streamIntercepts []grpc.StreamServerInterceptor) (*grpc.Server, error) {
	if len(descriptors) == 0 {
		return nil, errors.New("Missing descriptors")
	}
	// By using prometheus.DefaultRegister we benefits from the go runtime
	// defaults metrics and Linux processes metrics.
	registry := prometheus.DefaultRegisterer

	m := metrics.NewRegisteredServerMetrics(registry, metrics.WithServerHandlingTimeHistogram())
	if collector, ok := service.(prometheus.Collector); ok {
		if err := registry.Register(collector); err != nil {
			return nil, fmt.Errorf("Failed registering metrics: %w", err)
		}
	}

	logger := zerolog.Ctx(ctx)
	defaultStreamInterceptors := []grpc.StreamServerInterceptor{
		logging.StreamServerInterceptor(grpczerolog.InterceptorLogger(*logger)),
		metrics.StreamServerInterceptor(m),
		recovery.StreamServerInterceptor(),
	}
	defaultUnaryInterceptors := []grpc.UnaryServerInterceptor{
		logging.UnaryServerInterceptor(grpczerolog.InterceptorLogger(*logger)),
		metrics.UnaryServerInterceptor(m),
		recovery.UnaryServerInterceptor(),
	}

	defaultUnaryInterceptors = append(defaultUnaryInterceptors, unaryIntercepts...)
	defaultStreamInterceptors = append(defaultStreamInterceptors, streamIntercepts...)

	server := grpc.NewServer(grpc.ChainStreamInterceptor(defaultStreamInterceptors...), grpc.ChainUnaryInterceptor(defaultUnaryInterceptors...))

	for _, desc := range descriptors {
		logger.Info().Msgf("Registering grpc service: %s", desc.ServiceName)
		server.RegisterService(desc, service)
	}

	// Ensure that all metrics for all endpoints are default to NULL instead of
	// being lazily added to the metrics the first time an endpoint is hit.
	//
	// This must be called once all gRPC services are registered.
	m.InitializeMetrics(server)

	return server, nil
}

func WithDescriptors(descs ...*grpc.ServiceDesc) []*grpc.ServiceDesc {
	return descs
}
