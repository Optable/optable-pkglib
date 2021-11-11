package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type (
	// Servable is implemented by components that listen and serve requests. The
	// most notable type implementing this are http.Server and grpc.Server.
	Servable interface {
		Serve(net.Listener) error
	}
)

// ServeWithGracefulShutdown glue a Servable with a proper shutdown routine.
// register signals to trigger a proper shutdown sequence. This function does
// not block and returns immediately a channel where an error will be emitted
// if it failed to serve, or the returned shutdown error (or nil if none).
func ServeWithGracefulShutdown(ctx context.Context, listen net.Listener, server Servable, shutdownTimeout time.Duration) <-chan error {
	ctx, cancel := context.WithCancel(ctx)

	logger := zerolog.Ctx(ctx)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	shutdownCompleted := make(chan error, 1)
	go func() {
		defer close(shutdownCompleted)
		defer logger.Info().Msg("Shutdown sequence completed")

		var sig os.Signal
		select {
		case <-ctx.Done():
			logger.Info().Msg("Shutdown triggered by context cancellation")
		case sig = <-signals:
			logger.Info().Str("signal", sig.String()).Msgf("Shutdown triggered by signal: %s", sig)
		}

		ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()

		if err := MaybeGracefulShutdown(ctx, server); err != nil {
			shutdownCompleted <- fmt.Errorf("Unclean shutdown of grpc server: %w", err)
			return
		}
	}()

	go func() {
		if err := server.Serve(listen); err != nil {
			cancel()
			shutdownCompleted <- fmt.Errorf("Server failed to listen: %w", err)
		}
	}()

	return shutdownCompleted
}

// ServeGrpcAndMetrics behaves like ServeWithGracefulShutdown excepts that it
// also starts a prometheus HTTP1 service on the same Listener to expose
// metrics.
func ServeGrpcAndMetrics(ctx context.Context, l net.Listener, server *grpc.Server, shutdownTimeout time.Duration) <-chan error {
	errs := make(chan error, 1)

	go func() {
		mux := cmux.New(l)
		httpL := mux.Match(cmux.HTTP1Fast())
		grpcL := mux.Match(cmux.Any())

		defer close(errs)
		defer mux.Close()

		group, ctx := errgroup.WithContext(ctx)

		// Serve requests for the gRPC service.
		group.Go(func() error {
			if err := <-ServeWithGracefulShutdown(ctx, grpcL, server, shutdownTimeout); err != nil && !isClosedErr(err) {
				return fmt.Errorf("Failed serving grpc: %w", err)
			}

			// When the grpc service shutdowns, it closes the net.Listener given by
			// cmux which will also closes all net.Listener derived. Thus cascading
			// other worker functions to exit.
			return nil
		})

		// Serve requests for the prometheus HTTP metric handler.
		group.Go(func() error {
			httpServer := &http.Server{
				Handler: promhttp.Handler(),
			}
			if err := httpServer.Serve(httpL); err != nil && !isClosedErr(err) {
				return fmt.Errorf("Failed serving http: %w", err)
			}
			return nil
		})

		// Serve routing the listener
		group.Go(func() error {
			if err := mux.Serve(); err != nil && !isClosedErr(err) {
				return fmt.Errorf("Failed serving mux: %w", err)
			}
			return nil
		})

		// The errgroup.Group will stop on 2 conditions:
		//
		// - When any error is encountered, this cascaded in a context cancellation
		//   for the other workers.
		//
		// - When the grpc service gracefully shutdowns, it closes the listener
		//   which will make the other workers to receive the ErrClosed error.
		errs <- group.Wait()
	}()

	return errs
}

func isClosedErr(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, cmux.ErrServerClosed)
}
