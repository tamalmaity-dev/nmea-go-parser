package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VBW is the Dual Ground and Water Speed sentence. It reports longitudinal
// and transverse speed measured through the water and over the ground, which
// is what a vessel with fixed-thrust propellers or a tunnel thruster needs,
// because fore-aft and athwartships speeds differ.
//
//	$IIVBW,4.5,0.3,A,5.8,0.4,A
//
// Field layout:
//
//	0 longitudinal water speed   knots, negative astern
//	1 transverse water speed     knots, negative to port
//	2 water speed status         A
//	3 longitudinal ground speed  knots, negative astern
//	4 transverse ground speed    knots, negative to port
//	5 ground speed status        A
//	6 stern traverse water speed knots, NMEA 3.0 and later
//	7 status                     A
//	8 stern traverse ground      knots, NMEA 3.0 and later
//	9 status                     A
//
// The signs carry the direction and must be preserved: a negative
// longitudinal speed is movement astern, not a measurement error, and losing
// the sign would make a vessel appear to move forward while going backwards.
type VBW struct {
	Base
	WaterLongitudinal  float64
	HasWaterLong       bool
	WaterTransverse    float64
	HasWaterTrans      bool
	GroundLongitudinal float64
	HasGroundLong      bool
	GroundTransverse   float64
	HasGroundTrans     bool
	WaterStatus        nmea.StatusFlag
	GroundStatus       nmea.StatusFlag
	// SternWater and SternGround arrived in NMEA 3.0.
	SternWater     float64
	HasSternWater  bool
	SternGround    float64
	HasSternGround bool
}

// SpeedKnots returns the longitudinal ground speed, which is the figure most
// programs want, and false when it is absent.
func (v *VBW) SpeedKnots() (knots float64, ok bool) { return v.GroundLongitudinal, v.HasGroundLong }

// CourseOverGround returns the direction of the transverse ground speed as a
// bearing, and false when there is no transverse component or when it is too
// small to have a meaningful direction. A near-zero vector has an unstable
// angle, so reporting one would be worse than reporting none.
func (v *VBW) CourseOverGround() (degrees float64, ok bool) {
	if !v.HasGroundLong || !v.HasGroundTrans {
		return 0, false
	}
	long, trans := v.GroundLongitudinal, v.GroundTransverse
	if abs(long) < 1e-6 && abs(trans) < 1e-6 {
		return 0, false
	}
	// atan2 gives the angle from the bow, positive to starboard, which is
	// exactly the course-over-ground convention.
	deg := atan2(trans, long) * 180 / pi
	if deg < 0 {
		deg += 360
	}
	return deg, true
}

type vbw struct{}

func (vbw) Formatter() string { return "VBW" }

func (vbw) Decode(s nmea.Sentence) (any, error) {
	// A VBW with everything blank is how a receiver with no log or no
	// ground-speed input reports itself, so an empty field list is the only
	// failure.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := VBW{Base: newBase(s)}

	var err error
	if out.WaterLongitudinal, out.HasWaterLong, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.WaterTransverse, out.HasWaterTrans, err = optionalFloat(s, 1); err != nil {
		return out, err
	}
	out.WaterStatus = nmea.ParseStatusFlag(s.Field(2))
	if out.GroundLongitudinal, out.HasGroundLong, err = optionalFloat(s, 3); err != nil {
		return out, err
	}
	if out.GroundTransverse, out.HasGroundTrans, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	out.GroundStatus = nmea.ParseStatusFlag(s.Field(5))
	if out.SternWater, out.HasSternWater, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	if out.SternGround, out.HasSternGround, err = optionalFloat(s, 8); err != nil {
		return out, err
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: the stern traverse fields
// arrived in NMEA 3.0.
func (v VBW) NMEASupports() []nmea.Feature {
	if v.HasSternWater || v.HasSternGround {
		return nmea.FeatureOnly(nmea.FeatureGroundSpeed)
	}
	return nil
}

// ApplyFix folds the VBW into the fix state.
func (v VBW) ApplyFix(f *nmea.Fix) {
	if v.HasGroundLong {
		f.SetSpeedKnots(v.GroundLongitudinal)
	}
	if v.HasWaterLong {
		f.WaterSpeedKnots, f.HasWaterSpeed = v.WaterLongitudinal, true
		f.WaterSpeedKmh = v.WaterLongitudinal * 1.852
	}
}
