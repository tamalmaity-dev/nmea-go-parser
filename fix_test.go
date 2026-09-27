package nmea

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// frame builds a complete sentence from a body, so a test never carries a
// hand-computed checksum. A wrong checksum would make the parser reject the
// sentence and the test would fail for a reason unrelated to what it checks.
func frame(body string) string { return Frame(body) + "\r\n" }

// gsvBody builds a GSV body from satellite quadruples.
func gsvBody(talker string, total, msg, inView int, quads [][4]string, signal string) string {
	var b strings.Builder
	b.WriteString(talker)
	b.WriteString("GSV,")
	b.WriteString(strconv.Itoa(total))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(msg))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(inView))
	for _, q := range quads {
		b.WriteByte(',')
		b.WriteString(strings.Join(q[:], ","))
	}
	if signal != "" {
		b.WriteByte(',')
		b.WriteString(signal)
	}
	return b.String()
}

// gsaBody builds a GSA body. The 12 satellite slots are always written, so the
// trailing DOP and system-id fields land in the right places.
func gsaBody(talker string, fixType int, ids []int, pdop, hdop, vdop, systemID string) string {
	slots := make([]string, 12)
	for i, id := range ids {
		if i < len(slots) {
			slots[i] = strconv.Itoa(id)
		}
	}
	var b strings.Builder
	b.WriteString(talker)
	b.WriteString("GSA,A,")
	b.WriteString(strconv.Itoa(fixType))
	b.WriteByte(',')
	b.WriteString(strings.Join(slots, ","))
	b.WriteByte(',')
	b.WriteString(strings.Join([]string{pdop, hdop, vdop}, ","))
	if systemID != "" {
		b.WriteByte(',')
		b.WriteString(systemID)
	}
	return b.String()
}

// gsvOne is a single-sentence GSV cycle, the common case for a small sky.
func gsvOne(talker string, inView int, quads [][4]string) string {
	return frame(gsvBody(talker, 1, 1, inView, quads, ""))
}

// gpsCycleSatellites is the eleven satellites in the cycle below, as the
// quadruples a receiver would send.
var gpsCycleSatellites = [][4]string{
	{"03", "45", "150", "42"},
	{"07", "30", "210", "39"},
	{"11", "62", "080", "45"},
	{"14", "15", "310", "35"},
	{"23", "05", "140", "45"},
	{"25", "50", "220", "42"},
	{"31", "10", "060", "41"},
	{"28", "12", "090", "40"},
	{"22", "42", "067", "42"},
	{"24", "14", "311", "43"},
	{"27", "05", "244", "42"},
}

// gpsGSVCycle is a GSV cycle for GPS: eleven satellites in view, sent as three
// sentences of four, four, and three.
var gpsGSVCycle = func() string {
	var b strings.Builder
	for i, chunk := range [][][4]string{
		gpsCycleSatellites[0:4], gpsCycleSatellites[4:8], gpsCycleSatellites[8:11],
	} {
		b.WriteString(frame(gsvBody("GP", 3, i+1, 11, chunk, "")))
	}
	return b.String()
}()

// gsaGPS names satellites 7 and 14 as being in the solution.
var gsaGPS = frame(gsaBody("GP", 3, []int{7, 14}, "1.8", "1.0", "1.5", ""))

