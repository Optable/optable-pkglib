// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package lifecycle

import "context"

type (
	// GracefulShutdown is implemented by components that need to be gracefully
	// terminated, e.g. flushing buffers to permanent storage. Shutdown is
	// traditionally done in multiple fashions:
	//
	//   1. By passing a context to the builder object, e.g. `NewGizmo(ctx, ...)`.
	//      and cancelling the context to stop internal state (goroutines).
	//   2. By providing a `Close() error` method.
	//
	// The first method suffers from multiple issues:
	//  - No signal *when* the component is done shutting down.
	//  - No success status on the shutdown.
	// The second method (and also the first) suffers from indeterminate timeout;
	// how long do we wait for Close to return?
	//
	// By implementing the GracefulShutdown interface, a component or a service, can
	// implement graceful shutdown and have parent (main process) know about the
	// lifecycle of the components. It also ties nicely with Kubernetes' via
	// `terminationGracePeriodSeconds`.
	GracefulShutdown interface {
		// Shutdown context should be respected.
		Shutdown(context.Context) error
	}
)

// MaybeGracefulShutdown takes an object and invokes Shutdown if the object
// implements GracefulShutdown. This function exists to avoid forcing every
// interface to also implement this.
//
// This is often useful when builder functions, e.g. NewX() -> X returns
// an interface where not all implementations implements GracefulShutdown.
// It avoids leaking the GracefulShutdown in all interface.
func MaybeGracefullShutdown(ctx context.Context, i interface{}) error {
	if s, ok := i.(GracefulShutdown); ok {
		return s.Shutdown(ctx)
	}

	return ctx.Err()
}
