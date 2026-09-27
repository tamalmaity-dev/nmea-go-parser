package value

import (
	"fmt"
	"math"
)

// FormatGPS renders a coordinate in the NMEA ddmm.mmmm form, without a
// hemisphere, e.g. "4807.0380" for 48.1173 degrees.
//
// The sign is discarded, because the wire format carries the hemisphere in a
// separate field. Use FormatCoordinate to get the hemisphere as well.
func FormatGPS(degrees float64) string {
	abs := math.Abs(degrees)
	d := int(math.Floor(abs))
	m := (abs - float64(d)) * 60
	return fmt.Sprintf("%02d%07.4f", d, m)
}

// FormatDMS renders a coordinate in degrees, minutes, and seconds, e.g.
// "48° 7' 2.28" for 48.1173 degrees. The hemisphere is not included.
func FormatDMS(degrees float64) string {
	abs := math.Abs(degrees)
	d := int(math.Floor(abs))
	mFloat := 60 * (abs - float64(d))
	m := int(math.Floor(mFloat))
	s := 60 * (mFloat - float64(m))
	return fmt.Sprintf("%d° %d' %.2f\"", d, m, s)
}

// FormatCoordinate renders a Coordinate in NMEA form, the way it arrived,
// including its hemisphere. A coordinate that was never populated renders as
// the empty string.
func FormatCoordinate(c Coordinate) string { return c.String() }

// LatDir returns "N" for a non-negative latitude and "S" for a negative one,
// which is the hemisphere letter a NMEA latitude field needs. Zero is
// positive, since latitude zero is on the equator rather than at either pole.
func LatDir(degrees float64) string {
	if degrees < 0 {
		return "S"
	}
	return "N"
}

// LonDir returns "E" for a non-negative longitude and "W" for a negative
// one.
func LonDir(degrees float64) string {
	if degrees < 0 {
		return "W"
	}
	return "E"
}

// FormatPosition renders a latitude and longitude pair with their
// hemispheres, e.g. "4807.0380N 01131.0000E". It is the form to log or
// display, where a bare decimal degree is ambiguous to read aloud.
func FormatPosition(lat, lon float64) string {
	return FormatGPS(lat) + LatDir(lat) + " " + FormatGPS(lon) + LonDir(lon)
}

// FormatSpeedKnots renders a speed in knots, which is the unit NMEA reports.
func FormatSpeedKnots(knots float64) string { return fmt.Sprintf("%.2f kn", knots) }

// FormatBearing renders a bearing in degrees, which is the unit NMEA reports.
func FormatBearing(degrees float64) string { return fmt.Sprintf("%.1f°", degrees) }

// FormatMetres renders a distance in metres.
func FormatMetres(m float64) string { return fmt.Sprintf("%.2f m", m) }

// FormatNauticalMiles renders a distance in nautical miles.
func FormatNauticalMiles(nm float64) string { return fmt.Sprintf("%.2f NM", nm) }

// FormatLatitude renders a signed latitude with its hemisphere.
func FormatLatitude(degrees float64) string { return FormatGPS(degrees) + LatDir(degrees) }

// FormatLongitude renders a signed longitude with its hemisphere.
func FormatLongitude(degrees float64) string { return FormatGPS(degrees) + LonDir(degrees) }
