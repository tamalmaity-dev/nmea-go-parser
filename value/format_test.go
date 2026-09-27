package value_test

import (
	"testing"

	value "github.com/tamalmaity-dev/nmea-go-parser/value"
)

// TestFormatHelpers pins the conversions advertised in the README. They are
// pure functions on the public surface, so a change in their output is a
// change to what a caller's log or UI shows.
func TestFormatHelpers(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"FormatGPS", value.FormatGPS(48.1173), "4807.0380"},
		{"FormatGPS discards sign", value.FormatGPS(-48.1173), "4807.0380"},
		{"FormatDMS", value.FormatDMS(48.1173), "48° 7' 2.28\""},
		{"FormatDMS discards sign", value.FormatDMS(-48.1173), "48° 7' 2.28\""},
		{"FormatPosition", value.FormatPosition(48.1173, 11.5167), "4807.0380N 1131.0020E"},
		{"FormatLatitude north", value.FormatLatitude(48.1173), "4807.0380N"},
		{"FormatLatitude south", value.FormatLatitude(-48.1173), "4807.0380S"},
		{"FormatLongitude east", value.FormatLongitude(11.5167), "1131.0020E"},
		{"FormatLongitude west", value.FormatLongitude(-11.5167), "1131.0020W"},
		{"FormatSpeedKnots", value.FormatSpeedKnots(12.3), "12.30 kn"},
		{"FormatBearing", value.FormatBearing(45.6), "45.6°"},
		{"FormatMetres", value.FormatMetres(9.5), "9.50 m"},
		{"FormatNauticalMiles", value.FormatNauticalMiles(1.5), "1.50 NM"},
		{
			"FormatCoordinate",
			value.FormatCoordinate(value.Coordinate{
				Degrees: 48, Minutes: 7.038, Hemisphere: value.North, Valid: true,
			}),
			"4807.0380N",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestLatLonDir covers the hemisphere letters directly, including zero, which
// is positive: latitude zero sits on the equator rather than at either pole.
func TestLatLonDir(t *testing.T) {
	if got := value.LatDir(0); got != "N" {
		t.Errorf("LatDir(0) = %q, want N", got)
	}
	if got := value.LonDir(0); got != "E" {
		t.Errorf("LonDir(0) = %q, want E", got)
	}
	if got := value.LatDir(-1); got != "S" {
		t.Errorf("LatDir(-1) = %q, want S", got)
	}
	if got := value.LonDir(-1); got != "W" {
		t.Errorf("LonDir(-1) = %q, want W", got)
	}
}
