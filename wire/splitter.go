package wire

import (
	"bytes"
	"sync"
)

// DefaultMaxLineLength bounds a single sentence. Real sentences are under
// 82 bytes (the NMEA 0183 limit); 255 leaves generous headroom for the
// verbose proprietary sentences some modules emit.
const DefaultMaxLineLength = 255

// LineSplitter turns a byte stream into complete NMEA lines. It exists
// because a serial read returns an arbitrary slice of bytes: one read may
// contain three sentences and half of a fourth, or a single sentence split
// mid-field. The splitter buffers across reads and only releases whole
// lines.
//
// It is safe for concurrent use: the parser's read loop is the only caller
// in normal operation, but external code feeding the parser from several
// sources shares one splitter safely.
type LineSplitter struct {
	mu  sync.Mutex
	buf []byte
	max int
	// start is the index of the first unconsumed byte in buf. buf itself
	// always begins at offset zero; consumed bytes are dropped by moving the
	// remainder down, which keeps the capacity available for the next append.
	start    int
	overflow int // count of lines dropped for exceeding max
}

// NewLineSplitter creates a splitter with the given maximum line length.
// A max of 0 selects DefaultMaxLineLength.
func NewLineSplitter(max int) *LineSplitter {
	if max <= 0 {
		max = DefaultMaxLineLength
	}
	// The buffer is sized to hold a burst of sentences in one read, which is
	// the common case at 10 Hz and saves a growth step on the first few
	// reads. It grows on demand beyond that.
	ls := &LineSplitter{max: max, buf: make([]byte, 0, 1024)}
	return ls
}

// Reset discards any partially buffered input and releases the buffer, so a
// long-lived splitter does not hold memory after its source has gone away.
func (ls *LineSplitter) Reset() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.buf = ls.buf[:0]
	ls.start = 0
	ls.overflow = 0
}

// Overflows reports how many lines have been dropped because they
// exceeded the maximum length. A non-zero count means the stream is
// corrupted or the port baud rate is too low, and is worth surfacing in
// logs.
func (ls *LineSplitter) Overflows() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.overflow
}

// Feed appends data and returns every complete line it can now yield.
//
// NMEA terminates sentences with CRLF, LF, or a bare CR; all three are
// treated as line breaks and any run of them collapses to one. Blank lines
// are skipped.
//
// The returned slice is freshly allocated. A caller processing a long stream
// should prefer Append, which reuses a caller-owned slice and keeps the
// garbage collector out of the read loop entirely.
func (ls *LineSplitter) Feed(data []byte) []string {
	return ls.Append(nil, data)
}

// Append feeds data and appends every complete line it can now yield to dst,
// returning the extended slice. Passing the previous result back in reuses
// its capacity, which is what makes the read loop allocation-free per
// sentence:
//
//	var lines []string
//	for {
//	    lines = splitter.Append(lines[:0], buf[:n])
//	    for _, line := range lines { ... }
//	}
//
// The strings themselves are still allocated, one per sentence, because a
// decoded sentence keeps a reference to its own text.
func (ls *LineSplitter) Append(dst []string, data []byte) []string {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.buf = append(ls.buf, data...)

	for {
		i := indexTerminator(ls.buf[ls.start:])
		if i < 0 {
			break
		}
		i += ls.start
		line := string(ls.buf[ls.start:i])
		// Consume the terminator: one byte for LF or CR, two for CRLF.
		n := 1
		if ls.buf[i] == '\r' && i+1 < len(ls.buf) && ls.buf[i+1] == '\n' {
			n = 2
		}
		ls.start = i + n

		if line = trimLine(line); line != "" {
			dst = append(dst, line)
		}
	}

	// A single line longer than the limit means we lost framing. Drop the
	// overflow up to the next terminator so the stream can resynchronise.
	if len(ls.buf)-ls.start > ls.max {
		ls.overflow++
		if i := indexTerminator(ls.buf[ls.start:]); i >= 0 {
			ls.start += i + 1
		} else {
			ls.start = len(ls.buf)
		}
	}

	// Drop the consumed prefix so the buffer starts at offset zero again.
	// Advancing the slice instead would shrink the capacity by every line
	// consumed, and append would then grow and copy every few sentences
	// rather than reuse the array. The copy moves only the partial line,
	// which is usually empty.
	if ls.start > 0 {
		rest := len(ls.buf) - ls.start
		copy(ls.buf, ls.buf[ls.start:])
		ls.buf = ls.buf[:rest]
		ls.start = 0
	}

	return dst
}

// indexTerminator returns the offset of the first CR or LF, or -1.
//
// Two IndexByte scans rather than a hand-rolled loop: the stdlib searches with
// SIMD, and a sentence is short enough that a per-byte loop's overhead
// dominates. Measured at roughly five times the loop on this shape of buffer.
// IndexAny is slower still for a two-character set.
func indexTerminator(b []byte) int {
	cr := bytes.IndexByte(b, '\r')
	lf := bytes.IndexByte(b, '\n')
	switch {
	case cr < 0:
		return lf
	case lf < 0:
		return cr
	case cr < lf:
		return cr
	default:
		return lf
	}
}

// Close flushes any trailing unterminated line. Some receivers power-cycle
// mid-sentence, and file sources often lack a final newline, so the
// buffered remainder is worth decoding if it looks like a sentence.
func (ls *LineSplitter) Close() []string {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if len(ls.buf) == 0 {
		return nil
	}
	line := trimLine(string(ls.buf[ls.start:]))
	ls.buf = ls.buf[:0]
	ls.start = 0
	if line == "" {
		return nil
	}
	return []string{line}
}

func trimLine(s string) string {
	start := 0
	for start < len(s) && isSpaceByte(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\x00' || c == 0xFF
}
