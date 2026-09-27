package value

import (
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"math"
	"strconv"
	"strings"
	"time"
)

// Hemisphere is the N/S/E/W suffix that qualifies a coordinate field.
type Hemisphere byte

const (
	// HemisphereUnknown means the receiver sent no hemisphere field, or
	// sent something that is not one of N/S/E/W.
	HemisphereUnknown Hemisphere = 0
	North             Hemisphere = 'N'
	South             Hemisphere = 'S'
	East              Hemisphere = 'E'
	West              Hemisphere = 'W'
)

func (h Hemisphere) String() string {
	if h == HemisphereUnknown {
		return "?"
	}
	return string(rune(h))
}

// Positive reports whether the hemisphere adds to the magnitude (N or E).
func (h Hemisphere) Positive() bool { return h == North || h == East }

// Axis identifies which component of a coordinate pair a value belongs to.
// Decoders use it to range-check without duplicating latitude/longitude
// logic.
type Axis uint8

const (
	AxisLatitude Axis = iota
	AxisLongitude
)

func (a Axis) String() string {
	if a == AxisLongitude {
		return "longitude"
	}
	return "latitude"
}

// maxDegrees is the inclusive upper bound for each axis in whole degrees.
func (a Axis) maxDegrees() float64 {
	if a == AxisLongitude {
		return 180
	}
	return 90
}

// Coordinate is a latitude or longitude exactly as it appears on the wire:
// whole degrees, arc-minutes, and a hemisphere letter.
//
// NMEA encodes position as ddmm.mmmm (lat) or dddmm.mmmm (lon), so the
// decimal point is not where you would expect. Use Decimal to convert.
type Coordinate struct {
	// Value is the raw ddmm.mmmm / dddmm.mmmm field, kept for round-trip
	// formatting and diagnostics.
	Value string
	// Degrees is the whole-degree part.
	Degrees int
	// Minutes is the arc-minute part, always in [0, 60).
	Minutes float64
	// Hemisphere is N/S for latitude and E/W for longitude.
	Hemisphere Hemisphere
	// Valid is false when the receiver blanked the field.
	Valid bool
}

// ParseCoordinate converts an NMEA coordinate pair such as ("4807.038", "N")
// into a Coordinate. axis selects the range check. A blank value field
// yields an invalid Coordinate and a nil error, matching the NMEA
// convention that an empty field means "not available".
func ParseCoordinate(value, hemisphere string, axis Axis) (Coordinate, error) {
	c := Coordinate{Value: value, Hemisphere: ParseHemisphere(hemisphere)}
	if IsBlank(value) {
		return c, nil
	}

	// Minutes are always the final two digits before the decimal point, so
	// the split point is found from the right rather than assumed. That
	// handles the 2-digit and 3-degree-digit forms without knowing which
	// axis this is.
	dot := strings.LastIndexByte(value, '.')
	minutesStart := len(value) - 2
	if dot >= 0 {
		minutesStart = dot - 2
	}
	if minutesStart < 0 {
		minutesStart = 0
	}

	degPart, minPart := value[:minutesStart], value[minutesStart:]
	if degPart == "" {
		degPart = "0"
	}

	deg, err := strconv.Atoi(degPart)
	if err != nil {
		return c, fmt.Errorf("%w: degrees %q: %v", fault.ErrFieldValue, degPart, err)
	}
	min, err := strconv.ParseFloat(minPart, 64)
	if err != nil {
		return c, fmt.Errorf("%w: minutes %q: %v", fault.ErrFieldValue, minPart, err)
	}

	// The pole is a legal position, so the degree bound is inclusive. It is
	// the one place the whole-degree and minute parts interact: 90 degrees
	// 30 minutes does not exist.
	max := axis.maxDegrees()
	if deg < 0 || float64(deg) > max {
		return c, fmt.Errorf("%w: %s degrees %d must be 0..%d",
			fault.ErrFieldRange, axis, deg, int(max))
	}
	if min < 0 || min >= 60 {
		return c, fmt.Errorf("%w: %s minutes %.4f must be 0..60", fault.ErrFieldRange, axis, min)
	}
	if float64(deg) == max && min != 0 {
		return c, fmt.Errorf("%w: %s cannot be %d degrees %.2f minutes", fault.ErrFieldRange, axis, deg, min)
	}

	c.Degrees = deg
	c.Minutes = min
	c.Valid = true
	return c, nil
}

