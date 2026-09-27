package nmea

import (
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"sort"
	"sync"
)

// Decoder turns a Sentence into a typed value. One decoder handles one
// sentence formatter, e.g. GGA.
//
// Implementations live outside this package so that sentence definitions
// can evolve without touching the parser, and so a program can be given a
// registry containing only the sentences it cares about. The built-in
// decoders are in the sentences subpackage.
type Decoder interface {
	// Formatter is the three-character sentence formatter this decoder
	// claims, e.g. "GGA". It must be upper case and must not include the
	// talker.
	Formatter() string
	// Decode interprets the sentence. A returned error means the sentence
	// could not be understood. Returning a partially populated value
	// alongside the error is allowed, but callers should not trust it.
	Decode(s Sentence) (any, error)
}

// FixContributor is implemented by decoded sentence values that feed the
// parser's aggregate fix state.
//
// This is how a sentence updates Fix without the parser importing the
// sentence's package. A decoder that fills in latitude also declares "fold
// me into the fix", and the parser calls ApplyFix after a successful
// decode. Values that carry no positional data (GSV, ZDA, WPL) simply do
// not implement it.
type FixContributor interface {
	ApplyFix(f *Fix)
}

// Registry maps sentence formatters to decoders. A Registry is safe for
// concurrent use.
type Registry struct {
	mu     sync.RWMutex
	byType map[string]Decoder
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byType: make(map[string]Decoder)}
}

// DefaultRegistry is the registry that the built-in decoders register into
// when the sentences package is imported. A Parser starts from this
// registry, so importing the sentences package is all that is needed to
// make every built-in sentence available:
//
//	import _ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
var DefaultRegistry = NewRegistry()

// Register adds a decoder, replacing any existing decoder for the same
// formatter, and returns the decoder it displaced (nil if none).
//
// Re-registering the same formatter is the intended way to override a
// built-in decoder with a vendor-specific variant.
func (r *Registry) Register(d Decoder) Decoder {
	if d == nil || d.Formatter() == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.byType[d.Formatter()]
	r.byType[d.Formatter()] = d
	return old
}

// MustRegister adds a decoder and panics if it has no formatter, which
// can only be a programming error. It exists for package init functions.
func (r *Registry) MustRegister(d Decoder) {
	if d == nil || d.Formatter() == "" {
		panic("nmea: decoder registered without a sentence formatter")
	}
	r.Register(d)
}

// Lookup finds the decoder for a sentence, preferring an exact formatter
// match. A talker-qualified registration such as "GNGGA" takes precedence
// over a bare "GGA", so a program can special-case one constellation
// without shadowing the general case.
func (r *Registry) Lookup(talker, typ string) (Decoder, bool) {
	if typ == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	if d, ok := r.byType[typ]; ok {
		return d, true
	}
	if talker != "" {
		if d, ok := r.byType[talker+typ]; ok {
			return d, true
		}
	}
	return nil, false
}

// Decode looks up and runs the decoder for a sentence.
//
// An unregistered formatter yields fault.ErrUnknownSentence itself rather than a
// wrapped copy. That is deliberate: a receiver streaming proprietary
// sentences produces one of these per sentence, and wrapping allocated an
// error on the hot path for every one. The formatter is already available to
// the caller on the Sentence, so nothing is lost.
func (r *Registry) Decode(s Sentence) (any, error) {
	d, ok := r.Lookup(s.Talker, s.Type)
	if !ok {
		return nil, fault.ErrUnknownSentence
	}
	return d.Decode(s)
}

// Types lists every registered formatter, sorted. Useful for diagnostics
// and for a CLI that wants to report what the library understands.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byType))
	for t := range r.byType {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Len reports how many decoders are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byType)
}

// Clone returns an independent copy, so a parser can be given a private set
// of decoders without disturbing the default registry.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c := &Registry{byType: make(map[string]Decoder, len(r.byType))}
	for t, d := range r.byType {
		c.byType[t] = d
	}
	return c
}
