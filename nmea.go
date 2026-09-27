// Package nmea is a golang parser for NMEA 0183 sentences from any
// io.Reader: a serial port, a TCP socket, a log file, or a byte buffer of
// recorded data.
//
// It decodes the formatters receivers actually emit — GGA, RMC, GLL, VTG,
// GSA, GSV, GNS, ZDA and the rest — into typed Go values, and merges them
// into a single position fix. Vendor-proprietary sentences can be supported
// by registering a decoder of your own.
//
// # Design
//
// The code is split so a project can use it at whatever level of ceremony it
// needs:
//
//   - The root package handles framing, checksums, field splitting, the
//     decoder registry, and the aggregate Fix. It contains no knowledge of
//     any individual sentence.
//   - The sentences subpackage holds one decoder per sentence formatter and
//     registers itself with the root registry on import.
//   - The device subpackage opens a serial port and wires it to a Parser.
//
// The root package never imports the sentences package, so the dependency
// runs one way and a program can substitute or drop decoders freely.
//
// # Getting started
//
// Import the sentences package for its side effects, then point a Parser at
// a reader:
//
//	import (
//	    "github.com/tamalmaity-dev/nmea-go-parser"
//	    _ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
//	)
//
//	p := nmea.New(nmea.WithOnFix(func(f *nmea.Fix) error {
//	    log.Printf("lat=%v lon=%v", f.Latitude, f.Longitude)
//	    return nil
//	}))
//	go p.Consume(ctx, port)
//
// For a serial port, use the device package instead of opening the port
// yourself. Importing device already brings in every built-in decoder:
//
//	dev, err := device.Open(device.Config{Port: "COM3", BaudRate: 9600})
//	defer dev.Close()
//	go dev.Consume(ctx)
//
// # Reading the position
//
// Fix is the merged current state. Poll it from your own goroutine with
// Parser.Fix, or take the OnFix callback. Every optional field has a Has*
// flag, because a blank NMEA field means "not available", which is not the
// same as zero: latitude 0.0 and longitude 0.0 are real positions.
//
// # Errors
//
// Every error wraps a sentinel from this package, so test with errors.Is
// rather than by matching strings. A handler may return an error to report
// a problem; it is routed to the error handlers and never stops the stream.
package nmea

import (
	"context"
	"errors"
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"github.com/tamalmaity-dev/nmea-go-parser/value"
	"io"
	"reflect"
	"sync"
	"time"
)

// Event is the result of parsing one sentence, delivered to handlers and to
// the channel.
//
// A sentence that fails to decode still carries its raw text, so a failing
// sentence can be logged or forwarded rather than silently lost. A checksum
// mismatch is the one case where both Value and Err are set: the checksum is
// wrong but the payload is usually still readable, so it is handed over
// alongside the warning.
type Event struct {
	// Sentence is the framed sentence, including the checksum verdict.
	Sentence Sentence
	// Value is the decoded sentence, e.g. a *sentences.GGA. It is nil when
	// decoding failed or the formatter has no registered decoder.
	Value any
	// Err is the failure, if any.
	Err error
	// Time is when the sentence was parsed.
	Time time.Time
}

// Type returns the sentence formatter, e.g. "GGA", or "" if the sentence
// was never framed.
func (e Event) Type() string { return e.Sentence.Type }

// Address returns talker plus formatter, e.g. "GNGGA".
func (e Event) Address() string { return e.Sentence.Address() }

// OK reports whether the sentence decoded without error. A bad checksum
// makes OK false even though Value is populated.
func (e Event) OK() bool { return e.Err == nil }

// As copies the decoded value into a target pointer, which lets a handler
// be written once and shared between sentence types:
//
//	var gga sentences.GGA
//	if err := ev.As(&gga); err != nil {
//	    return err // different sentence type, or a failed decode
//	}
func (e Event) As(target any) error {
	if target == nil {
		return errors.New("nmea: As needs a non-nil target")
	}
	tv := reflect.ValueOf(target)
	if tv.Kind() != reflect.Pointer || tv.IsNil() {
		return fmt.Errorf("nmea: As target must be a non-nil pointer, got %T", target)
	}
	if e.Value == nil {
		if e.Err != nil {
			return e.Err
		}
		return fault.ErrNotValid
	}
	want := tv.Type().Elem()
	v := reflect.ValueOf(e.Value)
	// Accept both *GGA and GGA from a decoder, since either is a
	// reasonable thing for a custom decoder to return. The value itself is
	// dereferenced, not just its type: Set needs a GGA to put into a GGA,
	// and a *GGA would panic.
	if v.Type() != want {
		if v.Kind() == reflect.Pointer && v.Type().Elem() == want {
			if v.IsNil() {
				return fmt.Errorf("nmea: event holds a nil *%s", want)
			}
			v = v.Elem()
		} else {
			return fmt.Errorf("nmea: event holds %s, cannot use as %s", v.Type(), want)
		}
	}
	tv.Elem().Set(v)
	return nil
}

