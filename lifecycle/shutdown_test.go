// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	ShutdownFn func(context.Context) error
)

func (fn ShutdownFn) Shutdown(ctx context.Context) error {
	return fn(ctx)
}

var (
	// A basic GracefulShutdown that delegates to ctx.Err().
	basic = ShutdownFn(func(ctx context.Context) error { return ctx.Err() })
	// A GracefulShutdown that always fail.
	errShutdown  = errors.New("Always error on shutdown")
	failShutdown = ShutdownFn(func(ctx context.Context) error { return errShutdown })
)

func TestGracefulShutdown(t *testing.T) {
	ctx := context.Background()

	assert.NoError(t, basic.Shutdown(ctx))
	assert.ErrorIs(t, failShutdown.Shutdown(ctx), errShutdown)

	ctx, cancel := context.WithCancel(ctx)
	cancel()

	assert.ErrorIs(t, basic.Shutdown(ctx), context.Canceled)
}

func TestMaybeGracefullShutdown(t *testing.T) {
	ctx := context.Background()
	assert.NoError(t, MaybeGracefulShutdown(ctx, basic))
	assert.ErrorIs(t, MaybeGracefulShutdown(ctx, failShutdown), errShutdown)

	aMap := make(map[string]string)
	assert.NoError(t, MaybeGracefulShutdown(ctx, aMap))

	ctx, cancel := context.WithCancel(ctx)
	cancel()

	assert.ErrorIs(t, MaybeGracefulShutdown(ctx, basic), context.Canceled)
	assert.ErrorIs(t, MaybeGracefulShutdown(ctx, aMap), context.Canceled)
}