// TestSatelliteStatusJoinsPerConstellation covers the case that a PRN is unique
// only within a system. GPS 07, GLONASS 07, and Galileo 07 are three different
// satellites, and a receiver can track all three while using only the GPS one.
// Joining on the number alone would report the other two as in the solution,
// which overstates the solution and hides a receiver that has stopped using a
// system.
func TestSatelliteStatusJoinsPerConstellation(t *testing.T) {
	// The same PRN in all three systems, each tracked, only GPS in the solution.
	gpsView := frame(gsvBody("GP", 1, 1, 1, [][4]string{{"07", "45", "150", "42"}}, ""))
	galView := frame(gsvBody("GA", 1, 1, 1, [][4]string{{"07", "62", "080", "45"}}, ""))
	gloView := frame(gsvBody("GL", 1, 1, 1, [][4]string{{"07", "10", "028", "52"}}, ""))
	gsa := frame(gsaBody("GN", 3, []int{7}, "1.8", "1.0", "1.5", "1"))

	f := parseFix(t, gpsView+galView+gloView+gsa)

	if len(f.Satellites) != 3 {
		t.Fatalf("got %d satellites, want 3", len(f.Satellites))
	}

	got := map[Constellation]bool{}
	for _, s := range f.SatelliteStatus() {
		if s.ID != 7 {
			t.Errorf("satellite id = %d, want 7", s.ID)
		}
		got[s.Constellation] = s.Used
	}
	if !got[ConstellationGPS] {
		t.Error("GPS 07 is in the GSA solution but is not marked used")
	}
	if got[ConstellationGalileo] {
		t.Error("Galileo 07 is marked used because GPS 07 is: a PRN is not global")
	}
	if got[ConstellationGLONASS] {
		t.Error("GLONASS 07 is marked used because GPS 07 is: a PRN is not global")
	}

	// The per-system totals have to agree with the join, or the sky view and
	// the status list would contradict each other.
	perSystem := map[Constellation]int{}
	for _, cs := range f.SatellitesByConstellation() {
		perSystem[cs.Constellation] = cs.Used
	}
	if perSystem[ConstellationGPS] != 1 {
		t.Errorf("GPS used = %d, want 1", perSystem[ConstellationGPS])
	}
	if perSystem[ConstellationGalileo] != 0 || perSystem[ConstellationGLONASS] != 0 {
		t.Errorf("used per system = %v, want only GPS 1", perSystem)
	}
}

// TestSatelliteStatusUnknownConstellation checks the fallback for a receiver
// that does not name its systems. A GSA and a GSV that both leave the system
// unstated can only be matched on the number, and must still match, or a
// pre-NMEA-4.10 receiver would report nothing in use at all.
func TestSatelliteStatusUnknownConstellation(t *testing.T) {
	view := frame(gsvBody("XX", 1, 1, 1, [][4]string{{"11", "30", "090", "40"}}, ""))
	gsa := frame(gsaBody("XX", 3, []int{11}, "1.8", "1.0", "1.5", ""))

	f := parseFix(t, view+gsa)
	status := f.SatelliteStatus()
	if len(status) != 1 {
		t.Fatalf("got %d statuses, want 1", len(status))
	}
	if !status[0].Used {
		t.Error("satellite not marked used when neither sentence names a system")
	}
}

func parseFix(t *testing.T, stream string) Fix {
	t.Helper()
	p := New()
	if err := p.Consume(context.Background(), strings.NewReader(stream)); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	return p.Fix()
}

// TestGSVCycleAccumulates checks that a full cycle is collected rather than
// only its last sentence. A receiver with eleven satellites in view sends three
// GSV sentences, and a sky view showing three of them looks complete while
// being wrong about two thirds of the sky.
func TestGSVCycleAccumulates(t *testing.T) {
	f := parseFix(t, gpsGSVCycle)

	if len(f.Satellites) != 11 {
		t.Fatalf("got %d satellites, want 11: a cycle is split across sentences", len(f.Satellites))
	}
	if f.SatellitesInView != 11 {
		t.Errorf("SatellitesInView = %d, want 11", f.SatellitesInView)
	}
	seen := make(map[int]bool, 11)
	for _, s := range f.Satellites {
		if seen[s.ID] {
			t.Errorf("satellite %d appears twice", s.ID)
		}
		seen[s.ID] = true
		if s.Constellation != ConstellationGPS || !s.ConstellationKnown {
			t.Errorf("satellite %d constellation = %v (known %v), want GPS",
				s.ID, s.Constellation, s.ConstellationKnown)
		}
	}
	for _, s := range gpsCycleSatellites {
		id, _ := strconv.Atoi(s[0])
		if !seen[id] {
			t.Errorf("satellite %d missing from the accumulated cycle", id)
		}
	}
}

