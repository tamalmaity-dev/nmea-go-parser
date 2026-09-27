package nmea_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
	_ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// corpus returns the recorded sample data as one blob, which is what a
// serial read actually delivers: many sentences in one buffer.
func corpus(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile("testdata/sample.nmea")
	if err != nil {
		tb.Fatalf("reading sample data: %v", err)
	}
	return data
}

func BenchmarkParseSentence(b *testing.B) {
	line := "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18"
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := nmea.ParseSentence(line); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChecksum(b *testing.B) {
	body := "GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000"
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()

	var sink uint8
	for i := 0; i < b.N; i++ {
		sink ^= nmea.Checksum(body)
	}
	_ = sink
}

func BenchmarkChecksumValidate(b *testing.B) {
	line := "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := nmea.Validate(line); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseLine measures a single line through the full pipeline:
// framing, checksum verification, registry lookup, and decode. This is the
// per-sentence cost a program pays on every read.
func BenchmarkParseLine(b *testing.B) {
	p := nmea.New()
	line := "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := p.ParseLine(line); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConsume measures the sustained rate over a realistic stream, which
// is what matters for a receiver running at 10 Hz. This is the parse-only
// case: a program that polls Parser.Fix rather than subscribing to it.
func BenchmarkConsume(b *testing.B) {
	one := corpus(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(one)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := nmea.New(nmea.WithReadBuffer(4096))
		if err := p.Consume(testContext(b), bytes.NewReader(one)); err != nil {
			b.Fatal(err)
		}
		if p.Fix().SentenceCount == 0 {
			b.Fatal("no sentences contributed to the fix")
		}
	}
}

// BenchmarkConsumeWithHandler adds a fix handler, which is the realistic
// case, and measures the cost of the callback dispatch.
func BenchmarkConsumeWithHandler(b *testing.B) {
	one := bytes.Repeat(corpus(b), 1)
	b.ReportAllocs()
	b.SetBytes(int64(len(one)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := nmea.New(nmea.WithReadBuffer(4096))
		var seen int
		p.OnFix(func(f *nmea.Fix) error { seen++; return nil })
		if err := p.Consume(testContext(b), bytes.NewReader(one)); err != nil {
			b.Fatal(err)
		}
		if seen == 0 {
			b.Fatal("no fix callbacks fired")
		}
	}
}

// BenchmarkParseOnly decodes without a fix handler, isolating the cost of
// framing and decoding from the cost of maintaining the aggregate state.
func BenchmarkParseOnly(b *testing.B) {
	data := corpus(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := nmea.New()
		sc := bufio.NewScanner(bytes.NewReader(data))
		for sc.Scan() {
			if _, err := p.ParseLine(sc.Text()); err != nil && !errors.Is(err, nmea.ErrNotSentence) {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkLineSplitterAppend(b *testing.B) {
	// The realistic read path: a caller-owned slice is reused across reads,
	// which is what the parser's read loop does.
	one := bytes.Repeat(corpus(b), 8)
	chunk := make([]byte, 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(one)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ls := nmea.NewLineSplitter(0)
		var lines []string
		for off := 0; off < len(one); off += len(chunk) {
			end := off + len(chunk)
			if end > len(one) {
				end = len(one)
			}
			lines = ls.Append(lines[:0], one[off:end])
		}
		ls.Close()
	}
}

func BenchmarkLineSplitterFeed(b *testing.B) {
	// The allocating convenience form, kept as a comparison point.
	one := bytes.Repeat(corpus(b), 8)
	chunk := make([]byte, 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(one)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ls := nmea.NewLineSplitter(0)
		for off := 0; off < len(one); off += len(chunk) {
			end := off + len(chunk)
			if end > len(one) {
				end = len(one)
			}
			ls.Feed(one[off:end])
		}
		ls.Close()
	}
}

func BenchmarkCoordinateParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := nmea.ParseCoordinate("3723.2475", "N", nmea.AxisLatitude); err != nil {
			b.Fatal(err)
		}
	}
}
