package value

import "testing"

// TestTalkerConstellations checks the talker prefixes against the systems the
// documentation assigns them. A misattributed talker is worse than an
// unattributed one, because a caller has no way to tell that the answer is
// wrong.
func TestTalkerConstellations(t *testing.T) {
	for _, tc := range []struct {
		talker string
		want   Constellation
	}{
		{"GP", ConstellationGPS},
		{"GL", ConstellationGLONASS},
		{"GA", ConstellationGalileo},
		{"GB", ConstellationBeiDou},
		{"BD", ConstellationBeiDou},
		{"GQ", ConstellationQZSS},
		{"QZ", ConstellationQZSS},
		{"PQ", ConstellationQZSS},
		{"GI", ConstellationNavIC},
		{"IR", ConstellationNavIC},
		// GN is a fused solution, not a system, and conflating the two is
		// what makes a sky view say "Mixed: 11 satellites" when it means
		// "eleven satellites from systems I am not telling you about".
		{"GN", ConstellationMixed},
		// A talker outside the GNSS set is normal, not an error.
		{"XX", ConstellationUnknown},
		{"", ConstellationUnknown},
	} {
		if got := TalkerConstellation(tc.talker); got != tc.want {
			t.Errorf("TalkerConstellation(%q) = %v, want %v", tc.talker, got, tc.want)
		}
	}
	// Case must not matter, because a receiver that lowercases its talker is
	// unusual but not broken.
	if got := TalkerConstellation("gp"); got != ConstellationGPS {
		t.Errorf(`TalkerConstellation("gp") = %v, want GPS`, got)
	}
}

func TestConstellationNames(t *testing.T) {
	for _, tc := range []struct {
		c    Constellation
		want string
	}{
		{ConstellationGPS, "GPS"},
		{ConstellationGLONASS, "GLONASS"},
		{ConstellationGalileo, "Galileo"},
		{ConstellationBeiDou, "BeiDou"},
		{ConstellationQZSS, "QZSS"},
		{ConstellationNavIC, "NavIC"},
		{ConstellationSBAS, "SBAS"},
		{ConstellationMixed, "Mixed"},
		{ConstellationUnknown, "Unknown"},
	} {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("Constellation(%d).String() = %q, want %q", tc.c, got, tc.want)
		}
	}
	if ConstellationMixed.IsMixed() != true {
		t.Error("ConstellationMixed.IsMixed() = false, want true")
	}
	if ConstellationGPS.IsMixed() != false {
		t.Error("ConstellationGPS.IsMixed() = true, want false")
	}
}

// TestSystemIDOfRoundTrips checks the reverse mapping against the forward one.
// They are the same table read in two directions, so any disagreement is a bug
// in one of them.
func TestSystemIDOfRoundTrips(t *testing.T) {
	for _, c := range []Constellation{
		ConstellationGPS, ConstellationGLONASS, ConstellationGalileo,
		ConstellationBeiDou, ConstellationQZSS, ConstellationNavIC,
	} {
		sys := SystemIDOf(c)
		if sys == SystemIDNone {
			t.Errorf("SystemIDOf(%v) = none, want a system id", c)
			continue
		}
		if got := sys.Constellation(); got != c {
			t.Errorf("%v -> %v -> %v, want the round trip to return %v", c, sys, got, c)
		}
	}
	if got := SystemIDOf(ConstellationMixed); got != SystemIDNone {
		t.Errorf("SystemIDOf(Mixed) = %v, want none: a fused set names no system", got)
	}
	// SBAS transmits system id 1, the same as GPS. Reporting a system id of
	// its own would be a fiction the wire does not support.
	if got := SystemIDOf(ConstellationSBAS); got != SystemIDGPS {
		t.Errorf("SystemIDOf(SBAS) = %v, want GPS: SBAS shares system id 1", got)
	}
}

// TestIsSBASPRN pins the only discriminator SBAS has. With no talker and no
// system id of its own, the satellite number is all that separates WAAS from
// GPS.
func TestIsSBASPRN(t *testing.T) {
	// 120 to 158 is the SBAS range; 33 to 64 is where some receivers place
	// them under the older, non-strict numbering.
	for _, prn := range []int{120, 121, 138, 158} {
		if !IsSBASPRN(prn) {
			t.Errorf("IsSBASPRN(%d) = false, want true", prn)
		}
	}
	for _, prn := range []int{0, 1, 32, 64, 119, 159, 200} {
		if IsSBASPRN(prn) {
			t.Errorf("IsSBASPRN(%d) = true, want false", prn)
		}
	}
}

