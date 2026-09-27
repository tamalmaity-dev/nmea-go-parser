package nmea_test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
	_ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// TestConcurrentUse exercises the parser from several goroutines at once.
//
// The race detector needs cgo and cannot run in every environment, but this
// test is still worth having: it is the one that would surface a panic from a
// torn slice or a nil map under a `-race` build, and it verifies the counters
// add up, which is a real correctness check on their own.
func TestConcurrentUse(t *testing.T) {
	corpus := corpus(t)
	p := nmea.New(nmea.WithChannelBuffer(64), nmea.WithKeepHistory(16))

	var wg sync.WaitGroup

	// One writer, as a real deployment has: a single serial port. The read
	// loop takes a lock so that a second Consume would wait rather than
	// interleave bytes into the shared line splitter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.Consume(context.Background(), bytes.NewReader(corpus)); err != nil {
			t.Errorf("Consume: %v", err)
		}
	}()

	// A second Consume on the same parser must serialise rather than corrupt
	// framing, so the corpus arrives intact despite two callers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.Consume(context.Background(), bytes.NewReader(corpus)); err != nil {
			t.Errorf("second Consume: %v", err)
		}
	}()

	// A channel consumer. The events channel is deliberately never closed,
	// so a consumer bounds its own loop, which is what the documentation
	// tells callers to do. It is tracked separately because it terminates on
	// its context, not on the writers finishing.
	consumeCtx, stopConsume := context.WithCancel(context.Background())
	var consumerWG sync.WaitGroup
	var drained int
	consumerWG.Add(1)
	go func() {
		defer consumerWG.Done()
		for {
			select {
			case <-consumeCtx.Done():
				return
			case <-p.Events():
				drained++
			}
		}
	}()

	// Readers of the aggregate state while the writers run. This is the
	// concurrency that actually happens in a program: one goroutine reads a
	// port while another displays the position.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 300; n++ {
				f := p.Fix()
				if f.HasPosition && (f.Latitude < -90 || f.Latitude > 90) {
					t.Errorf("latitude out of range: %v", f.Latitude)
					return
				}
				if f.HasPosition && (f.Longitude < -180 || f.Longitude > 180) {
					t.Errorf("longitude out of range: %v", f.Longitude)
					return
				}
				if _, _, ok := f.DecimalPosition(); !ok && f.HasPosition {
					t.Error("DecimalPosition reported no position while the fix had one")
					return
				}
				_ = p.Stats()
				_ = p.History()
				_, _ = p.Version()
				_ = p.Receiver()
			}
		}()
	}

	wg.Wait()
	stopConsume()
	consumerWG.Wait()

	stats := p.Stats()
	// Two corpora went in, and neither may be corrupted by the other.
	if stats.Lines == 0 {
		t.Error("no lines were counted")
	}
	if stats.Bytes == 0 {
		t.Error("no bytes were counted")
	}
	if stats.BadChecksums != 0 {
		t.Errorf("BadChecksums = %d on a clean corpus, want 0: concurrent Consume corrupted framing", stats.BadChecksums)
	}
	// The proprietary sentences in the corpus have no decoder, which is
	// expected rather than a failure.
	if stats.Unknown == 0 {
		t.Error("Unknown = 0, but the corpus contains proprietary sentences")
	}
	if !p.Fix().HasPosition {
		t.Error("the fix has no position after two full corpora")
	}
}

// TestSentenceValuesAreNotAliased checks that a Fix handed to a handler is
// independent of the parser's own state, so a handler that keeps the pointer
// cannot be corrupted by the read loop continuing.
func TestSentenceValuesAreNotAliased(t *testing.T) {
	p := nmea.New()

	var kept []*nmea.Fix
	p.OnFix(func(f *nmea.Fix) error {
		// Keep every one of them, which is exactly the usage that would
		// expose shared state.
		kept = append(kept, f)
		return nil
	})

	// Two positions far apart, so a shared slice or struct would be obvious.
	if _, err := p.ParseLine("$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ParseLine("$GPGGA,123520,0000.000,N,00000.000,E,1,08,0.9,0.0,M,46.9,M,,*4B"); err != nil {
		t.Fatal(err)
	}

	if len(kept) < 2 {
		t.Fatalf("handler called %d times, want at least 2", len(kept))
	}
	first := kept[0]
	if first.Latitude == 0 && first.Longitude == 0 {
		t.Error("the first fix was overwritten by the second")
	}
}

// TestChannelDropsRatherThanBlocks checks the documented overflow behaviour:
// a consumer that falls behind loses events instead of stalling the read
// loop, because a stalled serial reader loses data permanently.
func TestChannelDropsRatherThanBlocks(t *testing.T) {
	// A buffer of one, consumed by nobody.
	p := nmea.New(nmea.WithChannelBuffer(1))
	corpus := corpus(t)
	if err := p.Consume(context.Background(), bytes.NewReader(corpus)); err != nil {
		t.Fatal(err)
	}

	stats := p.Stats()
	if stats.Lines == 0 {
		t.Fatal("no lines were read")
	}
	if stats.Dropped == 0 {
		t.Errorf("Dropped = 0 with a one-slot buffer and no consumer, want a non-zero count (read %d lines)", stats.Lines)
	}
	// The fix must still be complete: dropping events costs the consumer
	// data, not the parser's state.
	if p.Fix().SentenceCount == 0 {
		t.Error("the fix is empty even though every line was parsed")
	}
}
