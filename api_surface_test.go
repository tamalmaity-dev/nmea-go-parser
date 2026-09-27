package nmea_test

// This file exists to prove that the API the README documents still compiles
// after the value types and sentinel errors moved into their own packages and
// the root package began re-exporting them. Every symbol named here is one the
// README or docs/nmea.md tells a user to write.

import (
	"errors"
	"log"
	"testing"

	nmea "github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
	_ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
	"github.com/tamalmaity-dev/nmea-go-parser/value"
)

func TestDocumentedAPISurfaceCompiles(t *testing.T) {
	// The README's headline example: parse one sentence and read fields.
	const raw = "$GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70"

	s, err := nmea.ParseSentence(raw)
	if err != nil {
		t.Fatalf("ParseSentence: %v", err)
	}
	v, err := nmea.DefaultRegistry.Decode(s)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if nmea.DataTypeOf(v) != nmea.TypeRMC {
		t.Fatalf("DataTypeOf = %q, want RMC", nmea.DataTypeOf(v))
	}
	m := v.(sentences.RMC)
	if m.Latitude == 0 || m.Longitude == 0 {
		t.Error("RMC position did not decode")
	}
	if got := nmea.FormatGPS(m.Latitude); got != "5133.8200" {
		t.Errorf("FormatGPS = %q, want 5133.8200", got)
	}
	if got := nmea.FormatDMS(m.Latitude); got == "" {
		t.Error("FormatDMS returned nothing")
	}
	if !s.ChecksumOK() {
		t.Error("ChecksumOK = false for a sentence the README says verifies")
	}

	// Forming and framing, as the custom-decoder example shows.
	if got := nmea.Frame("XYZ,42.5"); got != "$XYZ,42.5*6A" {
		t.Errorf("Frame = %q, want $XYZ,42.5*6A", got)
	}

	// The value types, reached through the root package as the docs show.
	var lat nmea.Coordinate
	lat, err = nmea.ParseCoordinate("3723.2475", "N", nmea.AxisLatitude)
	if err != nil {
		t.Fatalf("ParseCoordinate: %v", err)
	}
	if !lat.Valid {
		t.Error("coordinate did not validate")
	}
	var tod nmea.TOD
	tod, err = nmea.ParseTOD("161229.487")
	if err != nil {
		t.Fatalf("ParseTOD: %v", err)
	}
	if tod.String() == "" {
		t.Error("TOD.String() is empty")
	}

	// The same types through the value package, which must be identical.
	var vlat value.Coordinate = lat
	if vlat.ValueString() != lat.ValueString() {
		t.Error("nmea.Coordinate and value.Coordinate disagree")
	}
	vtod, err := value.ParseTOD("161229.487")
	if err != nil {
		t.Fatalf("value.ParseTOD: %v", err)
	}
	if vtod.String() != tod.String() {
		t.Error("nmea.TOD and value.TOD disagree")
	}

	// Errors, tested through either path.
	if !errors.Is(faultValueErr(), nmea.ErrFieldValue) {
		t.Error("a field error must satisfy errors.Is against nmea.ErrFieldValue")
	}

	// The aggregate fix, and the handler registrations the README shows.
	p := nmea.New(nmea.WithKeepHistory(8), nmea.WithChannelBuffer(8))
	p.OnFix(func(f *nmea.Fix) error {
		if f.HasPosition {
			_ = f.Latitude
		}
		return nil
	})
	p.OnError(func(error) {})
	p.OnEvent(func(nmea.Event) error { return nil })
	p.On("GGA", func(nmea.Event) error { return nil })
	sentences.Watch(p, "GGA", func(g sentences.GGA) error {
		_ = g.Latitude
		return nil
	})
	ev, err := p.ParseLine("$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47")
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if ev.Type() != "GGA" {
		t.Errorf("ev.Type() = %q, want GGA", ev.Type())
	}
	var gga sentences.GGA
	if err := ev.As(&gga); err != nil {
		t.Fatalf("Event.As: %v", err)
	}
	if len(p.History()) == 0 {
		t.Error("History is empty after ParseLine")
	}
	// Sentences counts what reached the fix. Lines counts what came off the
	// wire, and ParseLine deliberately bypasses the splitter, so it stays 0.
	if p.Stats().Sentences == 0 {
		t.Error("Stats.Sentences did not advance for a contributing sentence")
	}
	if _, _, ok := p.Position(); !ok {
		t.Error("Position reported no fix after a GGA")
	}
}

func faultValueErr() error {
	_, err := nmea.ParseCoordinate("nonsense", "N", nmea.AxisLatitude)
	if err == nil {
		log.Fatal("ParseCoordinate accepted nonsense")
	}
	return err
}