// Stats is a snapshot of parser counters, useful for health checks and for
// diagnosing a stream that is not behaving.
type Stats struct {
	// Sentences counts decoded sentences that carried positional or
	// satellite data and therefore updated the fix.
	Sentences uint64
	// BadChecksums counts sentences whose *hh checksum did not verify. A
	// steady climb usually means the wrong baud rate.
	BadChecksums uint64
	// MissingChecksums counts sentences sent without a checksum at all,
	// which is only an error under WithRequireChecksum.
	MissingChecksums uint64
	// Unknown counts formatters with no registered decoder. A non-zero
	// value is normal if the receiver emits sentences the library does not
	// implement.
	Unknown uint64
	// Dropped counts events discarded because the channel buffer was full.
	Dropped uint64
	// Overflows counts lines discarded for exceeding the length limit,
	// which means the stream lost framing. The count is owned by the line
	// splitter, which is where the decision to drop is made, so Stats reads
	// it rather than maintaining a second copy that could disagree.
	Overflows uint64
	// Bytes counts bytes read from the source.
	Bytes uint64
	// Lines counts complete NMEA lines pulled off the wire.
	Lines uint64
}

// Parser turns a byte stream into typed sentences and a merged Fix.
//
// A Parser is safe to read while it runs: Fix, Stats, and the handler
// registration methods may be called from any goroutine. Configure it first,
// though. Registering handlers or decoders while a read loop is running is a
// data race, because the handler slices and the registry are read on the read
// loop without a lock. Set everything up before calling Consume, ReadFrom, or
// Feed.
//
// Handlers run on whichever goroutine is feeding the parser, so a slow
// handler delays reading; move slow work to your own goroutine.
type Parser struct {
	registry *Registry
	splitter *LineSplitter
	now      func() time.Time
	detector *VersionDetector
	revision VersionInfo

	strictChecksum  bool
	requireChecksum bool
	readBufSize     int
	chanBuf         int
	historySize     int

	onFix   []func(*Fix) error
	onEvent []func(Event) error
	onError []func(error)
	byType  map[string][]func(Event) error

	mu     sync.RWMutex
	fix    Fix
	events chan Event

	// consumeMu serialises the read loop. The line splitter is stateful, so
	// two Consume calls on one parser would interleave their bytes and
	// corrupt sentence framing. Serialising is friendlier than documenting
	// the hazard: a second caller waits rather than silently producing
	// garbage, and a program merging two sources still works.
	consumeMu sync.Mutex

	history []Event
	// historyHead is the index of the oldest event once history is full. It
	// stays zero while the buffer is still filling.
	historyHead int
	stats       Stats
}

