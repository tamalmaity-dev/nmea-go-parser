package nmea

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseCoordinate(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		hemi       string
		axis       Axis
		wantDeg    int
		wantMin    float64
		wantSigned float64
		wantErr    error
	}{
		{name: "latitude north", value: "4807.038", hemi: "N", axis: AxisLatitude,
			wantDeg: 48, wantMin: 7.038, wantSigned: 48.1173},
		{name: "latitude south", value: "3345.6800", hemi: "S", axis: AxisLatitude,
			wantDeg: 33, wantMin: 45.68, wantSigned: -33.7613},
		{name: "longitude east three digits", value: "01131.000", hemi: "E", axis: AxisLongitude,
			wantDeg: 11, wantMin: 31.0, wantSigned: 11.516667},
		{name: "longitude west three digits", value: "12311.12", hemi: "W", axis: AxisLongitude,
			wantDeg: 123, wantMin: 11.12, wantSigned: -123.185333},
		{name: "integer minutes", value: "4916.45", hemi: "N", axis: AxisLatitude,
			wantDeg: 49, wantMin: 16.45, wantSigned: 49.2742},
		{name: "blank field", value: "", hemi: "N", axis: AxisLatitude, wantErr: nil},
		{name: "latitude at 90", value: "9000.000", hemi: "N", axis: AxisLatitude,
			wantDeg: 90, wantMin: 0, wantSigned: 90},
		{name: "latitude over 90", value: "9100.000", hemi: "N", axis: AxisLatitude,
			wantErr: ErrFieldRange},
		{name: "longitude over 180", value: "18100.000", hemi: "E", axis: AxisLongitude,
			wantErr: ErrFieldRange},
		{name: "minutes over 60", value: "4860.000", hemi: "N", axis: AxisLatitude,
			wantErr: ErrFieldRange},
		{name: "not a number", value: "abcd.mm", hemi: "N", axis: AxisLatitude,
			wantErr: ErrFieldValue},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCoordinate(tc.value, tc.hemi, tc.axis)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseCoordinate(%q, %q) error = %v, want %v",
						tc.value, tc.hemi, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCoordinate(%q, %q) unexpected error: %v", tc.value, tc.hemi, err)
			}
			if tc.value == "" {
				if got.Valid {
					t.Error("blank field produced a valid coordinate, want invalid")
				}
				if _, err := got.Decimal(); !errors.Is(err, ErrNotValid) {
					t.Errorf("Decimal on invalid coordinate = %v, want ErrNotValid", err)
				}
				return
			}
			if !got.Valid {
				t.Fatal("coordinate is invalid, want valid")
			}
			if got.Degrees != tc.wantDeg {
				t.Errorf("Degrees = %d, want %d", got.Degrees, tc.wantDeg)
			}
			if math.Abs(got.Minutes-tc.wantMin) > 1e-6 {
				t.Errorf("Minutes = %v, want %v", got.Minutes, tc.wantMin)
			}
			dec, err := got.Decimal()
			if err != nil {
				t.Fatalf("Decimal: %v", err)
			}
			if math.Abs(dec-tc.wantSigned) > 1e-3 {
				t.Errorf("Decimal = %v, want %v", dec, tc.wantSigned)
			}
		})
	}
}

func TestCoordinateRoundTrip(t *testing.T) {
	// A decoded value must re-encode to the exact text it arrived as, or a
	// program that reads a waypoint out of a receiver and writes it back
	// would corrupt the route. The leading zero on the longitude is the
	// detail that would break a naive formatter.
	cases := []struct {
		value, hemi string
		axis        Axis
	}{
		{value: "4807.038", hemi: "N", axis: AxisLatitude},
		{value: "01131.000", hemi: "E", axis: AxisLongitude},
		{value: "12311.12", hemi: "W", axis: AxisLongitude},
		{value: "4917.16", hemi: "N", axis: AxisLatitude},
	}
	for _, tc := range cases {
		c, err := ParseCoordinate(tc.value, tc.hemi, tc.axis)
		if err != nil {
			t.Fatalf("ParseCoordinate(%q): %v", tc.value, err)
		}
		if got := c.ValueString(); got != tc.value {
			t.Errorf("ValueString round trip = %q, want %q", got, tc.value)
		}
		if got, want := c.String(), tc.value+tc.hemi; got != want {
			t.Errorf("String round trip = %q, want %q", got, want)
		}
	}
}