// TestGSVNewCycleResets checks that a repeated cycle replaces the list rather
// than growing it, and that a satellite which has gone out of view stops being
// reported.
func TestGSVNewCycleResets(t *testing.T) {
	// The first cycle sees satellite 5; the second does not.
	stream := gsvOne("GP", 2, [][4]string{{"05", "45", "150", "42"}, {"06", "30", "210", "39"}}) +
		gsvOne("GP", 1, [][4]string{{"06", "30", "210", "39"}})

	f := parseFix(t, stream)
	if len(f.Satellites) != 1 || f.Satellites[0].ID != 6 {
		t.Fatalf("satellites = %+v, want just satellite 6 from the second cycle", f.Satellites)
	}
	if f.SatellitesInView != 1 {
		t.Errorf("SatellitesInView = %d, want 1 from the second cycle", f.SatellitesInView)
	}
}

// TestGSVCyclesAccumulatePerSystem covers the multi-GNSS case. Each
// constellation runs its own cycle, so a single flat list would be emptied
// every time the next system started reporting and a sky view would show only
// whichever system spoke last.
func TestGSVCyclesAccumulatePerSystem(t *testing.T) {
	stream := gpsGSVCycle +
		gsvOne("GL", 1, [][4]string{{"88", "07", "028", "29"}}) +
		gsvOne("GA", 2, [][4]string{{"02", "10", "150", "40"}, {"11", "60", "330", "45"}}) +
		gsvOne("GB", 3, [][4]string{
			{"23", "45", "120", "44"}, {"24", "30", "240", "42"}, {"25", "60", "100", "40"}})

	f := parseFix(t, stream)
	if len(f.Satellites) != 17 {
		t.Fatalf("got %d satellites, want 17 across four systems", len(f.Satellites))
	}

	bySystem := f.SatellitesByConstellation()
	want := map[Constellation]int{
		ConstellationGPS:     11,
		ConstellationGLONASS: 1,
		ConstellationGalileo: 2,
		ConstellationBeiDou:  3,
	}
	if len(bySystem) != len(want) {
		t.Fatalf("got %d systems, want %d: %+v", len(bySystem), len(want), bySystem)
	}
	for _, cs := range bySystem {
		if cs.Tracked != want[cs.Constellation] {
			t.Errorf("%v tracked = %d, want %d", cs.Constellation, cs.Tracked, want[cs.Constellation])
		}
		if !cs.ConstellationKnown {
			t.Errorf("%v reported ConstellationKnown = false", cs.Constellation)
		}
	}

	// The in-view total is the sum of the per-system cycle totals, because each
	// system reports only its own.
	if f.SatellitesInView != 17 {
		t.Errorf("SatellitesInView = %d, want 17 summed across systems", f.SatellitesInView)
	}
	if got := f.SatellitesFor(ConstellationGalileo); len(got) != 2 {
		t.Errorf("SatellitesFor(Galileo) returned %d satellites, want 2", len(got))
	}
}

// TestMixedGSVIsNotFused checks the GN case. A GNGSV is a receiver reporting a
// combined set without saying which system each satellite belongs to, which is
// a fact about the data and not a gap in it.
func TestMixedGSVIsNotFused(t *testing.T) {
	f := parseFix(t, gsvOne("GN", 2, [][4]string{{"03", "45", "150", "42"}, {"07", "30", "210", "39"}}))

	sats := f.SatellitesByConstellation()
	if len(sats) != 1 {
		t.Fatalf("got %d systems, want 1", len(sats))
	}
	if sats[0].Constellation != ConstellationMixed {
		t.Errorf("constellation = %v, want Mixed for a GNGSV", sats[0].Constellation)
	}
	if !sats[0].ConstellationKnown {
		t.Error("ConstellationKnown = false for a GNGSV: a fused set is a known fact")
	}
	// A band cannot be named without a system, because digit 1 means a
	// different band for each.
	if _, ok := f.Satellites[0].SignalName(); ok {
		t.Error("SignalName named a band for a Mixed satellite, want none")
	}
}