// New creates a Parser.
//
// With no options it decodes every sentence the sentences package
// registered, reports bad checksums without discarding their payload, keeps
// no history, and infers the receiver's standard version from the traffic.
func New(opts ...Option) *Parser {
	p := &Parser{
		registry:    DefaultRegistry.Clone(),
		splitter:    NewLineSplitter(DefaultMaxLineLength),
		now:         time.Now,
		detector:    NewVersionDetector(Version{}),
		revision:    VersionInfo{},
		readBufSize: 1024,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	if p.chanBuf > 0 {
		p.events = make(chan Event, p.chanBuf)
	}
	if p.historySize > 0 {
		p.history = make([]Event, 0, p.historySize)
	}
	return p
}

// Registry exposes the parser's decoders, so a program can register a
// vendor-specific sentence formatter or override a built-in one.
func (p *Parser) Registry() *Registry { return p.registry }

// Detector returns the version detector, which infers which revision of the
// standard the receiver speaks from the shape of the sentences it emits.
// Most receivers never state their version, so this is usually how a program
// finds out.
func (p *Parser) Detector() *VersionDetector { return p.detector }

// Version returns the receiver's standard version, and whether the receiver
// stated it or it was inferred from the traffic.
func (p *Parser) Version() (v Version, reported bool) { return p.detector.Version() }

// Receiver describes the receiver: manufacturer, product, software, and
// whatever the version detector has worked out. A receiver that never sends
// VER leaves most of this empty, with the version marked as inferred.
func (p *Parser) Receiver() VersionInfo {
	p.mu.RLock()
	info := p.revision
	info.Sentences = append([]string(nil), p.revision.Sentences...)
	p.mu.RUnlock()

	v, reported := p.detector.Version()
	info.ProtocolVersion, info.HasProtocolVersion = v, reported
	return info
}

// Use adds decoders to this parser. An argument may be a Decoder, a
// *Registry to copy wholesale, or a mix of both, which makes swapping the
// whole decoder set a one-liner.
func (p *Parser) Use(items ...any) {
	for _, it := range items {
		switch v := it.(type) {
		case Decoder:
			p.registry.Register(v)
		case *Registry:
			if v != nil {
				p.registry = v.Clone()
			}
		}
	}
}

// OnFix registers a handler called for every sentence that updates the fix.
// This is the primary hook: a complete position report arrives here.
//
// The handler runs on the read loop, so keep it short or hand work to your
// own goroutine. Returning an error reports a problem to the error handlers
// and does not stop the stream.
func (p *Parser) OnFix(fn func(*Fix) error) {
	if fn != nil {
		p.onFix = append(p.onFix, fn)
	}
}

// OnEvent registers a handler called for every framed sentence, before the
// fix is updated. Event.Value holds the decoded value, so this is where
// per-sentence logging belongs.
func (p *Parser) OnEvent(fn func(Event) error) {
	if fn != nil {
		p.onEvent = append(p.onEvent, fn)
	}
}

// OnError registers a handler for bad checksums, unknown sentences, and
// decode failures. It is not called for non-NMEA noise on the line.
func (p *Parser) OnError(fn func(error)) {
	if fn != nil {
		p.onError = append(p.onError, fn)
	}
}

// On registers a handler for one formatter, e.g. "RMC". It is called only
// for sentences of that type that decoded successfully.
func (p *Parser) On(typ string, fn func(Event) error) {
	if fn == nil || typ == "" {
		return
	}
	if p.byType == nil {
		p.byType = make(map[string][]func(Event) error)
	}
	p.byType[typ] = append(p.byType[typ], fn)
}

// OnValue registers a handler for one formatter that receives the decoded
// value already checked against a type, so the handler never has to
// type-assert and a shared callback can serve several sentence types.
//
//	p.OnValue("GGA", &sentences.GGA{}, func(v any) error {
//	    g := v.(*sentences.GGA)
//	    return log.Print(g.Latitude.MustDecimal())
//	})
//
// The target is only used for its type: a pointer element is unwrapped, so
// both sentences.GGA and *sentences.GGA work. If a decoded value does not
// match, the handler is simply not called, which means one OnValue per
// formatter is safe regardless of what else the stream carries.
//
// This is the reflection-based primitive. sentences.Watch is the generic
// version of the same idea and needs no reflection, so prefer it when the
// callback can name its type:
//
//	sentences.Watch(p, "GGA", func(g sentences.GGA) error {
//	    return log.Print(g.Latitude)
//	})
func (p *Parser) OnValue(typ string, target any, fn func(any) error) {
	if fn == nil || target == nil {
		return
	}
	want := reflect.TypeOf(target)
	for want != nil && want.Kind() == reflect.Pointer {
		want = want.Elem()
	}
	if want == nil {
		return
	}

	// A decoder may return either T or *T, so both are accepted.
	asPointer := reflect.PointerTo(want)

	p.On(typ, func(ev Event) error {
		switch got := reflect.TypeOf(ev.Value); {
		case got == want:
			return fn(ev.Value)
		case got == asPointer:
			return fn(ev.Value)
		default:
			return nil
		}
	})
}

// Fix returns the current merged fix as a copy, safe to read while the
// parser keeps running. The satellite slice is copied too, so mutating the
// result cannot corrupt parser state.
func (p *Parser) Fix() Fix {
	p.mu.RLock()
	defer p.mu.RUnlock()
	f := p.fix
	// The two satellite slices are the only reference types in a Fix, and both
	// are copied so a Fix held by a caller cannot change under them. The
	// per-GSA list needs a second level of copy because each entry owns its own
	// slice of satellite numbers.
	if f.Satellites != nil {
		f.Satellites = make([]Satellite, len(p.fix.Satellites))
		copy(f.Satellites, p.fix.Satellites)
	}
	if f.SatelliteUses != nil {
		uses := make([]SatelliteUse, len(p.fix.SatelliteUses))
		for i, u := range p.fix.SatelliteUses {
			uses[i] = u
			uses[i].IDs = append([]int(nil), u.IDs...)
		}
		f.SatelliteUses = uses
	}
	if f.satelliteGroups != nil {
		groups := make([]satelliteGroup, len(p.fix.satelliteGroups))
		for i, g := range p.fix.satelliteGroups {
			groups[i] = g
			groups[i].Satellites = append([]Satellite(nil), g.Satellites...)
		}
		f.satelliteGroups = groups
	}
	return f
}

// Position is a shorthand for the current latitude and longitude in signed
// decimal degrees. It reports false until the receiver has produced a
// position.
func (p *Parser) Position() (lat, lon float64, ok bool) {
	f := p.Fix()
	return f.Latitude, f.Longitude, f.HasPosition
}

// Stats returns a snapshot of the parser's counters.
func (p *Parser) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := p.stats
	s.Overflows = uint64(p.splitter.Overflows())
	return s
}

