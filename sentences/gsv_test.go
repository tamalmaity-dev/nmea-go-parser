package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGSVCycle(t *testing.T) {
	first := decodeOne[GSV](t, "$GPGSV,2,1,07,07,79,048,42,02,51,062,43,26,36,256,42,27,27,138,42*71")
	if first.TotalMessages != 2 || first.MessageNumber != 1 {
		t.Errorf("message %d of %d, want 1 of 2", first.MessageNumber, first.TotalMessages)
	}
	if first.CycleComplete() {
		t.Error("CycleComplete = true for the first of two sentences")
	}
	if len(first.Satellites) != 4 {
		t.Fatalf("got %d satellites, want 4", len(first.Satellites))
	}
	// Elevation, azimuth, and SNR are fields 3, 4, and 5 of the first group.
	sat := first.Satellites[0]
	if sat.ID != 7 || sat.Elevation != 79 || sat.Azimuth != 48 || !sat.HasSNR || sat.SNR != 42 {
		t.Errorf("first satellite = %+v, want ID 7, elev 79, azim 48, SNR 42", sat)
	}

	second := decodeOne[GSV](t, "$GPGSV,2,2,07,09,23,313,42,04,19,159,41,15,12,041,42*41")
	if !second.CycleComplete() {
		t.Error("CycleComplete = false for the last of two sentences")
	}
	if len(second.Satellites) != 3 {
		t.Errorf("got %d satellites in the second sentence, want 3", len(second.Satellites))
	}
	// SatellitesInView is the count for the whole cycle, so both sentences
	// must report the same total.
	if first.SatellitesInView != 7 || second.SatellitesInView != 7 {
		t.Errorf("SatellitesInView = %d and %d, want 7 in both",
			first.SatellitesInView, second.SatellitesInView)
	}
	if best, ok := first.Strongest(); !ok || best.SNR != 43 {
		t.Errorf("Strongest = %+v (ok %v), want the satellite with SNR 43", best, ok)
	}
}
func TestGSVTruncatedGroupIsAnError(t *testing.T) {
	// A group of four fields is one satellite. A trailing partial group
	// means the sentence was truncated, and must not be silently read as a
	// satellite with missing data.
	line := nmea.Frame("GPGSV,1,1,05,01,40,083,46,02,17")
	s, err := nmea.ParseSentence(line)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nmea.DefaultRegistry.Decode(s); err == nil {
		t.Error("truncated GSV decoded without error, want an error")
	}
}
func TestGSVSignalID(t *testing.T) {
	// NMEA 4.10 and later append ONE signal id for the whole sentence, after
	// the last satellite group. Treating it as a fifth value in each group
	// would shift every group after the first.
	g := decodeOne[GSV](t, "$GPGSV,3,1,12,03,45,150,42,07,30,210,39,11,62,080,45,14,15,310,35,1*60")

	if len(g.Satellites) != 4 {
		t.Fatalf("got %d satellites, want 4", len(g.Satellites))
	}
	// The GP talker attributes every satellite to GPS, and the sentence's one
	// signal id is stamped onto each of them along with the name it resolves
	// to in that system.
	want := []nmea.Satellite{
		{ID: 3, Elevation: 45, Azimuth: 150, SNR: 42, HasSNR: true,
			Constellation: nmea.ConstellationGPS, ConstellationKnown: true,
			Signal: nmea.SignalDigit(1), HasSignal: true},
		{ID: 7, Elevation: 30, Azimuth: 210, SNR: 39, HasSNR: true,
			Constellation: nmea.ConstellationGPS, ConstellationKnown: true,
			Signal: nmea.SignalDigit(1), HasSignal: true},
		{ID: 11, Elevation: 62, Azimuth: 80, SNR: 45, HasSNR: true,
			Constellation: nmea.ConstellationGPS, ConstellationKnown: true,
			Signal: nmea.SignalDigit(1), HasSignal: true},
		{ID: 14, Elevation: 15, Azimuth: 310, SNR: 35, HasSNR: true,
			Constellation: nmea.ConstellationGPS, ConstellationKnown: true,
			Signal: nmea.SignalDigit(1), HasSignal: true},
	}
	for i, w := range want {
		if g.Satellites[i] != w {
			t.Errorf("satellite %d = %+v, want %+v", i, g.Satellites[i], w)
		}
	}
	if !g.HasSignalID {
		t.Fatal("HasSignalID = false for a NMEA 4.10 GSV")
	}
	if feats := g.NMEASupports(); len(feats) != 1 {
		t.Errorf("NMEASupports = %v, want one feature", feats)
	}

	// The band name is resolved per satellite, so a caller does not have to
	// remember to pair the signal id with the system it belongs to.
	for i, sat := range g.Satellites {
		name, ok := sat.SignalName()
		if !ok {
			t.Errorf("satellite %d: SignalName reported no name for a GPS signal id", i)
			continue
		}
		if name != "L1 C/A" {
			t.Errorf("satellite %d: SignalName = %q, want %q", i, name, "L1 C/A")
		}
	}
}
func TestGSVWithoutSignalID(t *testing.T) {
	// A pre-4.10 receiver sends no signal id, and the last satellite group
	// must not be mistaken for one.
	g := decodeOne[GSV](t, "$GPGSV,2,1,07,07,79,048,42,02,51,062,43,26,36,256,42,27,27,138,42*71")
	if g.HasSignalID {
		t.Error("HasSignalID = true for a pre-4.10 GSV")
	}
	if len(g.Satellites) != 4 {
		t.Fatalf("got %d satellites, want 4", len(g.Satellites))
	}
	if last := g.Satellites[3]; last.ID != 27 || last.Elevation != 27 || last.Azimuth != 138 || last.SNR != 42 {
		t.Errorf("last satellite = %+v, want ID 27, elev 27, azim 138, SNR 42", last)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGSV verifies the three-field header, the four-field
// satellite groups, and the optional NMEA 4.10 signal ID.
func TestPublishedGSV(t *testing.T) {
	// Published example 3, which is the one with a trailing blank group.
	third := published[GSV](t, "GPGSV,3,3,11,22,42,067,42,24,14,311,43,27,05,244,00,,,,")
	if third.TotalMessages != 3 || third.MessageNumber != 3 {
		t.Errorf("fields 0 and 1 = %d of %d, want 3 of 3",
			third.MessageNumber, third.TotalMessages)
	}
	// Field 2: eleven satellites in view across the whole cycle.
	if third.SatellitesInView != 11 {
		t.Errorf("field 2 satellites in view = %d, want 11", third.SatellitesInView)
	}
	if !third.CycleComplete() {
		t.Error("CycleComplete = false for sentence 3 of 3")
	}
	// Three complete groups then an empty one, which must be skipped rather
	// than read as a fourth satellite with zero values.
	if len(third.Satellites) != 3 {
		t.Fatalf("got %d satellites, want 3: %+v", len(third.Satellites), third.Satellites)
	}
	// Fields 3 to 6: PRN 22, elevation 42, azimuth 067, SNR 42.
	first := third.Satellites[0]
	if first.ID != 22 || first.Elevation != 42 || first.Azimuth != 67 || !first.HasSNR || first.SNR != 42 {
		t.Errorf("first satellite = %+v, want ID 22, elev 42, azim 67, SNR 42", first)
	}
	// Fields 7 to 10: PRN 24, elevation 14, azimuth 311, SNR 43.
	second := third.Satellites[1]
	if second.ID != 24 || second.Elevation != 14 || second.Azimuth != 311 || second.SNR != 43 {
		t.Errorf("second satellite = %+v, want ID 24, elev 14, azim 311, SNR 43", second)
	}

	// A NMEA 4.10 sentence appends one signal ID for the whole cycle, not
	// one per satellite.
	modern := published[GSV](t, "GPGSV,3,1,11,03,03,111,00,04,15,270,00,06,01,010,00,13,06,292,00,1")
	if !modern.HasSignalID {
		t.Fatal("the trailing signal ID was not decoded")
	}
	if len(modern.Satellites) != 4 {
		t.Errorf("got %d satellites, want 4: the signal ID must not cost a group", len(modern.Satellites))
	}
	if modern.Satellites[0].ID != 3 {
		t.Errorf("first satellite ID = %d, want 3", modern.Satellites[0].ID)
	}
}
