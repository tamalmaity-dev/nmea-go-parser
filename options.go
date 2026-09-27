package nmea

import "time"

// Option configures a Parser. Options exist so the common cases read well
// at the call site while every default still has a sensible value:
//
//	p := nmea.New(
//	    nmea.WithOnFix(func(f *nmea.Fix) error { fmt.Println(f); return nil }),
//	    nmea.WithStrictChecksum(true),
//	)
type Option func(*Parser)

// WithRegistry gives the parser a private set of decoders instead of the
// default registry. Pass nmea.NewRegistry() for none, or
// DefaultRegistry.Clone() to start from the built-ins and extend them.
func WithRegistry(r *Registry) Option {
	return func(p *Parser) {
		if r == nil {
			r = NewRegistry()
		}
		p.registry = r.Clone()
	}
}

// WithOnFix registers a handler called every time a sentence changes the
// fix. This is the primary hook: a full position report arrives here.
//
// The handler runs on the read loop, so keep it short or hand work to your
// own goroutine. Returning an error does not stop the stream; it is
// delivered to the error handlers.
func WithOnFix(fn func(*Fix) error) Option {
	return func(p *Parser) { p.onFix = append(p.onFix, fn) }
}

// WithOnEvent registers a handler called for every sentence the parser
// recognises, before it updates the fix. Event.Value holds the decoded
// value, so this is where per-sentence logging belongs.
func WithOnEvent(fn func(Event) error) Option {
	return func(p *Parser) { p.onEvent = append(p.onEvent, fn) }
}

// WithOnError registers a handler for bad checksums, unknown sentences,
// and decode failures. It is not called for non-NMEA noise.
func WithOnError(fn func(error)) Option {
	return func(p *Parser) { p.onError = append(p.onError, fn) }
}

// WithChannelBuffer enables the sentence channel with the given buffer
// size. Zero or negative disables it.
//
// The channel is fed on a best-effort basis: if a consumer falls behind and
// the buffer fills, sentences are dropped rather than stalling the read
// loop, because a stalled serial reader loses data permanently. Count the
// losses with Dropped().
func WithChannelBuffer(n int) Option {
	return func(p *Parser) { p.chanBuf = n }
}

// WithStrictChecksum makes the parser discard the payload of any sentence
// whose checksum does not verify. By default a bad checksum is reported but
// the payload is still used, since many receivers get the checksum wrong
// while the data itself is fine.
func WithStrictChecksum(strict bool) Option {
	return func(p *Parser) { p.strictChecksum = strict }
}

// WithRequireChecksum rejects a sentence that carries no checksum at all.
//
// The standard makes the checksum mandatory, but receivers omit it often
// enough that the default is to accept such a sentence. This turns that
// around, for a program feeding a receiver whose output is not trusted and
// where an unchecksummed line is more likely to be noise than data.
func WithRequireChecksum(require bool) Option {
	return func(p *Parser) { p.requireChecksum = require }
}

// WithReadBuffer sets the size of the buffer the read loop uses. A larger
// buffer means fewer syscalls when a receiver emits sentences faster than
// the loop wakes up. Default 1024, which comfortably holds a burst of
// sentences at 10 Hz and 115200 baud. The value only affects syscall count,
// never which bytes are read.
func WithReadBuffer(n int) Option {
	return func(p *Parser) {
		if n > 0 {
			p.readBufSize = n
		}
	}
}

// WithMaxLineLength bounds a single sentence. See DefaultMaxLineLength.
func WithMaxLineLength(n int) Option {
	return func(p *Parser) { p.splitter = NewLineSplitter(n) }
}

// WithKeepHistory retains the last n decoded sentence values for
// inspection, which is invaluable when debugging a misbehaving receiver.
// Zero disables history, which is the default.
func WithKeepHistory(n int) Option {
	return func(p *Parser) {
		p.historySize = n
		if n > 0 && p.history == nil {
			p.history = make([]Event, 0, n)
		}
	}
}

// WithClock overrides the time source used to stamp fix updates. Tests use
// it to get deterministic timestamps; leave it nil in production.
func WithClock(now func() time.Time) Option {
	return func(p *Parser) {
		if now == nil {
			now = time.Now
		}
		p.now = now
	}
}