func TestCoordinateNoHemisphere(t *testing.T) {
	// A coordinate with a magnitude but no hemisphere cannot be turned into
	// a position, and must not silently become a positive one.
	c, err := ParseCoordinate("4807.038", "", AxisLatitude)
	if err != nil {
		t.Fatalf("ParseCoordinate: %v", err)
	}
	if !c.Valid {
		t.Fatal("coordinate should be valid, only the hemisphere is missing")
	}
	if _, err := c.Decimal(); !errors.Is(err, ErrFieldValue) {
		t.Errorf("Decimal without hemisphere = %v, want ErrFieldValue", err)
	}
}

func TestParseTOD(t *testing.T) {
	tests := []struct {
		in      string
		want    TOD
		wantErr error
	}{
		{in: "123519", want: TOD{Hour: 12, Minute: 35, Second: 19, Valid: true, Available: true}},
		{in: "123519.00", want: TOD{Hour: 12, Minute: 35, Second: 19, Valid: true, Available: true}},
		{in: "201530.00", want: TOD{Hour: 20, Minute: 15, Second: 30, Valid: true, Available: true}},
		{in: "235959.999", want: TOD{Hour: 23, Minute: 59, Second: 59, Fraction: 0.999,
			Valid: true, Available: true}},
		{in: "235960", want: TOD{Hour: 23, Minute: 59, Second: 60, Valid: true, Available: true}}, // leap second
		{in: "", want: TOD{Available: false}},
		{in: "9999", wantErr: ErrFieldValue},
		{in: "253519", wantErr: ErrFieldRange},
		{in: "123560", want: TOD{Hour: 12, Minute: 35, Second: 60, Valid: true, Available: true}},
		{in: "246000", wantErr: ErrFieldRange},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseTOD(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseTOD(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTOD(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseTOD(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in      string
		want    Date
		wantErr error
	}{
		{in: "230394", want: Date{Day: 23, Month: 3, Year: 1994, Valid: true, Available: true}},
		{in: "091202", want: Date{Day: 9, Month: 12, Year: 2002, Valid: true, Available: true}},
		{in: "010100", want: Date{Day: 1, Month: 1, Year: 2000, Valid: true, Available: true}},
		{in: "010180", want: Date{Day: 1, Month: 1, Year: 1980, Valid: true, Available: true}},
		{in: "", want: Date{}},
		{in: "320394", wantErr: ErrFieldValue}, // 32 March
		{in: "300294", wantErr: ErrFieldValue}, // 30 February
		{in: "290204", want: Date{Day: 29, Month: 2, Year: 2004, Valid: true, Available: true}}, // leap year
		{in: "290296", want: Date{Day: 29, Month: 2, Year: 1996, Valid: true, Available: true}}, // also a leap year
		{in: "290290", wantErr: ErrFieldValue},                                                  // ddmmyy = 29 Feb 1990, and 1990 is not a leap year
		{in: "300294", wantErr: ErrFieldValue},                                                  // 30 February, in any year
		{in: "12345", wantErr: ErrFieldValue},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseDate(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseDate(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseDate(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestDateCombine(t *testing.T) {
	d, err := ParseDate("230394")
	if err != nil {
		t.Fatal(err)
	}
	tod, err := ParseTOD("123519")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := d.Combine(tod)
	if !ok {
		t.Fatal("Combine reported no time")
	}
	want := time.Date(1994, time.March, 23, 12, 35, 19, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Combine = %v, want %v", got, want)
	}

	// A receiver that has sent a time but no date cannot produce a
	// timestamp, and must not default the date to the epoch.
	if _, ok := (Date{}).Combine(tod); ok {
		t.Error("Combine with no date reported a time, want false")
	}
}

func TestParseFixQuality(t *testing.T) {
	tests := []struct {
		in      string
		want    FixQuality
		fixed   bool
		wantErr error
	}{
		{in: "0", want: QualityNone, fixed: false},
		{in: "1", want: QualityGPS, fixed: true},
		{in: "2", want: QualityDGPS, fixed: true},
		{in: "4", want: QualityRTKFixed, fixed: true},
		{in: "5", want: QualityRTKFloat, fixed: true},
		{in: "6", want: QualityEstimated, fixed: false},
		{in: "7", want: QualityManual, fixed: false},
		{in: "8", want: QualitySimulation, fixed: false},
		{in: "", want: QualityNone, fixed: false},
		{in: "9", wantErr: ErrFieldValue},
		{in: "x", wantErr: ErrFieldValue},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseFixQuality(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseFixQuality(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFixQuality(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseFixQuality(%q) = %v, want %v", tc.in, got, tc.want)
			}
			if got.Fixed() != tc.fixed {
				t.Errorf("ParseFixQuality(%q).Fixed() = %v, want %v", tc.in, got.Fixed(), tc.fixed)
			}
		})
	}
}

func TestParseSide(t *testing.T) {
	for in, want := range map[string]Side{
		"L": SideLeft, "R": SideRight, "": SideUnknown, "l": SideLeft, "r": SideRight,
	} {
		got, err := ParseSide(in)
		if err != nil {
			t.Fatalf("ParseSide(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseSide(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseSide("X"); !errors.Is(err, ErrFieldValue) {
		t.Errorf("ParseSide(\"X\") error = %v, want ErrFieldValue", err)
	}
}

func TestParseNavigationStatus(t *testing.T) {
	for in, want := range map[string]NavigationStatus{
		"A": StatusValid, "V": StatusInvalid, "": StatusInvalid,
	} {
		got, err := ParseNavigationStatus(in)
		if err != nil {
			t.Fatalf("ParseNavigationStatus(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseNavigationStatus(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseNavigationStatus("Q"); !errors.Is(err, ErrFieldValue) {
		t.Errorf("ParseNavigationStatus(\"Q\") error = %v, want ErrFieldValue", err)
	}
}

func TestParseSentence(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantTalker string
		wantType   string
		wantFields []string
		wantSum    uint8
		wantState  ChecksumStatus
		wantErr    error
	}{
		{
			// The published GGA example. Its checksum, and the ones below it,
			// are the strongest available check that the XOR runs over the
			// right bytes: these are values the standard itself prints.
			name:       "gga with checksum",
			in:         "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18",
			wantTalker: "GP", wantType: "GGA", wantSum: 0x18, wantState: ChecksumValid,
			wantFields: []string{"161229.487", "3723.2475", "N", "12158.3416", "W",
				"1", "07", "1.0", "9.0", "M", "", "", "", "0000"},
		},
		{
			name:       "gn talker",
			in:         "$GNGGA,103607.00,5327.03942,N,00214.42462,W,1,05,4.6,71.0,M,50.5,M,,*5C",
			wantTalker: "GN", wantType: "GGA", wantSum: 0x5C, wantState: ChecksumValid,
		},
		{
			name: "proprietary ubx", in: "$PUBX,00*33",
			wantTalker: "", wantType: "UBX", wantSum: 0x33, wantState: ChecksumValid,
			wantFields: []string{"00"},
		},
		{
			// A proprietary address is not a fixed width: the message id
			// belongs to the address, not to the fields.
			name: "proprietary mtk with message id", in: "$PMTK001,1,0*32",
			wantTalker: "", wantType: "MTK", wantSum: 0x32, wantState: ChecksumValid,
			wantFields: []string{"1", "0"},
		},
		{
			name: "no checksum", in: "$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,",
			wantTalker: "GP", wantType: "RMC", wantState: ChecksumAbsent,
		},
		{
			name:       "bad checksum",
			in:         "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,-,,*00",
			wantTalker: "GP", wantType: "GGA", wantSum: 0x00, wantState: ChecksumInvalid,
		},
		{
			name: "malformed checksum", in: "$GPGGA,123519*XY",
			wantTalker: "GP", wantType: "GGA", wantState: ChecksumMalformed,
		},
		{
			name: "checksum with one digit", in: "$GPGGA,123519*4",
			wantTalker: "GP", wantType: "GGA", wantState: ChecksumMalformed,
		},
		{
			name:       "crlf trimmed",
			in:         "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n",
			wantTalker: "GP", wantType: "GGA", wantSum: 0x18, wantState: ChecksumValid,
		},
		{
			name:       "nul padded",
			in:         "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\x00\x00",
			wantTalker: "GP", wantType: "GGA", wantSum: 0x18, wantState: ChecksumValid,
		},
		{
			name:       "leading garbage skipped",
			in:         "junk$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18",
			wantTalker: "GP", wantType: "GGA", wantSum: 0x18, wantState: ChecksumValid,
		},
		{
			name: "not a sentence", in: "hello world", wantErr: ErrNotSentence,
		},
		{
			name: "empty", in: "", wantErr: ErrNotSentence,
		},
		{
			name: "dollar only", in: "$", wantErr: ErrShortSentence,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSentence(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseSentence(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSentence(%q) unexpected error: %v", tc.in, err)
			}
			if got.Talker != tc.wantTalker {
				t.Errorf("Talker = %q, want %q", got.Talker, tc.wantTalker)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tc.wantType)
			}
			if got.ChecksumState != tc.wantState {
				t.Errorf("ChecksumState = %v, want %v", got.ChecksumState, tc.wantState)
			}
			if tc.wantSum != 0 && got.Checksum != tc.wantSum {
				t.Errorf("Checksum = %02X, want %02X", got.Checksum, tc.wantSum)
			}
			if got.ChecksumOK() != tc.wantState.OK() {
				t.Errorf("ChecksumOK = %v, want %v", got.ChecksumOK(), tc.wantState.OK())
			}
			if tc.wantFields != nil {
				if len(got.Fields) != len(tc.wantFields) {
					t.Fatalf("Fields = %d (%q), want %d", len(got.Fields), got.Fields, len(tc.wantFields))
				}
				for i := range tc.wantFields {
					if got.Fields[i] != tc.wantFields[i] {
						t.Errorf("Fields[%d] = %q, want %q", i, got.Fields[i], tc.wantFields[i])
					}
				}
			}
		})
	}
}

func TestChecksum(t *testing.T) {
	// Values published in the standard, used here as an independent check
	// that the XOR is computed over the right bytes.
	tests := []struct{ body, want string }{
		{"GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000", "18"},
		{"GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,", "10"},
		{"GPGSA,A,3,07,02,26,27,09,04,15,,,,,,1.8,1.0,1.5", "33"},
		{"GPGSV,2,1,07,07,79,048,42,02,51,062,43,26,36,256,42,27,27,138,42", "71"},
		{"GPGSV,2,2,07,09,23,313,42,04,19,159,41,15,12,041,42", "41"},
		{"GPVTG,309.62,T, ,M,0.13,N,0.2,K,A", "23"},
		{"GPGLL,3723.2475,N,12158.3416,W,161229.487,A,A", "41"},
		{"GPZDA,160012.71,11,03,2004,-1,00", "7D"},
		{"GPRTE,1,1,c,0", "07"},
		{"PUBX,00", "33"},
	}
	for _, tc := range tests {
		got := Checksum(tc.body)
		if want := fmt.Sprintf("%02X", got); want != tc.want {
			t.Errorf("Checksum(%q) = %s, want %s", tc.body, want, tc.want)
		}
	}
}

func TestFrameAndValidate(t *testing.T) {
	body := "GPWPL,4917.16,N,12310.64,W,003"
	framed := Frame(body)
	if want := "$GPWPL,4917.16,N,12310.64,W,003*65"; framed != want {
		t.Errorf("Frame = %q, want %q", framed, want)
	}
	// Frame and Validate must be inverses, or a program cannot round-trip a
	// waypoint it read out of a receiver.
	if err := Validate(framed); err != nil {
		t.Errorf("Validate(Frame(...)) = %v, want nil", err)
	}
	if !Valid(framed) {
		t.Error("Valid(Frame(...)) = false, want true")
	}
}

func TestSentenceAccessors(t *testing.T) {
	s, err := ParseSentence("$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47")
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Field(0); got != "123519" {
		t.Errorf("Field(0) = %q, want 123519", got)
	}
	// Out of range must be empty, not a panic, because decoders index fields
	// that a truncated sentence may not have.
	if got := s.Field(999); got != "" {
		t.Errorf("Field(999) = %q, want empty", got)
	}
	if !s.Blank(12) {
		t.Error("Blank(12) = false, want true for the empty DGPS age field")
	}
	if s.Blank(0) {
		t.Error("Blank(0) = true, want false")
	}
	if !s.HasFields(14) {
		t.Error("HasFields(14) = false, want true")
	}
	if s.HasFields(99) {
		t.Error("HasFields(99) = true, want false")
	}
	if got := s.Address(); got != "GPGGA" {
		t.Errorf("Address = %q, want GPGGA", got)
	}

	if _, err := s.Float(999); !errors.Is(err, ErrEmptyField) {
		t.Errorf("Float on out-of-range field = %v, want ErrEmptyField", err)
	}
	if _, err := s.Int(999); !errors.Is(err, ErrEmptyField) {
		t.Errorf("Int on out-of-range field = %v, want ErrEmptyField", err)
	}
}

func TestLineSplitterAcrossReads(t *testing.T) {
	// The critical case: a sentence split across three reads, which is what a
	// serial port does constantly and what a naive strings.Split would get
	// wrong.
	ls := NewLineSplitter(0)
	var got []string

	got = append(got, ls.Feed([]byte("$GPGGA,1235"))...)
	got = append(got, ls.Feed([]byte("19,4807.038,N*47\r"))...)
	got = append(got, ls.Feed([]byte("\n$GPRMC,123519,A*6A\n"))...)

	if len(got) != 2 {
		t.Fatalf("got %d lines (%q), want 2", len(got), got)
	}
	if !strings.HasPrefix(got[0], "$GPGGA") || !strings.HasSuffix(got[0], "*47") {
		t.Errorf("line 0 = %q, want the complete GGA with checksum", got[0])
	}
	if got[1] != "$GPRMC,123519,A*6A" {
		t.Errorf("line 1 = %q, want the RMC without the CRLF", got[1])
	}
}

func TestLineSplitterTerminators(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "crlf", input: "a\r\nb\r\n", want: []string{"a", "b"}},
		{name: "lf only", input: "a\nb\n", want: []string{"a", "b"}},
		{name: "cr only", input: "a\rb\r", want: []string{"a", "b"}},
		{name: "mixed", input: "a\r\nb\nc\r", want: []string{"a", "b", "c"}},
		{name: "blank lines collapsed", input: "a\r\n\r\n\r\nb\r\n", want: []string{"a", "b"}},
		{name: "no trailing terminator", input: "a\nb", want: []string{"a"}},
		{name: "whitespace only lines dropped", input: "a\n   \n\x00\nb\n", want: []string{"a", "b"}},
		{name: "empty input", input: "", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewLineSplitter(0).Feed([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestLineSplitterCloseFlushesRemainder(t *testing.T) {
	// A recorded file or a port closed mid-sentence leaves a partial line.
	// Flushing it is better than discarding a whole sentence over one missing
	// newline.
	ls := NewLineSplitter(0)
	if got := ls.Feed([]byte("first\nsecond")); len(got) != 1 {
		t.Fatalf("Feed returned %q, want one line", got)
	}
	got := ls.Close()
	if len(got) != 1 || got[0] != "second" {
		t.Fatalf("Close = %q, want [second]", got)
	}
	if again := ls.Close(); again != nil {
		t.Errorf("second Close = %q, want nil", again)
	}
}

func TestLineSplitterOverflow(t *testing.T) {
	// A corrupt stream with no terminator must not grow the buffer without
	// bound; the splitter drops the excess, counts it, and resynchronises on
	// the next terminator.
	ls := NewLineSplitter(16)
	if lines := ls.Feed([]byte(strings.Repeat("x", 100))); len(lines) != 0 {
		t.Fatalf("got %q, want no lines from unterminated data", lines)
	}
	if ls.Overflows() == 0 {
		t.Error("Overflows = 0, want a non-zero count")
	}
	lines := ls.Feed([]byte("\n$valid\n"))
	var found bool
	for _, l := range lines {
		if l == "$valid" {
			found = true
		}
	}
	if !found {
		t.Errorf("lines = %q, want the valid sentence to survive resynchronisation", lines)
	}
}

func TestSentenceProprietary(t *testing.T) {
	s, err := ParseSentence("$PGRME,15.0,M,45.0,M,25.0,M*22")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Proprietary {
		t.Error("Proprietary = false, want true")
	}
	if s.Type != "GRM" {
		t.Errorf("Type = %q, want GRM", s.Type)
	}
	if s.Talker != "" {
		t.Errorf("Talker = %q, want empty for a proprietary sentence", s.Talker)
	}
}

// TestStatsSentencesMatchesTheFix guards the counter that was documented but
// never incremented, so it read zero for every stream. It has to agree with
// Fix.SentenceCount: both count the sentences that moved the aggregate state,
// and a diagnostic that reports zero while the fix is full of data is worse
// than no diagnostic.
func TestStatsSentencesMatchesTheFix(t *testing.T) {
	// A stream with both contributing and non-contributing sentences. The
	// waypoint and the route are definitions the program loaded, not
	// observations, so neither may be counted.
	const stream = "" +
		"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n" +
		"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10\r\n" +
		"$GPWPL,4917.16,N,12310.64,W,003*65\r\n" +
		"$GPGSA,A,3,04,05,,09,12,,,24,,,,,2.5,1.3,2.1*39\r\n" +
		"$GPRTE,2,1,c,POINTB,POINTC*3E\r\n"

	p := New()
	if err := p.Consume(context.Background(), strings.NewReader(stream)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	stats := p.Stats()
	if stats.Sentences == 0 {
		t.Fatal("Stats.Sentences = 0 for a stream with three contributing sentences")
	}
	if got, want := stats.Sentences, p.Fix().SentenceCount; got != want {
		t.Errorf("Stats.Sentences = %d, Fix.SentenceCount = %d; they count the same thing", got, want)
	}
	// GGA, RMC, and GSA contribute. WPL and RTE do not.
	if stats.Sentences != 3 {
		t.Errorf("Stats.Sentences = %d, want 3: GGA, RMC, and GSA only", stats.Sentences)
	}
	if stats.Lines != 5 {
		t.Errorf("Stats.Lines = %d, want 5", stats.Lines)
	}
}

// TestEventAs covers the shapes a decoder may hand back. The pointer case is
// the one that used to panic: As reassigned the local *type* variable and
// then called Set with the *value*, which reflect rejects.
func TestEventAs(t *testing.T) {
	type target struct{ N int }

	tests := []struct {
		name    string
		ev      Event
		wantErr bool
	}{
		{"value type", Event{Value: target{N: 7}}, false},
		{"pointer type", Event{Value: &target{N: 7}}, false},
		{"nil pointer", Event{Value: (*target)(nil)}, true},
		{"wrong type", Event{Value: "not a target"}, true},
		{"nil value with error", Event{Err: errors.New("boom")}, true},
		{"nil value without error", Event{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got target
			err := tc.ev.As(&got)
			if tc.wantErr {
				if err == nil {
					t.Fatal("As error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("As error = %v, want nil", err)
			}
			if got.N != 7 {
				t.Errorf("As filled %+v, want N=7", got)
			}
		})
	}
}

func TestEventAsTargetValidation(t *testing.T) {
	type target struct{ N int }
	ev := Event{Value: target{N: 1}}

	if err := ev.As(nil); err == nil {
		t.Error("As(nil) error = nil, want an error")
	}
	var notPointer target
	if err := ev.As(notPointer); err == nil {
		t.Error("As(value) error = nil, want an error")
	}
	var nilPointer *target
	if err := ev.As(nilPointer); err == nil {
		t.Error("As((*target)(nil)) error = nil, want an error")
	}
}

// TestFeedBytesReusesSlice pins the allocation behaviour readLoop depends on.
// feedBytes used to return nil on every path, so the lines[:0] passed back in
// was always nil[:0] and Append allocated a fresh backing array per read.
func TestFeedBytesReusesSlice(t *testing.T) {
	p := New()
	data := []byte("$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n")

	dst := make([]string, 0, 8)
	first := p.feedBytes(dst, data)
	if len(first) != 1 {
		t.Fatalf("feedBytes produced %d lines, want 1", len(first))
	}
	if got, want := reflect.ValueOf(first).Pointer(), reflect.ValueOf(dst).Pointer(); got != want {
		t.Error("feedBytes did not reuse the dst backing array")
	}

	second := p.feedBytes(first[:0], data)
	if got, want := reflect.ValueOf(second).Pointer(), reflect.ValueOf(dst).Pointer(); got != want {
		t.Error("the second feedBytes did not reuse the backing array")
	}
	if len(second) != 1 {
		t.Fatalf("the second feedBytes produced %d lines, want 1", len(second))
	}
}

// TestFeedBytesKeepsCapacityOnPartial covers the path where no complete line
// arrived: dst comes back unchanged and must still hold its capacity.
func TestFeedBytesKeepsCapacityOnPartial(t *testing.T) {
	p := New()
	dst := make([]string, 0, 8)
	got := p.feedBytes(dst, []byte("$GPGGA,123519"))
	if got != nil && len(got) != 0 {
		t.Fatalf("feedBytes on a partial line returned %q, want empty", got)
	}
	if cap(got) != cap(dst) {
		t.Errorf("cap = %d, want %d: the partial-line path dropped the capacity", cap(got), cap(dst))
	}
}

// TestHistoryRingWrapsInOrder pins the ring buffer's ordering. The buffer
// used to shift the slice left on every insert once full, which is where an
// off-by-one in the head index would show up.
func TestHistoryRingWrapsInOrder(t *testing.T) {
	p := New(WithKeepHistory(3))
	for i := 1; i <= 5; i++ {
		p.Feed([]byte(Frame(fmt.Sprintf("GPTXT,01,01,msg%d", i)) + "\r\n"))
	}

	got := p.History()
	if len(got) != 3 {
		t.Fatalf("History() returned %d events, want 3", len(got))
	}
	// The first two were evicted; the rest come back oldest first.
	for i, ev := range got {
		want := fmt.Sprintf("msg%d", i+3)
		if !strings.Contains(ev.Sentence.Raw, want) {
			t.Errorf("History()[%d] = %q, want it to contain %q", i, ev.Sentence.Raw, want)
		}
	}
}

// TestHistoryRingExactFill covers the boundary where the buffer fills but
// does not yet wrap: the head is still zero and the order must be plain.
func TestHistoryRingExactFill(t *testing.T) {
	p := New(WithKeepHistory(3))
	for i := 1; i <= 3; i++ {
		p.Feed([]byte(Frame(fmt.Sprintf("GPTXT,01,01,msg%d", i)) + "\r\n"))
	}

	got := p.History()
	if len(got) != 3 {
		t.Fatalf("History() returned %d events, want 3", len(got))
	}
	for i, ev := range got {
		want := fmt.Sprintf("msg%d", i+1)
		if !strings.Contains(ev.Sentence.Raw, want) {
			t.Errorf("History()[%d] = %q, want it to contain %q", i, ev.Sentence.Raw, want)
		}
	}
}
