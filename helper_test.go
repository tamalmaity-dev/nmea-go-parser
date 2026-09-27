package nmea_test

import (
	"context"
	"testing"
)

// testContext returns a context for benchmarks, which cannot use the
// per-iteration cancellation the tests do.
func testContext(tb testing.TB) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)
	return ctx
}
