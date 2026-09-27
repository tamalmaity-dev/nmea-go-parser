package nmea

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestVersionOrdering(t *testing.T) {
	// The minor component is not a semver minor: 4.30 is later than 4.11,
	// which a naive numeric-minor comparison would get backwards.
	ordered := []Version{V2_00, V2_01, V2_10, V2_20, V2_30, V3_00, V3_01, V4_00, V4_10, V4_11, V4_30}
	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if !cur.AtLeast(prev) {
			t.Errorf("%s.AtLeast(%s) = false, want true", cur, prev)
		}
		if !prev.Before(cur) {
			t.Errorf("%s.Before(%s) = false, want true", prev, cur)
		}
		if cur.AtLeast(cur) != true {
			t.Errorf("%s.AtLeast(itself) = false, want true", cur)
		}
	}

	if LatestVersion != V4_30 {
		t.Errorf("LatestVersion = %v, want %v (4.30, December 2023)", LatestVersion, V4_30)
	}
	if got := V4_30.String(); got != "4.30" {
		t.Errorf("V4_30.String() = %q, want 4.30", got)
	}
	if got := V2_10.String(); got != "2.10" {
		t.Errorf("V2_10.String() = %q, want 2.10", got)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in   string
		want Version
		ok   bool
	}{
		{in: "4.11", want: V4_11, ok: true},
		{in: "4.30", want: V4_30, ok: true},
		{in: "3.01", want: V3_01, ok: true},
		{in: "2.30", want: V2_30, ok: true},
		{in: " 4.11 ", want: V4_11, ok: true},
		// Build numbers and product versions must not be mistaken for a
		// standard revision: a receiver reporting "1.00" as its NMEA
		// version would poison every version-dependent decision.
		{in: "1.00", ok: false},
		{in: "1000", ok: false},
		{in: "3.2", ok: false},
		{in: "abc", ok: false},
		{in: "", ok: false},
		{in: "4.", ok: false},
		{in: "4.x", ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseVersion(tc.in)
			if ok != tc.ok {
				t.Fatalf("ParseVersion(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFeatureIntroduced(t *testing.T) {
	tests := []struct {
		feature Feature
		want    Version
	}{
		{FeatureFAA, V2_30},
		{FeatureGroundSpeed, V3_00},
		{FeatureRangeScale, V3_00},
		{FeatureSystemID, V4_10},
		{FeatureSignalID, V4_10},
		{FeatureNavStatus, V4_10},
	}
	for _, tc := range tests {
		if got := FeatureIntroduced(tc.feature); got != tc.want {
			t.Errorf("FeatureIntroduced(%v) = %v, want %v", tc.feature, got, tc.want)
		}
	}
}

func TestTalkerConstellation(t *testing.T) {
	tests := []struct {
		talker string
		want   Constellation
		system SystemID
	}{
		// Receivers in the field disagree on BeiDou and QZSS prefixes, and
		// the standard does not date either to a revision, so both are
		// accepted. Rejecting the "wrong" one fails on real hardware.
		{"GP", ConstellationGPS, SystemIDGPS},
		{"GL", ConstellationGLONASS, SystemIDGLONASS},
		{"GA", ConstellationGalileo, SystemIDGalileo},
		{"GB", ConstellationBeiDou, SystemIDBeiDou},
		{"BD", ConstellationBeiDou, SystemIDBeiDou},
		{"GQ", ConstellationQZSS, SystemIDQZSS},
		{"QZ", ConstellationQZSS, SystemIDQZSS},
		{"GI", ConstellationNavIC, SystemIDNavIC},
		{"GN", ConstellationMixed, SystemIDNone},
		{"AI", ConstellationUnknown, SystemIDNone},
		{"", ConstellationUnknown, SystemIDNone},
		{"gp", ConstellationGPS, SystemIDGPS},
	}
	for _, tc := range tests {
		t.Run(tc.talker, func(t *testing.T) {
			if got := TalkerConstellation(tc.talker); got != tc.want {
				t.Errorf("TalkerConstellation(%q) = %v, want %v", tc.talker, got, tc.want)
			}
			if got := TalkerSystemID(tc.talker); got != tc.system {
				t.Errorf("TalkerSystemID(%q) = %v, want %v", tc.talker, got, tc.system)
			}
		})
	}
}

func TestParseSystemID(t *testing.T) {
	// Zero is deliberately not a constellation: Trimble receivers emit 0 for
	// QZSS, and reading it as "no system" would be a silent misattribution.
	for in, want := range map[string]SystemID{
		"1": SystemIDGPS, "2": SystemIDGLONASS, "3": SystemIDGalileo,
		"4": SystemIDBeiDou, "5": SystemIDQZSS, "6": SystemIDNavIC,
	} {
		got, ok := ParseSystemID(in)
		if !ok || got != want {
			t.Errorf("ParseSystemID(%q) = %v, %v; want %v, true", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "0", "7", "x"} {
		if _, ok := ParseSystemID(in); ok {
			t.Errorf("ParseSystemID(%q) reported ok, want false", in)
		}
	}
}

func TestSignalDigitBandDependsOnSystem(t *testing.T) {
	// The same digit means different bands for different systems, so a name
	// resolved without the system would be wrong for most of them. Digit 1 is
	// L1 C/A for GPS, E1C for Galileo, B1I for BeiDou, L1 for GLONASS, and
	// L5-A for NavIC.
	digit, ok := ParseSignalID("1")
	if !ok {
		t.Fatal(`ParseSignalID("1") reported not ok`)
	}
	for _, tc := range []struct {
		sys  SystemID
		want SignalID
	}{
		{SystemIDGPS, SignalIDL1C},
		{SystemIDGalileo, SignalIDE1a},
		{SystemIDBeiDou, SignalIDB1I},
		{SystemIDGLONASS, SignalIDL1},
		{SystemIDNavIC, SignalIDL5A},
	} {
		band, ok := digit.Band(tc.sys)
		if !ok {
			t.Errorf("digit 1 under %v: Band reported not ok", tc.sys)
			continue
		}
		if band != tc.want {
			t.Errorf("digit 1 under %v = %v, want %v", tc.sys, band, tc.want)
		}
	}
}

// TestSignalDigitRefusesUndefined checks the two cases where a name must not be
// invented: an unknown system, and a digit the system does not define.
//
// Returning a plausible band for either is how a Galileo satellite ends up
// labelled "L1 C/A", which reads as correct and is not.
func TestSignalDigitRefusesUndefined(t *testing.T) {
	digit, _ := ParseSignalID("1")
	if _, ok := digit.Band(SystemIDNone); ok {
		t.Error("Band under an unknown system returned a band, want none")
	}
	// Galileo does not define digit 6 in the NMEA tables.
	six, _ := ParseSignalID("6")
	if _, ok := six.Band(SystemIDGalileo); ok {
		t.Error("Band returned a band for a digit Galileo does not define, want none")
	}
	// 0 means "all signals", which is a real answer but not a band name.
	zero, _ := ParseSignalID("0")
	if band, ok := zero.Band(SystemIDGPS); !ok || band != SignalIDAll {
		t.Errorf("digit 0 under GPS = %v, %v; want all signals, true", band, ok)
	}
}

// TestParseSignalIDKeepsDigits checks that the digit survives parsing unchanged.
// It is the property the old SignalID enum broke, where every value was one
// higher than the wire digit and so 1 came back as "all signals".
func TestParseSignalIDKeepsDigits(t *testing.T) {
	for d := 0; d <= 9; d++ {
		got, ok := ParseSignalID(strconv.Itoa(d))
		if !ok {
			t.Errorf("ParseSignalID(%d) reported not ok", d)
			continue
		}
		if int(got) != d {
			t.Errorf("ParseSignalID(%d) = %d, want %d", d, got, d)
		}
	}
	if _, ok := ParseSignalID(""); ok {
		t.Error(`ParseSignalID("") reported ok, want false`)
	}
	if got, _ := ParseSignalID(""); got != DigitNone {
		t.Errorf(`ParseSignalID("") = %v, want DigitNone`, got)
	}
}

// feed pushes a list of raw lines through a parser, which is how a stream
// arrives in practice.
func feed(t *testing.T, p *Parser, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if _, err := p.ParseLine(line); err != nil && !errors.Is(err, ErrNotSentence) {
			t.Fatalf("ParseLine(%q): %v", line, err)
		}
	}
}

func TestDetectorInfersVersionFromTraffic(t *testing.T) {
	// A pre-4.10 stream: no signal id, no system id, no nav status. The
	// detector must not claim a version the traffic does not support.
	p := New()
	feed(t, p,
		"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18",
		"$GPGSA,A,3,07,02,26,27,09,04,15,,,,,,1.8,1.0,1.5*33",
		"$GPGSV,2,1,07,07,79,048,42,02,51,062,43,26,36,256,42,27,27,138,42*71",
		"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10",
	)
	if v, reported := p.Version(); reported {
		t.Errorf("Version reported = true for a receiver that never sent VER, want false (v=%v)", v)
	}
	if got := p.Detector().Inferred(); got.AtLeast(V4_00) {
		t.Errorf("inferred version = %v, want below 4.00 for pre-4.10 traffic", got)
	}

	// A NMEA 4.10 receiver: GSV carries a signal id and GSA a system id.
	p2 := New()
	feed(t, p2,
		"$GPGSV,3,1,12,03,45,150,42,07,30,210,39,11,62,080,45,14,15,310,35,1*60",
		"$GNGSA,A,3,03,07,11,14,,,,,,,,,1.5,0.6,1.2,1*31",
		"$GNRMC,115522.000,A,4006.20885,N,11628.14498,E,0.000,0.50,041215,,,A,S*30",
	)
	got := p2.Detector().Inferred()
	if !got.AtLeast(V4_10) {
		t.Errorf("inferred version = %v, want at least 4.10 from signal id, system id, and nav status", got)
	}
	for _, f := range []Feature{FeatureSignalID, FeatureSystemID, FeatureNavStatus} {
		if seen, _ := p2.Detector().HasFeature(f); !seen {
			t.Errorf("detector did not observe %v", FeatureName(f))
		}
	}
}

func TestDetectorPrefersReportedVersion(t *testing.T) {
	// A VER sentence stating the revision beats anything inferred.
	p := New()
	feed(t, p,
		"$GPGSV,3,1,12,03,45,150,42,07,30,210,39,11,62,080,45,14,15,310,35,1*60",
		"$GPVER,Acme,GPS-1000,2.5,4.11*35",
	)
	v, reported := p.Version()
	if !reported {
		t.Error("Version reported = false after a VER with a version, want true")
	}
	if v != V4_11 {
		t.Errorf("Version = %v, want 4.11", v)
	}

	info := p.Receiver()
	if info.Manufacturer != "Acme" || info.Product != "GPS-1000" {
		t.Errorf("Receiver = %+v, want manufacturer Acme and product GPS-1000", info)
	}
	if !info.HasProtocolVersion || info.ProtocolVersion != V4_11 {
		t.Errorf("Receiver protocol = %v (present %v), want 4.11", info.ProtocolVersion, info.HasProtocolVersion)
	}
	if !strings.Contains(info.String(), "4.11") {
		t.Errorf("Receiver.String() = %q, want it to mention 4.11", info.String())
	}
}

func TestVERIgnoresBuildNumbers(t *testing.T) {
	// Field 3 is usually a build number. Reporting "1.00" as NMEA version
	// 1.00 would be nonsense, so it must be ignored.
	p := New()
	feed(t, p, "$GPVER,SiRF,GSiRF03,1000,1.00*0C")
	if _, reported := p.Version(); reported {
		t.Error("Version reported = true for a build number, want false")
	}
	if got := p.Receiver().Product; got != "GSiRF03" {
		t.Errorf("Product = %q, want GSiRF03", got)
	}
}

func TestVERReadsVersionFromSoftwareString(t *testing.T) {
	// Some receivers state the standard revision in the software field with
	// an explicit marker.
	p := New()
	feed(t, p, nmea_Frame("GPVER,Acme,GPS-1000,NMEA 4.11,2.5"))
	v, reported := p.Version()
	if !reported || v != V4_11 {
		t.Errorf("Version = %v, reported %v; want 4.11, true", v, reported)
	}
}

func TestVersionDetectorMinimum(t *testing.T) {
	// A caller can pin the floor, which matters for deciding whether an
	// absent field means "not implemented" or "pre-2.3".
	d := NewVersionDetector(V3_01)
	if got := d.Minimum(); got != V3_01 {
		t.Errorf("Minimum = %v, want 3.01", got)
	}
	if got := d.Inferred(); got != V3_01 {
		t.Errorf("Inferred = %v before any traffic, want the minimum 3.01", got)
	}
	// Traffic that implies an older version must not lower the floor.
	d.Observe(nil, "")
	if got := d.Inferred(); got != V3_01 {
		t.Errorf("Inferred = %v after empty traffic, want the minimum 3.01", got)
	}
}

// nmea_Frame is a small helper so a test can build a valid sentence without
// importing the root package under test by name.
func nmea_Frame(body string) string { return Frame(body) }
