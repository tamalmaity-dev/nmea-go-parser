package sentences

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	nmea "github.com/tamalmaity-dev/nmea-go-parser"
)

func TestInstallIntoPrivateRegistry(t *testing.T) {
	// A program that wants only some sentences can build a private registry.
	// Install must fill it without disturbing the default one.
	r := nmea.NewRegistry()
	Install(r)
	if r.Len() != len(all) {
		t.Errorf("private registry has %d decoders, want %d", r.Len(), len(all))
	}
	if nmea.DefaultRegistry.Len() != len(all) {
		t.Errorf("default registry has %d decoders, want %d", nmea.DefaultRegistry.Len(), len(all))
	}
}

// Helpers shared by the per-sentence test files.

func sampleLines(t *testing.T) []string {
	t.Helper()

	f, err := os.Open("../testdata/sample.nmea")
	if err != nil {
		t.Fatalf("opening sample data: %v", err)
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimPrefix(strings.TrimSpace(sc.Text()), "\uFEFF")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading sample data: %v", err)
	}
	return out
}
func decodeOne[T any](t *testing.T, line string) T {
	t.Helper()

	s, err := nmea.ParseSentence(line)
	if err != nil {
		t.Fatalf("ParseSentence(%q): %v", line, err)
	}
	if !s.ChecksumOK() {
		t.Fatalf("checksum of %q is %v, want valid", line, s.ChecksumState)
	}

	value, err := nmea.DefaultRegistry.Decode(s)
	if err != nil {
		t.Fatalf("decoding %q: %v", line, err)
	}
	typed, ok := value.(T)
	if !ok {
		t.Fatalf("decoding %q produced %T, want %T", line, value, *new(T))
	}
	return typed
}
func newTestParser() *nmea.Parser { return nmea.New() }
func firstLineOfType(t *testing.T, typ string) string {
	t.Helper()
	for _, line := range sampleLines(t) {
		s, err := nmea.ParseSentence(line)
		if err != nil {
			continue
		}
		if s.Type == typ {
			return line
		}
	}
	t.Fatalf("no sample sentence of type %s", typ)
	return ""
}
func feedSentence(t *testing.T, p *nmea.Parser, line string) {
	t.Helper()
	if _, err := p.ParseLine(line); err != nil && !errors.Is(err, nmea.ErrNotSentence) {
		t.Fatalf("ParseLine(%q): %v", line, err)
	}
}

func TestAllSampleSentencesDecode(t *testing.T) {
	// The proprietary sentences in the corpus have no decoder by design, so
	// they are expected to report ErrUnknownSentence.
	proprietary := map[string]bool{"UBX": true, "MTK": true, "GRM": true}

	seen := map[string]int{}
	for _, line := range sampleLines(t) {
		s, err := nmea.ParseSentence(line)
		if err != nil {
			t.Errorf("ParseSentence(%q): %v", line, err)
			continue
		}
		value, err := nmea.DefaultRegistry.Decode(s)

		if proprietary[s.Type] {
			if err == nil {
				t.Errorf("%s: proprietary sentence decoded to %T, want ErrUnknownSentence",
					s.Address(), value)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", s.Address(), err)
			continue
		}
		if value == nil {
			t.Errorf("%s: decoded to nil", s.Address())
		}
		seen[s.Type]++
	}

	// Every standard decoder must be exercised, so a decoder added without a
	// sample sentence is caught here.
	for _, want := range []string{
		// Position and fix quality.
		"GGA", "GNS", "RMC", "GLL", "VTG",
		// Constellations, satellites, and integrity.
		"GSA", "GSV", "GBS", "GRS", "GST", "ZDA",
		// Heading.
		"HDT", "THS", "HDG",
		// Speed, distance, and rate.
		"VHW", "VBW", "VLW", "ROT",
		// Navigation and cross-track.
		"WPL", "RTE", "RMB", "APB", "AAM", "BOD", "XTE", "BWC", "BWR", "WCV",
		// Environment and depth.
		"DPT", "MTW", "MWD", "MWV", "DTM",
		// Vessel.
		"OSD", "RSA",
		// Text and receiver identification.
		"TXT", "VER",
	} {
		if seen[want] == 0 {
			t.Errorf("no sample sentence exercised the %s decoder", want)
		}
	}
}

func TestTruncatedSentencesAreRejected(t *testing.T) {
	// A sentence cut short must produce an ErrFieldCount naming the
	// shortfall, not a zero value that a caller would mistake for real data.
	cases := []struct{ line, typ string }{
		{"GPWPL,4917.16,N,12310.64", "WPL"},
		{"GPAAM,A,A", "AAM"},
		{"GPBOD,097.0,T", "BOD"},
		{"GPRMB,A,0.66,L", "RMB"},
		{"GPRTE,1,1,c", "RTE"},
		{"GPZDA", "ZDA"},
		{"GPGLL,3723.2475,N", "GLL"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			s, err := nmea.ParseSentence("$" + tc.line + "*00")
			if err != nil {
				t.Fatalf("ParseSentence: %v", err)
			}
			_, err = nmea.DefaultRegistry.Decode(s)
			if !errors.Is(err, nmea.ErrFieldCount) {
				t.Errorf("decoding truncated %s = %v, want ErrFieldCount", tc.typ, err)
			}
		})
	}
}