// ParseHemisphere maps a one-character field to a Hemisphere. Anything
// unrecognised, including an empty field, becomes HemisphereUnknown.
func ParseHemisphere(s string) Hemisphere {
	if s == "" {
		return HemisphereUnknown
	}
	switch Hemisphere(s[0]) {
	case North, South, East, West:
		return Hemisphere(s[0])
	default:
		return HemisphereUnknown
	}
}

// Decimal returns signed decimal degrees, negative for S and W.
// Latitude comes back in [-90, 90], longitude in [-180, 180].
func (c Coordinate) Decimal() (float64, error) {
	if !c.Valid {
		return 0, fault.ErrNotValid
	}
	if c.Hemisphere == HemisphereUnknown {
		return 0, fmt.Errorf("%w: coordinate has no hemisphere", fault.ErrFieldValue)
	}
	d := float64(c.Degrees) + c.Minutes/60
	if !c.Hemisphere.Positive() {
		d = -d
	}
	return d, nil
}

// MustDecimal is Decimal for callers that treat an invalid coordinate as
// impossible because they already checked Valid. It returns 0 on failure.
func (c Coordinate) MustDecimal() float64 {
	d, err := c.Decimal()
	if err != nil {
		return 0
	}
	return d
}

// ValueString renders the degrees and minutes in NMEA form without the
// hemisphere, e.g. "4807.038".
//
// When the coordinate came off the wire the original text is returned
// unchanged, including its number of decimal places. That is what makes a
// waypoint round trip exactly: re-encoding a value that arrived as
// "01131.000" produces "01131.000" and not "01131.0000", so a route read
// out of a receiver and written back is byte-identical. A coordinate built
// by hand has no original text and is formatted with four decimal places,
// which is the precision the standard uses, about 0.185 m.
func (c Coordinate) ValueString() string {
	if !c.Valid {
		return ""
	}
	if c.Value != "" {
		return c.Value
	}
	abs := math.Abs(c.Minutes)
	deg := c.Degrees
	if c.Minutes < 0 {
		deg++
	}
	return fmt.Sprintf("%02d%07.4f", deg, abs)
}

// String renders the coordinate in NMEA form with the hemisphere appended,
// e.g. "4807.0380N". Sentences that use a single field for the pair, such as
// the proprietary UBX format, need this form.
func (c Coordinate) String() string {
	if !c.Valid {
		return ""
	}
	if c.Hemisphere == HemisphereUnknown {
		return c.ValueString()
	}
	return c.ValueString() + string(rune(c.Hemisphere))
}

// TOD is a time of day from an hhmmss.sss field.
type TOD struct {
	Hour      int
	Minute    int
	Second    int
	Fraction  float64
	Valid     bool
	Available bool // true when the receiver actually sent the field
}

// ParseTOD parses hhmmss or hhmmss.sss.
func ParseTOD(s string) (TOD, error) {
	t := TOD{Available: !IsBlank(s)}
	if IsBlank(s) {
		return t, nil
	}

	body, frac := s, ""
	if dot := strings.LastIndexByte(s, '.'); dot >= 0 {
		body, frac = s[:dot], s[dot+1:]
	}
	if len(body) != 6 {
		return t, fmt.Errorf("%w: time %q must be 6 digits", fault.ErrFieldValue, s)
	}
	if frac != "" {
		f, err := strconv.ParseFloat("0."+frac, 64)
		if err != nil {
			return t, fmt.Errorf("%w: time fraction %q: %v", fault.ErrFieldValue, frac, err)
		}
		t.Fraction = f
	}

	h, err := strconv.Atoi(body[0:2])
	if err != nil {
		return t, fmt.Errorf("%w: hour %q: %v", fault.ErrFieldValue, body[0:2], err)
	}
	m, err := strconv.Atoi(body[2:4])
	if err != nil {
		return t, fmt.Errorf("%w: minute %q: %v", fault.ErrFieldValue, body[2:4], err)
	}
	sec, err := strconv.Atoi(body[4:6])
	if err != nil {
		return t, fmt.Errorf("%w: second %q: %v", fault.ErrFieldValue, body[4:6], err)
	}

	// Receivers occasionally round a leap second up to 60. Accept it rather
	// than dropping an otherwise good fix on the second it lands.
	if h > 23 || m > 59 || sec > 60 {
		return t, fmt.Errorf("%w: time %02d:%02d:%02d out of range", fault.ErrFieldRange, h, m, sec)
	}

	t.Hour, t.Minute, t.Second, t.Valid = h, m, sec, true
	return t, nil
}