// Events returns the channel of decoded sentences, or nil unless
// WithChannelBuffer was given.
//
// The channel is never closed, so a consumer can range over its own
// lifetime and stop on context cancellation. If a consumer falls behind,
// sentences are dropped rather than queued without bound; see Stats.Dropped.
func (p *Parser) Events() <-chan Event { return p.events }

// History returns the most recent decoded sentences, oldest first, bounded
// by WithKeepHistory. It is empty unless history was enabled.
func (p *Parser) History() []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Event, len(p.history))
	// The ring is ordered oldest to newest starting at the head, so unwrap it
	// here; a caller asking for history wants chronological order, not the
	// buffer's internal layout.
	for i := range p.history {
		out[i] = p.history[(p.historyHead+i)%len(p.history)]
	}
	return out
}

// ParseLine decodes a single NMEA line. It is the one-shot entry point,
// useful for tests, for replaying a log file, and for sentences that arrive
// by some other route such as Bluetooth or a UDP packet.
//
// A line that is not an NMEA sentence yields fault.ErrNotSentence, which callers
// normally ignore. A sentence whose checksum failed is still decoded and
// returned, with Event.Err set.
func (p *Parser) ParseLine(line string) (Event, error) {
	s, err := ParseSentence(line)
	if err != nil {
		return Event{Sentence: s, Time: p.now()}, err
	}
	ev := p.handle(s)
	p.dispatch(ev)
	return ev, nil
}

// Consume reads from r until EOF or ctx is done, decoding every sentence it
// finds. It blocks, so run it in its own goroutine:
//
//	go func() { err := p.Consume(ctx, port) }()
//
// Consume returns nil on a clean EOF, ctx.Err() if the context was
// cancelled, or the underlying read error. Per-sentence problems never stop
// the stream; they go to the error handlers.
//
// Only one Consume runs at a time per parser. A second call waits for the
// first to finish, because the line splitter is stateful and interleaving two
// byte streams would corrupt sentence framing. Concurrent calls to Fix,
// Stats, and the handler methods are fine at any time.
//
// Cancellation is cooperative. Consume returns once the current Read
// returns, so a Read blocked on an idle serial port or socket does not wake
// up when the context is cancelled; close the underlying reader to interrupt
// it. The context is not derived here because nothing in the read loop
// cancels it: passing the caller's context straight through is equivalent and
// saves an allocation per stream.
func (p *Parser) Consume(ctx context.Context, r io.Reader) error {
	if r == nil {
		return errors.New("nmea: Consume needs a non-nil reader")
	}
	p.consumeMu.Lock()
	defer p.consumeMu.Unlock()

	_, err := p.readLoop(ctx, r, make([]byte, p.readBufSize))
	return err
}

// ReadFrom implements io.ReaderFrom so a Parser can sit at the end of an
// io.Copy chain. It reads until EOF and returns nil.
//
// It takes the same lock as Consume, for the same reason.
func (p *Parser) ReadFrom(r io.Reader) (int64, error) {
	if r == nil {
		return 0, errors.New("nmea: ReadFrom needs a non-nil reader")
	}
	p.consumeMu.Lock()
	defer p.consumeMu.Unlock()

	return p.readLoop(context.Background(), r, make([]byte, p.readBufSize))
}