// TestSignalDigitBandPerSystem is the table the whole signal feature rests on.
// The values are the NMEA 4.11 assignments; a band read with the wrong system's
// table is a plausible wrong answer, which is why each row is pinned.
func TestSignalDigitBandPerSystem(t *testing.T) {
	for _, tc := range []struct {
		digit int
		sys   SystemID
		want  SignalID
	}{
		// Digit 1 is the one that collides across every system.
		{1, SystemIDGPS, SignalIDL1C},
		{1, SystemIDGalileo, SignalIDE1a},
		{1, SystemIDBeiDou, SignalIDB1I},
		{1, SystemIDGLONASS, SignalIDL1},
		{1, SystemIDNavIC, SignalIDL5A},
		{1, SystemIDQZSS, SignalIDL1C},
		// Galileo reuses 2, 3, 4, and 5 for bands GPS spells differently.
		{2, SystemIDGalileo, SignalIDE5b},
		{3, SystemIDGalileo, SignalIDE5a},
		{4, SystemIDGalileo, SignalIDE6a},
		{5, SystemIDGalileo, SignalIDE6bc},
		// 7 is E1C for Galileo and L5-I for GPS.
		{7, SystemIDGalileo, SignalIDE1a},
		{7, SystemIDGPS, SignalIDL5I},
		// GLONASS L2 sits on 3, not 2.
		{2, SystemIDGLONASS, SignalID(0)}, // undefined
		{3, SystemIDGLONASS, SignalIDL2},
		// BeiDou bands.
		{2, SystemIDBeiDou, SignalIDB2I},
		{3, SystemIDBeiDou, SignalIDB1C},
		{5, SystemIDBeiDou, SignalIDB2a},
		{8, SystemIDBeiDou, SignalIDB3I},
		// QZSS adds L1S on 4.
		{4, SystemIDQZSS, SignalIDL1S},
		// 0 means "all signals" for every system that has a table.
		{0, SystemIDGPS, SignalIDAll},
		{0, SystemIDGalileo, SignalIDAll},
		{0, SystemIDNavIC, SignalIDAll},
	} {
		digit := SignalDigit(tc.digit)
		if tc.want == SignalID(0) {
			if _, ok := digit.Band(tc.sys); ok {
				t.Errorf("digit %d under %v: Band returned a band, want none", tc.digit, tc.sys)
			}
			continue
		}
		got, ok := digit.Band(tc.sys)
		if !ok {
			t.Errorf("digit %d under %v: Band reported not ok, want %v", tc.digit, tc.sys, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("digit %d under %v = %v (%q), want %v (%q)",
				tc.digit, tc.sys, got, got, tc.want, tc.want)
		}
	}
}

// TestSBASResolvesThroughGPS checks that SBAS gets a band name even though it
// has no system id of its own: it transmits 1, whose only signal is L1 C/A.
func TestSBASResolvesThroughGPS(t *testing.T) {
	digit, _ := ParseSignalID("1")
	band, ok := digit.Band(SystemIDOf(ConstellationSBAS))
	if !ok {
		t.Fatal("SBAS signal digit 1 resolved to no band, want L1 C/A")
	}
	if band != SignalIDL1C {
		t.Errorf("SBAS band = %v, want L1 C/A", band)
	}
}

func TestSignalDigitString(t *testing.T) {
	for d := 0; d <= 9; d++ {
		digit, _ := ParseSignalID(itoa(d))
		if got := digit.String(); got != itoa(d) {
			t.Errorf("SignalDigit(%d).String() = %q, want %q", d, got, itoa(d))
		}
	}
	// An absent digit must not print as 0, because 0 on the wire means "all
	// signals" and the two are different facts.
	if got := DigitNone.String(); got != "?" {
		t.Errorf("DigitNone.String() = %q, want %q", got, "?")
	}
}

// itoa avoids pulling strconv in for a test helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