// TestSatelliteStatusJoinsGSA checks the tracked/used join. GSV says which
// satellites are visible and GSA which are in the solution; neither says both,
// and they arrive in either order.
func TestSatelliteStatusJoinsGSA(t *testing.T) {
	f := parseFix(t, gsaGPS+gpsGSVCycle)

	status := f.SatelliteStatus()
	if len(status) != 11 {
		t.Fatalf("got %d statuses, want 11", len(status))
	}
	used := map[int]bool{}
	for _, s := range status {
		if s.Used {
			used[s.ID] = true
		}
	}
	if !used[7] || !used[14] {
		t.Errorf("used satellites = %v, want 7 and 14 marked used", used)
	}
	if used[3] || used[11] {
		t.Errorf("used satellites = %v, want 3 and 11 NOT marked used", used)
	}
	if f.SatellitesUsed != 2 {
		t.Errorf("SatellitesUsed = %d, want 2", f.SatellitesUsed)
	}
	ids, ok := f.UsedSatelliteIDs()
	if !ok || len(ids) != 2 {
		t.Errorf("UsedSatelliteIDs = %v, %v; want two ids, true", ids, ok)
	}
}

// TestGSAArrivingAfterGSV checks the join is not order-dependent, which is why
// it is computed rather than stored as a flag on the GSV value.
func TestGSAArrivingAfterGSV(t *testing.T) {
	after := parseFix(t, gpsGSVCycle+gsaGPS)
	before := parseFix(t, gsaGPS+gpsGSVCycle)

	if after.SatellitesUsed != before.SatellitesUsed {
		t.Errorf("SatellitesUsed depends on sentence order: %d after, %d before",
			after.SatellitesUsed, before.SatellitesUsed)
	}
	if len(after.SatelliteStatus()) != len(before.SatelliteStatus()) {
		t.Error("SatelliteStatus length depends on sentence order")
	}
}

// TestGSAPerSystemMerge covers the multi-GNSS case for the used list. A
// receiver sends one GSA per system and they all carry the GN talker, so
// keying on the sentence address would leave only the last system's satellites
// marked. The system id in the trailing field is what tells them apart.
//
// The GSV sentences use the per-system talkers, because in multi-GNSS operation
// NMEA requires that and it is what a receiver actually does.
func TestGSAPerSystemMerge(t *testing.T) {
	stream := gsvOne("GP", 1, [][4]string{{"03", "45", "150", "42"}}) +
		gsvOne("GA", 1, [][4]string{{"11", "60", "330", "45"}}) +
		gsvOne("GL", 1, [][4]string{{"88", "07", "028", "29"}}) +
		frame(gsaBody("GN", 3, []int{3}, "1.8", "1.0", "1.5", "1")) +
		frame(gsaBody("GN", 3, []int{11}, "1.5", "0.6", "1.2", "3")) +
		frame(gsaBody("GN", 3, []int{88}, "1.5", "0.6", "1.2", "2"))

	f := parseFix(t, stream)
	if f.SatellitesUsed != 3 {
		t.Errorf("SatellitesUsed = %d, want 3 summed across the three systems", f.SatellitesUsed)
	}
	if len(f.SatelliteUses) != 3 {
		t.Errorf("got %d per-system used lists, want 3: %+v", len(f.SatelliteUses), f.SatelliteUses)
	}
	for _, u := range f.SatelliteUses {
		if len(u.IDs) != 1 {
			t.Errorf("system %v has %d ids, want 1", u.System, len(u.IDs))
		}
	}

	bySystem := f.SatellitesByConstellation()
	if len(bySystem) != 3 {
		t.Fatalf("got %d systems, want 3: %+v", len(bySystem), bySystem)
	}
	for _, cs := range bySystem {
		if cs.Tracked != 1 || cs.Used != 1 {
			t.Errorf("%v tracked %d used %d, want 1 and 1", cs.Constellation, cs.Tracked, cs.Used)
		}
	}
}

