package nmea_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
	_ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// TestDataTypeOf checks the branch helper a caller reaches for before knowing
// a value's concrete type.
func TestDataTypeOf(t *testing.T) {
	if got := nmea.DataTypeOf(nil); got != "" {
		t.Errorf("DataTypeOf(nil) = %q, want empty", got)
	}
	if got := nmea.DataTypeOf(42); got != "" {
		t.Errorf("DataTypeOf(42) = %q, want empty", got)
	}

	s, err := nmea.ParseSentence("$GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70")
	if err != nil {
		t.Fatalf("ParseSentence: %v", err)
	}
	v, err := nmea.DefaultRegistry.Decode(s)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := nmea.DataTypeOf(v); got != nmea.TypeRMC {
		t.Errorf("DataTypeOf(decoded RMC) = %q, want %q", got, nmea.TypeRMC)
	}
}

// TestRealCaptureParses drives the whole recorded multi-GNSS capture through
// the parser. It is the aggregate check the per-sentence tests cannot give:
// the framing, checksums, registry, and fix merge all run over real
// receiver output at once.
func TestRealCaptureParses(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.nmea")
	if err != nil {
		t.Fatalf("reading capture: %v", err)
	}

	p := nmea.New()
	if err := p.Consume(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	st := p.Stats()
	if st.Lines == 0 {
		t.Fatal("no lines read from the capture")
	}
	if st.BadChecksums != 0 {
		t.Errorf("BadChecksums = %d, want 0 for a clean capture", st.BadChecksums)
	}
	if !p.Fix().HasPosition {
		t.Error("capture contains GGA/GLL fixes but the aggregate fix has no position")
	}
}
