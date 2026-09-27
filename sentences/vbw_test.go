package sentences

import (
	"testing"
)

// TestPublishedVBW verifies the dual ground and water speed layout against the
// documented field list:
//
//	$--VBW,x.x,x.x,A,x.x,x.x,A
//
// gpsd publishes no worked example, so the values come from the field list and
// from a real receiver sentence:
//
//	$IIVBW,4.5,0.3,A,5.8,0.4,A*48
func TestPublishedVBW(t *testing.T) {
	v := published[VBW](t, "IIVBW,4.5,0.3,A,5.8,0.4,A")

	// Fields 0 and 1: speed through the water, longitudinal and transverse.
	// Transverse is positive to starboard, so it is kept signed.
	if !v.HasWaterLong || !near(v.WaterLongitudinal, 4.5, 1e-9) {
		t.Errorf("field 0 water longitudinal = %v (present %v), want 4.5", v.WaterLongitudinal, v.HasWaterLong)
	}
	if !v.HasWaterTrans || !near(v.WaterTransverse, 0.3, 1e-9) {
		t.Errorf("field 1 water transverse = %v (present %v), want 0.3", v.WaterTransverse, v.HasWaterTrans)
	}
	// Field 2: the water data is valid.
	if v.WaterStatus != 'A' {
		t.Errorf("field 2 water status = %q, want A", v.WaterStatus)
	}
	// Fields 3 and 4: the same measurements over ground, which differ from
	// the water figures by the current and the wind.
	if !v.HasGroundLong || !near(v.GroundLongitudinal, 5.8, 1e-9) {
		t.Errorf("field 3 ground longitudinal = %v (present %v), want 5.8", v.GroundLongitudinal, v.HasGroundLong)
	}
	if !v.HasGroundTrans || !near(v.GroundTransverse, 0.4, 1e-9) {
		t.Errorf("field 4 ground transverse = %v (present %v), want 0.4", v.GroundTransverse, v.HasGroundTrans)
	}
	if v.GroundStatus != 'A' {
		t.Errorf("field 5 ground status = %q, want A", v.GroundStatus)
	}

	// SpeedKnots is the figure the fix stores, which is the ground speed.
	knots, ok := v.SpeedKnots()
	if !ok || !near(knots, 5.8, 1e-9) {
		t.Errorf("SpeedKnots = %v, %v; want 5.8, true", knots, ok)
	}

	// The NMEA 3.0 stern traverse fields are absent from this sentence.
	if v.HasSternWater || v.HasSternGround {
		t.Error("a NMEA 2.3 VBW reported stern traverse data")
	}
}

// TestVBWSternTraverse checks the NMEA 3.0 extension, where fields 6 to 9
// carry the stern traverse speeds. Reading them as part of the forward
// measurement would corrupt every VBW from a modern receiver.
func TestVBWSternTraverse(t *testing.T) {
	v := published[VBW](t, "IIVBW,4.5,0.3,A,5.8,0.4,A,1.2,A,1.4,A")

	if !v.HasSternWater || !near(v.SternWater, 1.2, 1e-9) {
		t.Errorf("field 6 stern water = %v (present %v), want 1.2", v.SternWater, v.HasSternWater)
	}
	if !v.HasSternGround || !near(v.SternGround, 1.4, 1e-9) {
		t.Errorf("field 8 stern ground = %v (present %v), want 1.4", v.SternGround, v.HasSternGround)
	}
	// The forward fields must be unaffected by the extension.
	if !near(v.WaterLongitudinal, 4.5, 1e-9) || !near(v.GroundLongitudinal, 5.8, 1e-9) {
		t.Errorf("the forward speeds shifted: water %v, ground %v; want 4.5 and 5.8",
			v.WaterLongitudinal, v.GroundLongitudinal)
	}
}

// TestVBWNegativeIsAsternOrPort checks the sign convention, which is the part
// of this sentence a decoder most easily loses. A negative longitudinal speed
// means the vessel is making sternway and a negative transverse speed means
// the water flow is to port, so the sign carries meaning and must survive.
func TestVBWNegativeIsAsternOrPort(t *testing.T) {
	v := published[VBW](t, "IIVBW,-2.0,-1.5,A,3.0,0.5,A")

	if !near(v.WaterLongitudinal, -2.0, 1e-9) {
		t.Errorf("field 0 = %v, want -2.0: a negative longitudinal speed means astern", v.WaterLongitudinal)
	}
	if !near(v.WaterTransverse, -1.5, 1e-9) {
		t.Errorf("field 1 = %v, want -1.5: a negative transverse speed means port", v.WaterTransverse)
	}
}

// TestVBWAllBlank covers a receiver with neither a log nor a ground-speed
// input, which reports the sentence with every field empty.
func TestVBWAllBlank(t *testing.T) {
	v := published[VBW](t, "IIVBW,,,,,,")

	if v.HasWaterLong || v.HasWaterTrans || v.HasGroundLong || v.HasGroundTrans {
		t.Error("an all-blank VBW reported a speed")
	}
	if _, ok := v.SpeedKnots(); ok {
		t.Error("SpeedKnots reported a value for an all-blank VBW")
	}
}