func TestFixContributorsOnly(t *testing.T) {
	// Only the sentences that genuinely carry a position or satellite state
	// may affect the fix. A waypoint or route definition is data the program
	// loaded, not an observation, so folding it in would move the reported
	// position.
	mustNotContribute := []string{"WPL", "RTE", "AAM", "APB", "BOD", "RMB"}
	mustContribute := []string{"GGA", "RMC", "GLL", "GSA", "GSV", "ZDA", "VTG"}

	for _, typ := range mustContribute {
		p := newTestParser()
		line := firstLineOfType(t, typ)
		if _, err := p.ParseLine(line); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if p.Fix().SentenceCount == 0 {
			t.Errorf("%s did not contribute to the fix, but carries position or state data", typ)
		}
	}

	for _, typ := range mustNotContribute {
		p := newTestParser()
		line := firstLineOfType(t, typ)
		// A decode failure here is fine; the point is the fix must not move.
		_, _ = p.ParseLine(line)
		if p.Fix().SentenceCount != 0 {
			t.Errorf("%s contributed to the fix, but it is guidance or a definition", typ)
		}
		if p.Fix().HasPosition {
			t.Errorf("%s set a position on the fix, which it must never do", typ)
		}
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// published decodes a body from the documentation and fails the test if it
// cannot be read. The body is taken verbatim from the source document; only
// the leading '$' and the checksum are supplied, since the parser requires
// them and the documented checksum is not always correct.
func published[T any](t *testing.T, body string) T {
	t.Helper()

	sentence, err := nmea.ParseSentence(nmea.Frame(body))
	if err != nil {
		t.Fatalf("parsing published %q: %v", body, err)
	}
	value, err := nmea.DefaultRegistry.Decode(sentence)
	if err != nil {
		t.Fatalf("decoding published %q: %v", body, err)
	}
	typed, ok := value.(T)
	if !ok {
		t.Fatalf("published %q decoded to %T, want %T", body, value, *new(T))
	}
	return typed
}

// near compares floats with an absolute tolerance, so a rounding difference
// in the last decimal place of a published example is not a failure.
func near(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

// publishedErr is the counterpart to published for the layouts where the
// documented behaviour is a rejection: a field out of range, a mislabelled
// unit, or a value that cannot exist. It returns the decode error so the test
// can assert on it.
func publishedErr(t *testing.T, body string) error {
	t.Helper()

	sentence, err := nmea.ParseSentence(nmea.Frame(body))
	if err != nil {
		t.Fatalf("parsing published %q: %v", body, err)
	}
	_, err = nmea.DefaultRegistry.Decode(sentence)
	return err
}

// TestPublishedHDT and TestPublishedMTW and TestPublishedROT verify the
// simple two-field sentences.
func TestPublishedSimpleTwoFieldSentences(t *testing.T) {
	h := published[HDT](t, "GPHDT,274.07,T")
	if !near(h.Heading.Degrees, 274.07, 1e-9) {
		t.Errorf("HDT field 0 = %v, want 274.07", h.Heading.Degrees)
	}
	if !h.Heading.True {
		t.Error("HDT field 1 reference T was not reported as true")
	}

	m := published[MTW](t, "INMTW,17.9,C")
	if !near(m.Celsius, 17.9, 1e-9) {
		t.Errorf("MTW field 0 = %v, want 17.9", m.Celsius)
	}
	if m.Unit != "C" {
		t.Errorf("MTW field 1 = %q, want C", m.Unit)
	}

	r := published[ROT](t, "HEROT,0.0,A")
	if !near(r.DegreesPerMinute, 0, 1e-9) || !r.HasDegrees {
		t.Errorf("ROT field 0 = %v, want 0", r.DegreesPerMinute)
	}
	if r.Status != nmea.FlagYes {
		t.Errorf("ROT field 1 = %q, want A", r.Status)
	}
}

// TestPublishedChecksumErrors records the published example sentences whose
// printed checksum does not verify against their own body.
//
// These are not parser bugs. They are errors in the source documentation,
// and they are listed here so that anyone comparing this library against a
// published example knows the difference is in the document rather than in
// the decoding.
func TestPublishedChecksumErrors(t *testing.T) {
	// body -> the checksum the documentation prints.
	cases := map[string]string{
		"GPRMB,A,0.66,L,003,004,4917.24,N,12309.57,W,001.3,052.5,000.5,V": "0B",
		"GPAAM,A,A,0.10,N,WPTNME":                         "43",
		"GPBOD,099.3,T,105.6,M,POINTB":                    "01",
		"GPBWC,081837,,,,,,T,,M,,N":                       "13",
		"GPBOD,097.0,T,103.2,M,POINTB,POINTA":             "52",
		"GLGSV,3,3,09,88,07,028":                          "51",
		"GPGST,182141.000,15.5,15.3,7.2,21.8,0.9,0.5,0.8": "54",
		"GPGRS,024603.00,1,-1.8,-2.7,0.3,,,,,,,":          "6C",
		"GPGBS,125027,23.43,M,13.91,M,34.01,M":            "07",
		"GPXTE,V,V,,,N,S":                                 "43",
		"GPHDT,274.07,T":                                  "03",
		"INDPT,2.3,0.0":                                   "46",
		"INMTW,17.9,C":                                    "1B",
		"HEROT,0.0,A":                                     "2B",
		"GPDTM,W84,C":                                     "52",
		"WIMWD,302.4,T,289.6,M,10.5,N,5.4,M":              "6F",
		"GPZDA,160012.71,11,03,2004,-1,00":                "7D",
		"GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000": "18",
	}

	var wrong []string
	for body, printed := range cases {
		computed := fmt.Sprintf("%02X", nmea.Checksum(body))
		framed := "$" + body + "*" + printed
		if err := nmea.Validate(framed); err != nil {
			wrong = append(wrong, fmt.Sprintf("      %-72s printed *%s, computed *%s",
				body, printed, computed))
		}
	}
	if len(wrong) > 0 {
		// Sorted for a stable message.
		for i := 0; i < len(wrong); i++ {
			for j := i + 1; j < len(wrong); j++ {
				if wrong[j] < wrong[i] {
					wrong[i], wrong[j] = wrong[j], wrong[i]
				}
			}
		}
		t.Logf("published examples whose printed checksum does not verify "+
			"(%d of %d checked). The bodies are correct; only the printed "+
			"checksums in the documentation are wrong:\n%s",
			len(wrong), len(cases), strings.Join(wrong, "\n"))
	}
}