// String renders hh:mm:ss.sss, or the empty string if unavailable.
func (t TOD) String() string {
	if !t.Available {
		return ""
	}
	if !t.Valid {
		return "--:--:--"
	}
	if t.Fraction > 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", t.Hour, t.Minute, t.Second, int(t.Fraction*1000))
	}
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
}

// Duration returns the elapsed time since UTC midnight.
func (t TOD) Duration() time.Duration {
	if !t.Valid {
		return 0
	}
	return time.Duration(t.Hour)*time.Hour +
		time.Duration(t.Minute)*time.Minute +
		time.Duration(t.Second)*time.Second +
		time.Duration(t.Fraction*float64(time.Second))
}

// Date is a calendar date from a ddmmyy field.
type Date struct {
	Day       int
	Month     int
	Year      int
	Valid     bool
	Available bool
}

// ParseDate parses ddmmyy. Two-digit years are windowed: 80..99 map to
// 19xx, 00..79 map to 20xx, which matches every GNSS receiver in service.
func ParseDate(s string) (Date, error) {
	d := Date{Available: !IsBlank(s)}
	if IsBlank(s) {
		return d, nil
	}
	if len(s) != 6 {
		return d, fmt.Errorf("%w: date %q must be 6 digits", fault.ErrFieldValue, s)
	}

	day, err := strconv.Atoi(s[0:2])
	if err != nil {
		return d, fmt.Errorf("%w: day %q: %v", fault.ErrFieldValue, s[0:2], err)
	}
	month, err := strconv.Atoi(s[2:4])
	if err != nil {
		return d, fmt.Errorf("%w: month %q: %v", fault.ErrFieldValue, s[2:4], err)
	}
	year, err := strconv.Atoi(s[4:6])
	if err != nil {
		return d, fmt.Errorf("%w: year %q: %v", fault.ErrFieldValue, s[4:6], err)
	}

	if year >= 80 {
		year += 1900
	} else {
		year += 2000
	}
	if month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) {
		return d, fmt.Errorf("%w: date %02d/%02d/%d does not exist", fault.ErrFieldValue, day, month, year)
	}

	d.Day, d.Month, d.Year, d.Valid = day, month, year, true
	return d, nil
}