// Feed pushes raw bytes into the parser directly, for callers that have NMEA
// bytes but not as an io.Reader, such as a WebSocket frame or a
// callback-based BLE notification. It returns the number of complete
// sentences it produced.
func (p *Parser) Feed(data []byte) int {
	lines := p.splitter.Feed(data)
	for _, l := range lines {
		p.parseAndDispatch(l)
	}
	return len(lines)
}

// Close flushes a buffered partial line, for sources that end without an
// EOF, such as a serial port the application closes itself.
func (p *Parser) Close() {
	for _, line := range p.splitter.Close() {
		p.parseAndDispatch(line)
	}
}

// Reset clears the merged fix, as after a receiver mode change. Counters
// and history are left alone.
func (p *Parser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fix.Reset()
}

// readLoop is the single read path behind Consume and ReadFrom.
//
// The line slice is reused across reads. A receiver at 10 Hz produces several
// sentences per read, and allocating a fresh slice for each one is pure
// garbage-collector pressure in the one place that must never fall behind.
func (p *Parser) readLoop(ctx context.Context, r io.Reader, buf []byte) (int64, error) {
	var total int64
	var lines []string
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		n, err := r.Read(buf)
		if n > 0 {
			total += int64(n)
			lines = p.feedBytes(lines[:0], buf[:n])
		}

		switch {
		case err == nil:
			continue
		case errors.Is(err, io.EOF):
			// Decode a trailing sentence the sender never terminated,
			// which happens whenever a file has no final newline.
			p.Close()
			return total, nil
		default:
			return total, err
		}
	}
}

// feedBytes accounts for the bytes and turns complete lines into events. The
// byte and line counters are batched into one lock acquisition per read
// rather than one per sentence.
func (p *Parser) feedBytes(dst []string, data []byte) []string {
	lines := p.splitter.Append(dst, data)
	if len(lines) == 0 {
		p.mu.Lock()
		p.stats.Bytes += uint64(len(data))
		p.mu.Unlock()
		// dst came back unchanged, so its capacity survives for the next read.
		return dst
	}

	p.mu.Lock()
	p.stats.Bytes += uint64(len(data))
	p.stats.Lines += uint64(len(lines))
	p.mu.Unlock()

	for _, l := range lines {
		p.parseAndDispatch(l)
	}
	// Handing the slice back is what makes the reuse real: readLoop passes
	// lines[:0] in again, and Append fills that backing array instead of
	// allocating a fresh one per read.
	return lines
}

// parseAndDispatch frames a line, handles it, and offers it to the channel.
func (p *Parser) parseAndDispatch(line string) {
	s, err := ParseSentence(line)
	if err != nil {
		// Noise on a shared serial bus is expected and is never surfaced
		// as an error.
		return
	}
	ev := p.handle(s)
	p.dispatch(ev)
}

// handle decodes a framed sentence, updates the fix, runs the handlers, and
// returns the resulting event. Handlers run on the caller's goroutine.
func (p *Parser) handle(s Sentence) Event {
	ev := Event{Sentence: s, Time: p.now()}

	// A missing checksum is not a failure: plenty of receivers omit it, and
	// the sentence is still well formed. An invalid or malformed one is a
	// real problem, but by default the payload is still folded in, because
	// many receivers get the checksum wrong while the data is fine. Strict
	// mode is the opt-in for discarding it, and requiring a checksum is the
	// stricter opt-in for untrusted input.
	requiresChecksum := p.requireChecksum && !s.ChecksumState.Present()
	if cstate := s.ChecksumState; !cstate.OK() {
		p.bump(func(st *Stats) { st.BadChecksums++ })
		ev.Err = fmt.Errorf("%w: %s got %02X, want %02X",
			cstate.Error(), s.Address(), s.Checksum, s.Computed)
		p.report(ev.Err)
	} else if requiresChecksum {
		p.bump(func(st *Stats) { st.MissingChecksums++ })
		ev.Err = fmt.Errorf("%w: %s", fault.ErrMissingChecksum, s.Address())
		p.report(ev.Err)
	}
	// The payload is folded into the fix unless the caller has asked us not
	// to trust it, either by requiring a checksum or by going strict.
	payloadUsable := true
	switch {
	case requiresChecksum:
		payloadUsable = false
	case !s.ChecksumState.OK() && p.strictChecksum:
		payloadUsable = false
	}

	decoded, err := p.registry.Decode(s)
	switch {
	case errors.Is(err, fault.ErrUnknownSentence):
		p.bump(func(st *Stats) { st.Unknown++ })
		ev.Err = err
		p.report(err)
	case err != nil:
		ev.Err = err
		p.report(err)
	default:
		ev.Value = decoded
	}

	// A decode failure means the value is missing fields by definition, so
	// the fold happens only when a payload came back at all.
	if ev.Value != nil && payloadUsable {
		p.applyToFix(s, ev.Value)
	}

	// The version detector and the receiver description are updated for
	// every decoded sentence, before the handlers run, so a handler can ask
	// what the receiver has revealed so far.
	if ev.Value != nil {
		p.detector.Observe(ev.Value, ev.Address())
		if ver, ok := value.AsVER(ev.Value); ok {
			p.mu.Lock()
			p.revision.Apply(ver)
			p.mu.Unlock()
		}
	}

	p.fireEvent(ev)
	p.record(ev)
	return ev
}