// TestGSASameSystemReplaces guards the other direction: the next fix's GSA for
// a system must replace that system's list rather than adding to it.
func TestGSASameSystemReplaces(t *testing.T) {
	stream := frame(gsaBody("GN", 3, []int{3, 7}, "1.8", "1.0", "1.5", "1")) +
		frame(gsaBody("GN", 3, []int{3}, "1.8", "1.0", "1.5", "1"))

	f := parseFix(t, stream)
	if f.SatellitesUsed != 1 {
		t.Errorf("SatellitesUsed = %d, want 1: the second GSA replaced the first", f.SatellitesUsed)
	}
}

// TestFixSatelliteSlicesAreIndependent checks that a Fix handed to a caller
// cannot change under them. The parser rebuilds these lists as new GSV and GSA
// sentences arrive, so a shared backing array would mutate a copy already
// taken.
func TestFixSatelliteSlicesAreIndependent(t *testing.T) {
	p := New()
	if err := p.Consume(context.Background(),
		strings.NewReader(gpsGSVCycle+gsaGPS)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	held := p.Fix()
	before := len(held.Satellites)
	beforeUsed := held.SatellitesUsed
	beforeID := held.Satellites[0].ID
	beforePerSystem := len(held.SatellitesFor(ConstellationGPS))
	beforeUsedIDs, _ := held.UsedSatelliteIDs()

	// A whole new cycle for GPS, a new one for Galileo, and a new GSA.
	second := gsvOne("GP", 1, [][4]string{{"99", "10", "010", "20"}}) +
		gsvOne("GA", 1, [][4]string{{"77", "20", "010", "25"}}) +
		frame(gsaBody("GP", 3, []int{99}, "1.8", "1.0", "1.5", ""))
	if err := p.Consume(context.Background(), strings.NewReader(second)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if len(held.Satellites) != before {
		t.Errorf("held Satellites length changed from %d to %d", before, len(held.Satellites))
	}
	if held.SatellitesUsed != beforeUsed {
		t.Errorf("held SatellitesUsed changed from %d to %d", beforeUsed, held.SatellitesUsed)
	}
	if held.Satellites[0].ID != beforeID {
		t.Errorf("held Satellites[0].ID changed from %d to %d", beforeID, held.Satellites[0].ID)
	}
	if got := len(held.SatellitesFor(ConstellationGPS)); got != beforePerSystem {
		t.Errorf("held per-system list changed from %d to %d satellites", beforePerSystem, got)
	}
	after, _ := held.UsedSatelliteIDs()
	if len(after) != len(beforeUsedIDs) {
		t.Errorf("held UsedSatelliteIDs changed from %v to %v", beforeUsedIDs, after)
	}
}

// TestSatelliteSignalNameNeedsSystem checks the refusal to name a band without
// knowing the system, which is the mistake that labels a Galileo satellite
// "L1 C/A".
func TestSatelliteSignalNameNeedsSystem(t *testing.T) {
	// Galileo, signal digit 7, which is E1C.
	sat := Satellite{
		ID: 11, Constellation: ConstellationGalileo, ConstellationKnown: true,
		Signal: SignalDigit(7), HasSignal: true,
	}
	if name, ok := sat.SignalName(); !ok || name != "E1a" {
		t.Errorf("SignalName = %q, %v; want E1a, true", name, ok)
	}

	// The same digit with no system cannot be named.
	unknown := Satellite{ID: 11, Signal: SignalDigit(7), HasSignal: true}
	if _, ok := unknown.SignalName(); ok {
		t.Error("SignalName named a band for an unattributed satellite, want none")
	}

	// No signal field at all.
	none := Satellite{ID: 11, Constellation: ConstellationGPS, ConstellationKnown: true}
	if _, ok := none.SignalName(); ok {
		t.Error("SignalName named a band with no signal field, want none")
	}

	// "All signals" is a real answer but not a band name.
	all := Satellite{
		ID: 11, Constellation: ConstellationGPS, ConstellationKnown: true,
		Signal: SignalDigit(0), HasSignal: true,
	}
	if _, ok := all.SignalName(); ok {
		t.Error("SignalName named a band for signal digit 0, want none")
	}
}