// NewDate builds a Date from separate components and validates it against
// the calendar, including month lengths and leap years.
//
// ParseDate is for the two-digit ddmmyy form that RMC uses. ZDA carries a
// four-digit year and so cannot go through it, which is why this constructor
// exists rather than a second date parser.
func NewDate(year, month, day int) (Date, error) {
	if year < 1 {
		return Date{}, fmt.Errorf("%w: year %d is not a calendar year", fault.ErrFieldRange, year)
	}
	if month < 1 || month > 12 {
		return Date{}, fmt.Errorf("%w: month %d must be 1 to 12", fault.ErrFieldRange, month)
	}
	if day < 1 || day > daysInMonth(year, month) {
		return Date{}, fmt.Errorf("%w: %04d-%02d-%02d does not exist",
			fault.ErrFieldValue, year, month, day)
	}
	return Date{Day: day, Month: month, Year: year, Valid: true, Available: true}, nil
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (d Date) String() string {
	if !d.Available {
		return ""
	}
	if !d.Valid {
		return "--/--/----"
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// Combine merges a Date and a TOD into a single time.Time. It reports
// false when either half is unavailable.
func (d Date) Combine(t TOD) (time.Time, bool) {
	if !d.Valid || !t.Valid {
		return time.Time{}, false
	}
	return time.Date(d.Year, time.Month(d.Month), d.Day,
		t.Hour, t.Minute, t.Second, int(t.Fraction*1e9), time.UTC), true
}

// FixQuality is the GGA quality indicator: how good the fix actually is.
type FixQuality int

const (
	QualityNone       FixQuality = iota // 0, no fix
	QualityGPS                          // 1, autonomous GPS
	QualityDGPS                         // 2, differential
	QualityPPS                          // 3, PPS applied
	QualityRTKFixed                     // 4, RTK fixed integer
	QualityRTKFloat                     // 5, RTK float
	QualityEstimated                    // 6, dead reckoning
	QualityManual                       // 7, manual input
	QualitySimulation                   // 8, simulator
	QualityUnknown    FixQuality = -1
)

var qualityNames = map[FixQuality]string{
	QualityNone:       "none",
	QualityGPS:        "gps",
	QualityDGPS:       "dgps",
	QualityPPS:        "pps",
	QualityRTKFixed:   "rtk-fixed",
	QualityRTKFloat:   "rtk-float",
	QualityEstimated:  "estimated",
	QualityManual:     "manual",
	QualitySimulation: "simulation",
	QualityUnknown:    "unknown",
}

func (q FixQuality) String() string {
	if n, ok := qualityNames[q]; ok {
		return n
	}
	return "unknown"
}

// Fixed reports whether the quality corresponds to a usable position fix.
func (q FixQuality) Fixed() bool {
	switch q {
	case QualityGPS, QualityDGPS, QualityPPS, QualityRTKFixed, QualityRTKFloat:
		return true
	default:
		return false
	}
}

// ParseFixQuality converts a GGA quality field. A blank field yields
// QualityNone with no error, since that is what receivers send when they
// have nothing to report.
func ParseFixQuality(s string) (FixQuality, error) {
	if s == "" {
		return QualityNone, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return QualityUnknown, fmt.Errorf("%w: quality %q: %v", fault.ErrFieldValue, s, err)
	}
	if q := FixQuality(n); q >= QualityNone && q <= QualitySimulation {
		return q, nil
	}
	return QualityUnknown, fmt.Errorf("%w: quality %d is not a defined indicator", fault.ErrFieldValue, n)
}

// NavigationStatus is the RMC/APB/RMB status flag: is the fix usable, or
// is the receiver warning about it?
type NavigationStatus int

const (
	StatusInvalid NavigationStatus = iota
	StatusValid
	StatusWarning // A, but the receiver flags a caution (e.g. APB approach)
)

func (s NavigationStatus) String() string {
	switch s {
	case StatusValid:
		return "valid"
	case StatusWarning:
		return "warning"
	default:
		return "invalid"
	}
}

// ParseNavigationStatus converts an "A"/"V" status field. A blank field is
// treated as invalid: NMEA 0183 says an omitted status field means the
// sentence did not come from a valid fix.
func ParseNavigationStatus(s string) (NavigationStatus, error) {
	switch s {
	case "", "V":
		return StatusInvalid, nil
	case "A":
		return StatusValid, nil
	default:
		return StatusInvalid, fmt.Errorf("%w: status %q must be A or V", fault.ErrFieldValue, s)
	}
}

// StatusFlag is an ASCII flag field that the spec defines as a single
// character, e.g. AAM's "A,A" or VTG's "T"/"M" for true vs magnetic.
type StatusFlag byte

const (
	FlagUnknown    StatusFlag = 0
	FlagYes        StatusFlag = 'A' // active / true
	FlagNo         StatusFlag = 'V' // void / false
	FlagLeft       StatusFlag = 'L'
	FlagRight      StatusFlag = 'R'
	FlagTrue       StatusFlag = 'T'
	FlagFalse      StatusFlag = 'M' // magnetic
	FlagModeAuto   StatusFlag = 'A'
	FlagModeManual StatusFlag = 'M'
)

// ParseStatusFlag maps a single-character field to a StatusFlag.
func ParseStatusFlag(s string) StatusFlag {
	if s == "" {
		return FlagUnknown
	}
	return StatusFlag(s[0])
}

func (f StatusFlag) String() string {
	if f == FlagUnknown {
		return "?"
	}
	return string(rune(f))
}

// Valid reports whether the flag means "yes".
func (f StatusFlag) Valid() bool { return f == FlagYes }

// Side is the lateral direction flag used by RMB, APB, and AAM to say
// which side of a course line the vessel is on. It is a distinct concept
// from Hemisphere even though both are single letters, and conflating them
// would let a bad field parse as a valid bearing.
type Side int

const (
	SideUnknown Side = iota
	SideLeft
	SideRight
)

func (s Side) String() string {
	switch s {
	case SideLeft:
		return "left"
	case SideRight:
		return "right"
	default:
		return "unknown"
	}
}

// ParseSide converts an "L"/"R" field. A blank field yields SideUnknown with
// no error, since a receiver omits the flag when there is no cross-track
// error to sign.
func ParseSide(s string) (Side, error) {
	switch s {
	case "":
		return SideUnknown, nil
	case "L", "l":
		return SideLeft, nil
	case "R", "r":
		return SideRight, nil
	default:
		return SideUnknown, fmt.Errorf("%w: side %q must be L or R", fault.ErrFieldValue, s)
	}
}

// NavStatus is the NMEA 4.10 navigational status indicator, carried by RMC
// and GNS. It answers a different question from the A/V status field: not
// "is there a fix" but "is this fix safe to navigate on", which is the
// distinction an autopilot or an electronic chart display needs.
type NavStatus int

const (
	// NavStatusUnknown is an absent or unrecognised value.
	NavStatusUnknown NavStatus = iota
	// NavStatusSafe means the fix passes the receiver's integrity checks.
	NavStatusSafe
	// NavStatusCaution means the fix is usable but marginal, for instance
	// because a satellite failed a check.
	NavStatusCaution
	// NavStatusUnsafe means the fix must not be relied on.
	NavStatusUnsafe
	// NavStatusNotValid means the data is not valid for navigation.
	NavStatusNotValid
)

func (s NavStatus) String() string {
	switch s {
	case NavStatusSafe:
		return "safe"
	case NavStatusCaution:
		return "caution"
	case NavStatusUnsafe:
		return "unsafe"
	case NavStatusNotValid:
		return "not valid"
	default:
		return "unknown"
	}
}

// Navigable reports whether the status permits navigation. Only Safe is
// unconditional; Caution and Unknown deliberately report false, because
// treating "I do not know" as permission to steer is the dangerous default.
func (s NavStatus) Navigable() bool { return s == NavStatusSafe }

// ParseNavStatus converts the S/C/U/V field.
//
// A blank field yields NavStatusUnknown with no error, since the field only
// exists from NMEA 4.10 and older receivers have no value to give.
//
// The values are S, C, U, and V. They are sometimes wrongly documented as
// the FAA mode alphabet of A, D, E, M, N, S, V; that table belongs to a
// different field, and reusing it here would mark a Caution as "Autonomous"
// and an Unsafe as "Manual".
func ParseNavStatus(s string) (NavStatus, error) {
	switch strings.TrimSpace(s) {
	case "":
		return NavStatusUnknown, nil
	case "S", "s":
		return NavStatusSafe, nil
	case "C", "c":
		return NavStatusCaution, nil
	case "U", "u":
		return NavStatusUnsafe, nil
	case "V", "v":
		return NavStatusNotValid, nil
	default:
		return NavStatusUnknown, fmt.Errorf("%w: navigational status %q must be S, C, U, or V",
			fault.ErrFieldValue, s)
	}
}

// ParseFieldFloat parses a field that must be a number, and reports the
// failure in the terms the rest of the module uses: an empty field yields
// fault.ErrEmptyField, and an unparsable one wraps fault.ErrFieldValue with
// the offending text. Decoders in this module share it so that a bad number
// is reported the same way whichever sentence carried it.
func ParseFieldFloat(s string) (float64, error) {
	if s == "" {
		return 0, fault.ErrEmptyField
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %v", fault.ErrFieldValue, s, err)
	}
	return v, nil
}

// ParseFieldInt is ParseFieldFloat for an integer field, and reports the same
// two sentinels.
func ParseFieldInt(s string) (int, error) {
	if s == "" {
		return 0, fault.ErrEmptyField
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %v", fault.ErrFieldValue, s, err)
	}
	return v, nil
}