// applyToFix folds a decoded value into the fix and notifies fix handlers.
//
// The fix is copied out for each handler because a handler may keep the
// pointer, and handing out a pointer into the parser's own state would let
// one handler's mutation race with the read loop. When no handler is
// registered the copy is skipped entirely: the state is still updated, but
// escaping a struct that nothing will ever see costs an allocation per
// contributing sentence, and most programs poll Fix rather than subscribe.
func (p *Parser) applyToFix(s Sentence, value any) {
	contributor, ok := value.(FixContributor)
	if !ok {
		return
	}

	p.mu.Lock()
	p.fix.applyAt(contributor, p.now(), s.Address())
	// Stats.Sentences counts the sentences that actually moved the aggregate
	// state, which is what the field documents. It is bumped here rather than
	// on every decoded sentence, because a WPL or an RTE is decoded but is
	// deliberately not an observation, and counting it would make the number
	// disagree with both Fix.SentenceCount and the OnFix handler count.
	p.stats.Sentences++
	if len(p.onFix) == 0 {
		p.mu.Unlock()
		return
	}
	f := p.fix
	// The satellite slice is the only reference inside the struct, so it is
	// the only one needing its own copy.
	if len(p.fix.Satellites) > 0 {
		f.Satellites = append([]Satellite(nil), p.fix.Satellites...)
	}
	p.mu.Unlock()

	for _, fn := range p.onFix {
		if err := fn(&f); err != nil {
			p.report(err)
		}
	}
}

// fireEvent runs the per-sentence handlers.
func (p *Parser) fireEvent(ev Event) {
	for _, fn := range p.onEvent {
		if err := fn(ev); err != nil {
			p.report(err)
		}
	}
	if ev.Value == nil {
		return
	}
	for _, fn := range p.byType[ev.Sentence.Type] {
		if err := fn(ev); err != nil {
			p.report(err)
		}
	}
}

// report hands an error to every error handler.
func (p *Parser) report(err error) {
	if err == nil {
		return
	}
	for _, fn := range p.onError {
		fn(err)
	}
}

// bump adjusts a counter under the lock.
func (p *Parser) bump(fn func(*Stats)) {
	p.mu.Lock()
	fn(&p.stats)
	p.mu.Unlock()
}

// record updates the ring buffer used by History.
func (p *Parser) record(ev Event) {
	if p.historySize <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.history) < p.historySize {
		p.history = append(p.history, ev)
		return
	}
	// The buffer is full, so overwrite the oldest slot and advance the head.
	// Shifting the slice to drop the first element is O(n) per sentence, which
	// at a history of a thousand events and a 10 Hz receiver is on the order
	// of a hundred kilobytes of copying a second.
	p.history[p.historyHead] = ev
	p.historyHead = (p.historyHead + 1) % p.historySize
}

// dispatch offers an event to the channel, dropping it if the buffer is
// full. Dropping is deliberate: blocking here would overflow the serial
// driver's buffer and lose data for good, whereas a dropped event shows up
// in Stats and the next sentence arrives a fraction of a second later.
func (p *Parser) dispatch(ev Event) {
	if p.events == nil {
		return
	}
	select {
	case p.events <- ev:
	default:
		p.bump(func(st *Stats) { st.Dropped++ })
	}
}
