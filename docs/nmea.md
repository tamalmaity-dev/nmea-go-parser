# NMEA 0183 sentence reference

Field layouts for every sentence this library decodes, taken from the NMEA
0183 documentation and cross-checked against a second independent
implementation. Field numbering throughout is **0-based, where field 0 is the
first field after the sentence formatter**, so for
`$GPGGA,123519,4807.038,N,...` field 0 is `123519`, field 1 is `4807.038`,
and so on. This matches the indices in the Go decoder comments exactly.

The Go type for each sentence is named alongside. Decoders return **values**,
not pointers, so a caller writes `s.(sentences.RMB)`.

## Contents

- [Conventions](#conventions)
- [Framing and tag blocks](#framing-and-tag-blocks)
- [Position and fix](#position-and-fix)
- [Satellites, integrity, and time](#satellites-integrity-and-time)
- [Heading](#heading)
- [Speed, distance, and rate](#speed-distance-and-rate)
- [Navigation and cross-track](#navigation-and-cross-track)
- [Trawl and tracking](#trawl-and-tracking)
- [Environment and depth](#environment-and-depth)
- [Vessel](#vessel)
- [AIS and alerting](#ais-and-alerting)
- [Text and identification](#text-and-identification)
- [Deliberate omissions](#deliberate-omissions)
- [Sources and known source errors](#sources-and-known-source-errors)

## Conventions

**Coordinates** are `ddmm.mmmm` for latitude and `dddmm.mmmm` for longitude,
each followed by a hemisphere letter. The Go type splits them into
`Latitude, Longitude float64` (signed decimal degrees) and
`LatitudeRaw, LongitudeRaw nmea.Coordinate` (the exact wire form, for
round-tripping).

**Blank fields** mean "not available" and are never a parse error. They are
reported through a `Has*` flag, because a blank is not the same as zero:
latitude `0.0` is a real position. A field containing only spaces is treated
as blank, because some receivers and documents spell a blank that way.

**Reference letters** — `T` true north, `M` magnetic, `N` knots,
`K` kilometres, `M` metres, `C` Celsius, `L`/`R` port/starboard. These are
validated rather than ignored, because reading a magnetic value as a true one
is a silent error of ten or more degrees.

**Version-dependent trailing fields** are marked. All of them are optional in
practice, even on receivers that support the version that introduced them.

**Talker IDs** are separate from the sentence type. `GPGGA`, `GNGGA`, and
`GAGGA` are all GGA from GPS, a multi-constellation solution, and Galileo
respectively.

| Prefix | System |
|---|---|
| `GP` | GPS (legacy, by far the most common) |
| `GL` | GLONASS |
| `GA` | Galileo |
| `GB`, `BD` | BeiDou — both are seen in the field |
| `GQ`, `QZ` | QZSS — both are seen in the field |
| `GI` | NavIC / IRNSS |
| `GN` | Combined multi-constellation solution |

## Framing and tag blocks

A sentence is a framing character, a body, `*`, and a two-digit checksum:

```
$<body>*<hh>     ordinary
!<body>*<hh>     encapsulated: AIS, and tag-blocked sentences
```

**Both prefixes are accepted.** `nmea.Split` finds either one and
`nmea.ParseSentence` decodes both, because AIS is framed with `!` and a decoder
that only handled `$` would silently drop every AIS message on the wire.
Generating one is a separate problem, since the prefix is the caller's
decision and `Frame` cannot know it:

```go
nmea.Frame(body)                                    // always '$'
nmea.FrameWith(nmea.SentenceStartEnclosed, body)    // '!' when AIS
```

A NMEA 4.10 **tag block** may precede any sentence. It carries a timestamp and
other metadata about a group of sentences, and has **its own checksum**,
verified separately from the sentence's, since it is passed through gateways and
recomputed by anything that rewrites it:

```
\g:1-2-73874,n:157036,s:r003669945,c:1241544035*4A\!AIVDM,1,1,,A,...
```

```go
s, err := nmea.ParseSentence(line)
if s.TagBlock != nil {
    s.TagBlock.Source    // "r003669945"     station identifier
    s.TagBlock.Grouping  // "1-2-73874"      group and line number
    s.TagBlock.Time      // 1241544035       with TimeIsMilli saying the unit
    s.TagBlock.Extra     // keys this library does not name, kept as received
}
```

IEC 62320-1 uses the same block with a partly different key set, so an
unrecognised key is **kept in `Extra` rather than dropped**: a receiver
implementing that variant would otherwise lose its metadata with no error
anywhere. A tag block on its own is not a sentence and is not dispatched as one.

## Position and fix

### GGA — Global Positioning System Fix Data

`$GPGGA,hhmmss.ss,ddmm.mm,a,dddmm.mm,a,x,xx,x.x,x.x,M,x.x,M,x.x,xxxx*hh`

Type: `GGA`. Contributes position, altitude, HDOP, satellite count, quality.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time, `hhmmss.ss` | `UTC` |
| 1 | Latitude, `ddmm.mmmm` | `Latitude`, `LatitudeRaw` |
| 2 | `N` or `S` | `LatitudeRaw.Hemisphere` |
| 3 | Longitude, `dddmm.mmmm` | `Longitude`, `LongitudeRaw` |
| 4 | `E` or `W` | `LongitudeRaw.Hemisphere` |
| 5 | Quality indicator, 0–8 | `Quality` |
| 6 | Satellites used, 00–12 | `SatellitesUsed` |
| 7 | HDOP | `HDOP` |
| 8 | Altitude, metres above MSL | `Altitude` |
| 9 | Altitude unit, normally `M` | not surfaced |
| 10 | Geoid separation, metres | `GeoidSeparation` |
| 11 | Geoid unit, normally `M` | not surfaced |
| 12 | DGPS age, seconds | `DGPSAge` |
| 13 | DGPS station id, 0000–1023 | `DGPSStationID` |

Quality values: 0 no fix, 1 GPS, 2 DGPS, 3 PPS, 4 RTK fixed, 5 RTK float,
6 estimated, 7 manual, 8 simulation.

**GGA is the only sentence that reports fix quality**, so it also decides
whether the fix is valid. A receiver that loses its fix sends quality 0 with
the position still populated.

### GNS — GNSS Fix Data

`$GPGNS,hhmmss.ss,ddmm.mm,a,dddmm.mm,a,c--c,xx,x.x,x.x,x.x,x.x,s*hh`

Type: `GNS`. Contributes position, altitude, HDOP, satellites, nav status.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time | `UTC` |
| 1 | Latitude | `Latitude` |
| 2 | `N` or `S` | `LatitudeRaw.Hemisphere` |
| 3 | Longitude | `Longitude` |
| 4 | `E` or `W` | `LongitudeRaw.Hemisphere` |
| 5 | **Mode indicator, one character per constellation** | `ModeIndicator` |
| 6 | Satellites in use, 00–99 (not capped at 12 as in GGA) | `SatellitesUsed` |
| 7 | HDOP | `HDOP` |
| 8 | Altitude, metres above MSL | `Altitude` |
| 9 | Geoid separation, metres | `GeoidSeparation` |
| 10 | DGPS age, seconds | `DGPSAge` |
| 11 | DGPS station id | `DGPSStationID` |
| 12 | Navigational status, NMEA 4.10+ | `NavStatus` |

The mode indicator is **positional**: character 1 is GPS, 2 GLONASS,
3 Galileo, 4 BeiDou, 5 QZSS, 6 NavIC. Trailing systems are simply absent, so
`AN` means GPS autonomous and GLONASS not in use. Per-character values:
`A` autonomous, `D` differential, `E` estimated/dead-reckoning, `F` RTK float,
`M` manual, `N` no fix, `P` precise, `R` RTK fixed, `S` simulator.

Use `ModePerConstellation()` to get these as typed values.

### RMC — Recommended Minimum Specific GNSS Data

`$GNRMC,hhmmss.ss,A,ddmm.mm,a,dddmm.mm,a,x.x,x.x,xxxx,x.x,a[,m][,s]*hh`

Type: `RMC`. Contributes position, speed, course, date, time.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time | `UTC` |
| 1 | Status, `A` valid / `V` warning | `Status` |
| 2 | Latitude | `Latitude` |
| 3 | `N` or `S` | `LatitudeRaw.Hemisphere` |
| 4 | Longitude | `Longitude` |
| 5 | `E` or `W` | `LongitudeRaw.Hemisphere` |
| 6 | Speed over ground, knots | `SpeedKnots` |
| 7 | Course over ground, degrees true | `CourseDegrees` |
| 8 | Date, `ddmmyy` | `Date` |
| 9 | Magnetic variation, degrees | `MagneticVariation` |
| 10 | `E` or `W` for the variation | sign applied |
| 11 | FAA mode, NMEA 2.3+ | `Mode` |
| 12 | **Navigational status, NMEA 4.10+** | `NavStatus` |

**RMC is the only common sentence carrying the date**, so it is what makes a
full timestamp possible. GGA and GNS give time of day alone.

**Field 12 uses `S`/`C`/`U`/`V`.** It is routinely mis-implemented with the
FAA mode alphabet `A`/`D`/`E`/`M`/`N`/`S`/`V`, which is a *different field*:
doing so reports a Caution as "Autonomous" and an Unsafe as "Manual", both of
which sound reassuring. `nmea.ParseNavStatus` rejects the FAA letters.

### GLL — Geographic Position, Latitude/Longitude

`$GNGLL,ddmm.mm,a,dddmm.mm,a,hhmmss.ss,a[,m]*hh`

Type: `GLL`. The smallest position sentence; no altitude, no DOP, no velocity.

| Field | Meaning | Go field |
|---|---|---|
| 0 | Latitude | `Latitude` |
| 1 | `N` or `S` | `LatitudeRaw.Hemisphere` |
| 2 | Longitude | `Longitude` |
| 3 | `E` or `W` | `LongitudeRaw.Hemisphere` |
| 4 | UTC time | `UTC` |
| 5 | Status, `A` / `V` | `Status` |
| 6 | FAA mode, NMEA 2.3+ | `Mode` |

### VTG — Course Over Ground and Ground Speed

`$GPVTG,220.86,T,,M,2.550,N,4.724,K,A*hh`

Type: `VTG`. Contributes ground speed and course. Carries no position.

| Field | Meaning | Go field |
|---|---|---|
| 0 | Course over ground, degrees true | `CourseTrue` |
| 1 | `T` | reference, validated |
| 2 | Course over ground, degrees magnetic | `CourseMagnetic` |
| 3 | `M` | reference, validated |
| 4 | Speed over ground, knots | `SpeedKnots` |
| 5 | `N` | not surfaced |
| 6 | Speed over ground, km/h | `SpeedKmh` |
| 7 | `K` | not surfaced |
| 8 | FAA mode, NMEA 2.3+ | `Mode` |

Both speed figures are read **as sent** rather than one being derived from
the other, because a receiver computes them from more internal precision than
it prints: the example above sends 2.550 knots and 4.724 km/h, where the
conversion gives 4.7226.

An older NMEA 0183 form has five fields with no reference letters at all.
The two are told apart by field 1 being the fixed text `T`.

## Satellites, integrity, and time

### GSA — GNSS DOP and Active Satellites

`$GNGSA,A,3,80,71,73,79,69,,,,,,,,1.83,1.09,1.47[,sysid]*hh`

Type: `GSA`. Contributes satellite count and dilution of precision.

| Field | Meaning | Go field |
|---|---|---|
| 0 | Selection mode, `A` automatic / `M` manual | `Mode` |
| 1 | Fix type, 1 none / 2 2D / 3 3D | `FixType` |
| 2–13 | Twelve satellite PRNs, blank for unused slots | `SatelliteIDs` |
| 14 | PDOP | `PDOP` |
| 15 | HDOP | `HDOP` |
| 16 | VDOP | `VDOP` |
| 17 | System id, NMEA 4.10+, optional | `SystemID` |

Fields 14, 15, 16 are PDOP, HDOP, VDOP **in that order**. Reading them the
other way round gives plausible numbers with the wrong meaning.

Field 17 is optional even on 4.10 receivers — the same model has been
observed emitting GSA with and without it in one session.

### GSV — GNSS Satellites in View

`$GPGSV,3,1,11,03,03,111,00,04,15,270,00,06,01,010,00,13,06,292,00[,1]*hh`

Type: `GSV`. Contributes satellites in view.

| Field | Meaning | Go field |
|---|---|---|
| 0 | Total sentences in this cycle | `TotalMessages` |
| 1 | Sentence number, 1-based | `MessageNumber` |
| 2 | Satellites in view, whole cycle | `SatellitesInView` |
| 3+4n | Satellite PRN | `Satellites[n].ID` |
| 4+4n | Elevation, degrees | `Satellites[n].Elevation` |
| 5+4n | Azimuth, degrees true | `Satellites[n].Azimuth` |
| 6+4n | SNR, dB-Hz; 0 means tracked with no signal | `Satellites[n].SNR` |
| n | **Signal id, NMEA 4.10+** | `SignalID` |

**The signal id is ONE field for the whole sentence, not one per satellite.**
It sits immediately before the checksum. Reading it as a fifth value in each
group shifts every group after the first.

A group of four blank trailing fields is tolerated, because receivers pad the
last group of a cycle. A partial group carrying data is an error, since the
sentence is genuinely truncated.

A GSV cycle is split across several sentences, so accumulate by cycle rather
than acting on each sentence. `CycleComplete()` reports the last of a cycle.

### GST — GNSS Pseudorange Noise Error Statistics

`$GPGST,182141.000,15.5,15.3,7.2,21.8,0.9,0.5,0.8*hh`

Type: `GST`. Contributes the receiver's own expected error, in metres.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time of the associated fix | `UTC` |
| 1 | Total RMS standard deviation of range inputs | `RMS` |
| 2 | Std dev of the error ellipse semi-major axis | `SemiMajor` |
| 3 | Std dev of the semi-minor axis | `SemiMinor` |
| 4 | Orientation of the semi-major axis, degrees true | `Orientation` |
| 5 | Std dev of latitude error | `ErrorLatitude` |
| 6 | Std dev of longitude error | `ErrorLongitude` |
| 7 | Std dev of altitude error | `ErrorAltitude` |

Fields are frequently blank, which is normal: a 2D fix has no altitude
error. GST is a **measurement**, not an estimate, so where present it is
better than an accuracy figure derived from HDOP.

### GBS — GNSS Satellite Fault Detection (RAIM)

`$GPGBS,125027,23.43,M,13.91,M,34.01,M[,satid,prob,bias,stdev]*hh`

Type: `GBS`. Contributes expected error, and names a satellite the receiver
believes is faulty.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time of the associated fix | `UTC` |
| 1 | Expected 1-sigma error, latitude | `ExpectedErrorLat` |
| 2 | units, `M` | validated |
| 3 | Expected 1-sigma error, longitude | `ExpectedErrorLon` |
| 4 | units, `M` | validated |
| 5 | Expected 1-sigma error, altitude | `ExpectedErrorAlt` |
| 6 | units, `M` | validated |
| 7 | Failed satellite id, 1–138; 0 means none | `FailedSatelliteID` |
| 8 | Probability of missed detection | `ProbabilityOfMissedDetection` |
| 9 | Estimated bias on that satellite, metres | `EstimatedBias` |
| 10 | Std dev of that bias estimate | `BiasStdDev` |

**This is the one layout where the published field list and the published
example disagree.** The field list reads fields 1 to 7 as three bare figures
followed by a satellite id; the example, and every real receiver observed in
the field, interleaves an `M` after each figure. This library follows the
example, because that is what the hardware emits, so the values sit at the
odd indices.

A named fault does **not** invalidate the fix: the receiver may already have
excluded that satellite from the solution.

### GRS — GNSS Range Residuals

`$GPGRS,024603.00,1,-1.8,-2.7,0.3,,,,,,,[sysid,sigid]*hh`

Type: `GRS`. Residuals behind a receiver's error estimate.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time of the associated fix | `UTC` |
| 1 | Mode, 0 used in GGA / 1 computed after | `Mode` |
| 2–13 | Residual in metres, satellites 1 to 12 | `Residuals` |
| 14 | System id, NMEA 4.10+ | `SystemID` |
| 15 | Signal id, NMEA 4.10+ | `SignalID` |

The residuals are **positional**, keyed to satellite slot order, and the
receiver's GSA is what says which satellite is which. Mixing a GRS from one
epoch with a GSV from another attributes residuals to the wrong satellites,
which is why the timestamp is kept.

### ZDA — Time and Date

`$GPZDA,160012.71,11,03,2004,-1,00*hh`

Type: `ZDA`. The full date in four digits, independent of any fix.

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time, `hhmmss.ss` | `UTC` |
| 1 | Day, 01–31 | `Date.Day` |
| 2 | Month, 01–12 | `Date.Month` |
| 3 | Year, **four digits** | `Date.Year` |
| 4 | Local zone hours, −13 to +13, may be negative | `LocalZoneHours` |
| 5 | Local zone minutes, same sign as the hours | `LocalZoneMinutes` |

ZDA does not depend on a fix, so a receiver can report the date while still
searching for satellites. RMC carries the date too, but in `ddmmyy`.

## Heading

### HDT — Heading, True

`$GPHDT,274.07,T*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Heading, degrees | `Heading.Degrees` |
| 1 | `T` for true | `Heading.True` |

### THS — True Heading and Status

`$GPTHS,338.01,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Heading, degrees true | `Heading.Degrees` |
| 1 | Source: `A` autonomous, `E` estimated, `M` manual, `S` simulator, `V` invalid | `Mode` |

THS says where the heading came from, which matters on a vessel with more
than one compass: an estimated heading from a rate gyro is a different claim
from an autonomous one from a satellite.

### HDG — Heading, Deviation and Variation

`$HCHDG,101.1,,,7.1,W*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Compass heading, degrees magnetic | `Magnetic` |
| 1 | Deviation, degrees, optional | `Deviation` |
| 2 | `E` or `W` for the deviation | sign applied |
| 3 | Variation, degrees, optional | `Variation` |
| 4 | `E` or `W` for the variation | sign applied |

A magnetic heading is **not** a true heading. `TrueHeading()` reports the
converted value and returns false when the variation is missing, rather than
passing a magnetic figure off as true. The difference can exceed 20 degrees.

## Speed, distance, and rate

### HDM — Heading, Magnetic

`$HCHDM,123.4,M*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Heading, degrees magnetic | `HeadingDegrees` |
| 1 | `M` | fixed by the format |

Deprecated in favour of HDT, and decoded because a great many magnetic
compasses still emit it. It sets `Fix.Heading` with `HeadingTrue` left **false**,
because a magnetic heading differs from a true one by the local variation and
reporting it as true would be silently tens of degrees out.

### HSC — Heading Steering Command

`$IIHSC,123.4,T,124.5,M*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Commanded heading, degrees true | `HeadingTrue` |
| 1 | `T` | fixed by the format |
| 2 | Commanded heading, degrees magnetic | `HeadingMagnetic` |
| 3 | `M` | fixed by the format |

A command to the autopilot, not a measurement, so it deliberately does not
implement `FixContributor`: folding a heading that has not been achieved yet into
the reported heading would have a vessel's autopilot lying about its position.

A documented ambiguity: the gpsd reference notes that GLOBALSAT describes HSC
as carrying water temperature sensor data instead, and says it is unclear which
reading is correct. The heading interpretation is implemented, being the one the
standard's own sentence list uses.

### VHW — Water Speed and Heading

`$IIVHW,100.0,T,105.0,M,5.5,N,10.2,K*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Heading, degrees true | `HeadingTrue` |
| 1 | `T` | validated |
| 2 | Heading, degrees magnetic | `HeadingMagnetic` |
| 3 | `M` | validated |
| 4 | Speed through water, knots | `SpeedKnots` |
| 5 | `N` | not surfaced |
| 6 | Speed through water, km/h | `SpeedKmh` |
| 7 | `K` | not surfaced |

Water speed is measured by a transducer, not by satellites. The difference
between this and the ground speed in VTG or RMC is the current, which is
often the more useful of the pair on a working vessel.

### VBW — Dual Ground and Water Speed

`$IIVBW,4.5,0.3,A,5.8,0.4,A[,stern]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Longitudinal water speed, knots; negative astern | `WaterLongitudinal` |
| 1 | Transverse water speed, knots; negative to port | `WaterTransverse` |
| 2 | Water status `A` | `WaterStatus` |
| 3 | Longitudinal ground speed, knots | `GroundLongitudinal` |
| 4 | Transverse ground speed, knots | `GroundTransverse` |
| 5 | Ground status `A` | `GroundStatus` |
| 6 | Stern traverse water speed, NMEA 3.0+ | `SternWater` |
| 8 | Stern traverse ground speed, NMEA 3.0+ | `SternGround` |

The signs carry the direction and are preserved: a negative longitudinal
speed is movement astern, not a measurement error.

### VLW — Distance Travelled through Water

`$GPVLW,,N,,N,1234.5,N,2345.6,N*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Total through water, nautical miles | `TotalWater` |
| 1 | `N` | validated |
| 2 | Through water since reset, nm | `TripWater` |
| 3 | `N` | validated |
| 4 | Total over ground, nm, NMEA 3.0+ | `TotalGround` |
| 5 | `N` | validated |
| 6 | Over ground since reset, nm, NMEA 3.0+ | `TripGround` |
| 7 | `N` | validated |

The difference between the water and ground totals is distance run, which is
how current set is measured without a second input.

### ROT — Rate of Turn

`$HEROT,-2.5,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Degrees per minute; negative turns to port | `DegreesPerMinute` |
| 1 | Status `A` valid | `Status` |

The sign is the whole point: a receiver that drops it makes a port turn look
like a starboard one. A lone `-` means turning to port with no magnitude
measured, and is accepted.

## Navigation and cross-track

### RPM — Revolutions

`$IIRPM,S,1,1200.0,10.5,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | `S` shaft, `E` engine | `Source` |
| 1 | Engine or shaft number | `Number` |
| 2 | Speed, revolutions per minute | `Speed` |
| 3 | Pitch, percent of maximum, `-` meaning astern | `Pitch`, `Astern` |
| 4 | `A` valid, `V` invalid | `Status` |

The source field is validated, because a shaft and its engine turn at different
speeds: "1200 RPM" is not a meaningful figure until you know which one was meant,
and a decoder that ignored the field would report a figure off by the gear ratio
on every boat whose shaft RPM is not its engine RPM.

A negative pitch is a **direction**, not a small negative trim, so the sign is
reported separately as `Astern` and `Pitch` is the magnitude.

### WPL — Waypoint Location

`$GPWPL,4917.16,N,12310.64,W,003*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Latitude | `Latitude` |
| 1 | `N` or `S` | `LatitudeRaw.Hemisphere` |
| 2 | Longitude | `Longitude` |
| 3 | `E` or `W` | `LongitudeRaw.Hemisphere` |
| 4 | Waypoint name | `Name` |

WPL **defines** a waypoint rather than observing one, so it does not
implement `nmea.FixContributor`. Loading a route full of waypoints cannot
move the reported position.

`Encode` round-trips byte for byte, leading zeros included, because it uses
the raw ddmm.mmmm values rather than the decimal degrees.

### RTE — Routes

`$GPRTE,2,1,c,0,PBRCPK,PBRTO,PTBR,PPBR*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Total sentences in this cycle | `TotalMessages` |
| 1 | Sentence number | `MessageNumber` |
| 2 | Mode, `c` complete or `w` working | `Mode` |
| 3 | **Route id** | `RouteID` |
| 4+ | Waypoint names | `Waypoints` |

Field 3 is the route identifier, **not a waypoint**. Reading it as one puts a
bogus entry at the head of every route.

`c` and `w` are the only two modes the standard defines. The load, delete,
and list variants of the format reuse the field for a form code; those arrive
with `IsRequest()` set and the route name in `Target`, so a caller can tell a
route report from a command before acting on it.

### RMB — Recommended Minimum Navigation Information

`$GPRMB,A,0.66,L,003,004,4917.24,N,12309.57,W,001.3,052.5,000.5,V[,mode]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Status, `A` active / `V` invalid | `Status` |
| 1 | Cross-track error, nautical miles | `CrossTrack` |
| 2 | Steer direction, `L` or `R` | `Steer` |
| 3 | **Origin** waypoint id | `OriginID` |
| 4 | **Destination** waypoint id | `DestinationID` |
| 5 | **Destination** latitude | `Destination.Latitude` |
| 6 | `N` or `S` | — |
| 7 | **Destination** longitude | `Destination.Longitude` |
| 8 | `E` or `W` | — |
| 9 | Range to destination, nm | `RangeToDestination` |
| 10 | Bearing to destination, degrees true | `BearingToDestination` |
| 11 | Closing velocity, knots | `ClosingVelocity` |
| 12 | Arrival status, `A` entered | `Arrival` |
| 13 | FAA mode, NMEA 2.3+ | `Mode` |

> **Fields 5 to 8 are the destination waypoint, not the vessel.**
>
> Several widely-copied decoders read them as the vessel position, which
> silently moves the reported fix onto the waypoint. It looks plausible on a
> map and is completely wrong. This library does not implement
> `nmea.FixContributor` on RMB at all, so there is no way for it to
> overwrite the position that GGA, RMC, or GNS established.

Note the field order: RMB names the **origin first** and the destination
second, the reverse of BOD.

### APB — Autopilot Sentence B

`$GPAPB,A,A,0.10,R,N,V,V,011,M,DEST,011,M,011,M*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Status, `A` data valid | `Status` |
| 1 | Status, `V` cycle-lock warning | `BearingStatus` |
| 2 | Cross-track error magnitude | `CrossTrack` |
| 3 | Direction to steer, `L` or `R` | `Steer` |
| 4 | Cross-track unit, `N` or `K` | `CrossTrackUnit` |
| 5 | Arrival circle entered, `A` | `Arrival` |
| 6 | Perpendicular passed, `A` | `Perpendicular` |
| 7 | Bearing origin to destination | `BearingOriginToDestination` |
| 8 | `M` or `T` reference for field 7 | `BearingOriginTrue` |
| 9 | Destination waypoint id | `DestinationID` |
| 10 | Bearing, present position to destination | `BearingToDestination` |
| 11 | `M` or `T` reference for field 10 | `BearingToDestTrue` |
| 12 | Heading to steer | `HeadingToSteer` |
| 13 | `M` or `T` reference for field 12 | `HeadingToSteerTrue` |

The three bearings are the field that trips people up. Field 7 is the leg as
planned, field 10 is from where the vessel actually is, and field 12 is what
the helm should be turned to. They coincide only when the vessel is exactly
on the course line.

The standard form carries **no destination coordinates**, so `Destination` is
populated only for the extended form some receivers append past field 13.

### AAM — Waypoint Arrival Alarm

`$GPAAM,A,A,0.10,N,WPTNME*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Arrival circle entered, `A` | `ArrivalCircle` |
| 1 | Perpendicular passed, `A` | `PerpendicularCrossed` |
| 2 | Arrival circle radius | `CircleRadius` |
| 3 | Units, nautical miles | `CircleUnit` |
| 4 | Waypoint id | `WaypointID` |

**The standard form is five fields, with no left/right field.** Several
receivers emit a six-field variant with a perpendicular offset and side
inserted before the unit; that is detected by an `L` or `R` in field 3, and
decoded into `PerpendicularOffset` and `Side`.

### BOD — Bearing, Origin to Destination

`$GPBOD,097.0,T,103.2,M,POINTB,POINTA*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Bearing, degrees true | `BearingTrue` |
| 1 | `T` | validated |
| 2 | Bearing, degrees magnetic | `BearingMagnetic` |
| 3 | `M` | validated |
| 4 | **Destination** waypoint | `Destination` |
| 5 | **Origin** waypoint, optional | `Origin` |

The destination precedes the origin, the reverse of RMB. The origin is
optional: a receiver in goto mode has only a destination, and the standard's
own example omits it.

### BWC / BWR — Bearing and Distance to Waypoint

Great circle (BWC) and rhumb line (BWR). The layouts are identical.

`$GPBWC,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,0004.6,N,EGLM[,mode]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | UTC time | `UTC` |
| 1 | Waypoint latitude | `Latitude` |
| 2 | `N` or `S` | — |
| 3 | Waypoint longitude | `Longitude` |
| 4 | `E` or `W` | — |
| 5 | Bearing, degrees true | `BearingTrue` |
| 6 | `T` | validated |
| 7 | Bearing, degrees magnetic | `BearingMagnetic` |
| 8 | `M` | validated |
| 9 | Distance, nautical miles | `Distance` |
| 10 | `N` | validated |
| 11 | Waypoint id | `WaypointID` |
| 12 | FAA mode, NMEA 2.3+ | `Mode` |

`Model` records which sentence it was, so a program handling both can tell
how the distance was computed. The rhumb figure is the larger, because a
constant bearing is a longer path than a great circle.

### XTE — Cross-Track Error, Measured

`$GPXTE,A,A,0.67,L,N[,mode]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Status, `A` valid | `Status` |
| 1 | Status, `V` cycle-lock warning | `LockStatus` |
| 2 | Cross-track error magnitude | `CrossTrack` |
| 3 | Direction to steer, `L` or `R` | `Steer` |
| 4 | Units, `N` or `K` | `Unit` |
| 5 | FAA mode, NMEA 2.3+ | `Mode` |

Carries no position at all, which is what distinguishes it from RMB and APB.

### WCV — Waypoint Closure Velocity

`$GPWCV,2.3,N,DEST[,mode]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Velocity, knots; positive when closing | `Velocity` |
| 1 | `N` | validated |
| 2 | Waypoint id | `WaypointID` |
| 3 | FAA mode, NMEA 3.0+ | `Mode` |

The sign is the useful part: a negative velocity means the vessel is opening
the distance, usually because it has passed the waypoint without noticing.

## Environment and depth

## Trawl and tracking

### TTM — Tracked Target Message

`$RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,N,SEN,Y*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Target number, 0 to 99 | `Number` |
| 1, 2, 3 | Distance, bearing, and `T`/`R` | `Distance`, `Bearing`, `DistanceBearingTrue` |
| 4, 5, 6 | Speed, course, and `T`/`R` | `Speed`, `Course`, `SpeedCourseTrue` |
| 7 | Distance of closest approach | `DistanceToCPA` |
| 8 | Time to closest approach, `-` meaning diverging | `TimeToCPA`, `Diverging` |
| 9 | `K` kilometres or `N` nautical miles | `Units` |
| 10 | Target name | `Name` |
| 11 | Target status | `Status` |
| 12 | Reference | `Reference` |
| 13 | UTC of the data, NMEA 3.0+ | `UTC` |
| 14 | `A` auto, `M` manual, `R` reported | `Type` |

The three reference fields are validated, because distance and bearing are
relative to the vessel, speed and course are relative to the water, and the
closest-approach figures are relative to neither. A relative bearing read as a
true one puts a target tens of degrees out of place.

A `-` on the time to closest approach means the vessels are separating, which is
a real answer rather than a missing one. A target number outside 0 to 99 is
rejected, and TTM never touches the fix: a target is not the vessel.

### TLL — Target Latitude and Longitude

`$GPTLL,31,4951.42,N,01211.75,W,WHALE,123519,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Target number, 0 to 99 | `Number` |
| 1, 2 | Latitude and hemisphere | `Latitude`, `LatitudeRaw` |
| 3, 4 | Longitude and hemisphere | `Longitude`, `LongitudeRaw` |
| 5 | Target name | `Name` |
| 6 | UTC of the data | `UTC` |
| 7 | `L` lost, `Q` acquiring, `T` tracking | `Status` |
| 8 | `R` for a reference target | `Reference` |

The number is what ties this to TTM and TLB. The number of decimal places is
equipment dependent, so the raw form is kept: a receiver sending two places and
one sending four would otherwise produce positions a hundred metres apart for the
same target.

**TLL carries a position and it is the wrong one**: a tracked target's, not the
vessel's. Several widely-copied decoders read it as a vessel position, which
silently moves the reported fix onto whatever the radar happened to be tracking.
It deliberately does not implement `FixContributor`.

### TLB — Target Label

`$GPTLB,1,WHALE,2,TARGET2,3,TARGET3*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0, 2, 4… | Target number | `Labels[i].Number` |
| 1, 3, 5… | Label assigned to it | `Labels[i].Name` |

Repeated until the sentence ends. A dangling number with no label is rejected,
because a name attached to nothing is worse than a name missing, and it would
otherwise attach to the wrong target. `Label(n)` is the per-target lookup a
display does.

### DPT — Depth of Water

`$INDPT,2.3,0.0[,range]*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Depth below the transducer, metres | `Depth` |
| 1 | Offset, transducer to waterline, metres | `Offset` |
| 2 | Range scale in use, metres, NMEA 3.0+ | `RangeScale` |

**The offset sign decides how keel depth is computed**, and the conventions
differ between sources. This library uses the one that makes the result
unambiguous: a positive offset means the transducer is above the waterline, so
keel depth is offset minus depth; a negative offset means it is below, so
keel depth is offset plus depth.

Getting it backwards reports a depth wrong by twice the offset, which on a
shallow-draft vessel is the difference between safe and aground.
`DepthBelowKeel()` returns false for a negative result, because a negative
depth means the vessel is aground rather than in very shallow water.

### MTW — Mean Temperature of Water

`$INMTW,17.9,C*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Temperature, Celsius | `Celsius` |
| 1 | `C` | validated |

Despite the name, this is the temperature at the transducer, so it reflects
the water the hull is in rather than the air.

### DBT / DBS / DBK — Depth

`$SDDBT,7.8,f,2.4,M,1.3,F*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Depth, feet | `Feet` |
| 1 | `f` | fixed by the format |
| 2 | Depth, metres | `Metres` |
| 3 | `M` | fixed by the format |
| 4 | Depth, fathoms | `Fathoms` |
| 5 | `F` | fixed by the format |

The three sentences are byte for byte identical and differ only in what the
depth is measured from: **DBT** below the transducer, **DBS** below the
surface, **DBK** below the keel. DBK is marked obsolete in favour of DPT, which
also carries the transducer offset and so can reach a keel depth a receiver does
not state directly.

Every unit is optional. The documentation's own note is that real sensors often
report only one, so `$SDDBT,,f,22.5,M,,F` is a normal sentence rather than a
truncated one, and `MetresOrConverted()` returns the depth in metres from
whichever unit arrived. A dangling value with no unit letter is rejected, because
`7.8` alone is ambiguous between feet and metres and guessing wrong reports a
depth out by a factor of 3.3.

### XDR — Transducer Measurement

`$HCXDR,A,171,D,PITCH,A,-37,D,ROLL,G,367,,MAGX*hh`
`$SDXDR,C,23.15,C,WTHI*hh`
`$IIXDR,P,0.9,B,PTYPE*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Transducer type | `Type`, `TypeLetter` |
| 1 | Value | `Value`, `HasValue` |
| 2 | Unit | `Unit`, `UnitLetter` |
| 3 | Name of the transducer | `Name` |

Repeated until the sentence ends. XDR is the only variable-length sentence here,
which is what makes it so useful: one sentence carries every analogue input a
device has, and a program that only wants position can ignore the rest rather
than needing a decoder per measurement.

Transducer types: `A` angular, `B` absolute humidity, `C` temperature, `D`
depth, `F` frequency, `G` generic, `H` humidity, `I` current, `L` salinity, `N`
force, `P` pressure, `R` flow, `S` switch, `T` tachometer, `U` voltage, `V`
volume.

**An unknown type or unit is not an error.** The format is explicitly extensible
and manufacturers do invent letters, so rejecting one would fail the whole
sentence and take the readings this library does understand down with it. The raw
letter is kept in `TypeLetter` and `UnitLetter`.

The standard **reuses unit letters**, and which one applies depends on the type:
`B` is bars or binary, `K` is kelvin or kg/m³, `M` is metres or cubic metres, and
`P` is percent or pascals. `UnitMeaning()` resolves the pair. Getting it wrong is
a factor-of-100 error, so the letter alone is never decoded on its own.

### MDA — Meteorological Composite

`$IIMDA,29.9,I,1.013,B,17.0,C,,C,60.0,,45.0,C,120.0,T,125.0,M,10.0,N,5.1,M*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0, 1 | Barometric pressure, inches of mercury | `PressureInHg` |
| 2, 3 | Barometric pressure, bars | `PressureBar` |
| 4, 5 | Air temperature, Celsius | `AirTempC` |
| 6, 7 | Water temperature, Celsius | `WaterTempC` |
| 8 | Relative humidity, percent | `RelativeHumidity` |
| 9 | Absolute humidity, percent | `AbsoluteHumidity` |
| 10, 11 | Dew point, Celsius | `DewPointC` |
| 12, 13 | Wind direction, degrees true | `WindDirectionTrue` |
| 14, 15 | Wind direction, degrees magnetic | `WindDirectionMagnetic` |
| 16, 17 | Wind speed, knots | `WindSpeedKnots` |
| 18, 19 | Wind speed, m/s | `SpeedMps`, derived |

Obsolete as of 2009 in favour of the individual sentences and XDR, and decoded
because a great many weather instruments still send it, and because it is the
only sentence carrying dew point and absolute humidity at all.

The water temperature is documented as left blank by WeatherStation, one of the
instruments this sentence came from, so a blank there is an expected shape rather
than a fault. `PressureHectopascals()` converts from whichever pressure unit
arrived.

### MTA — Air Temperature

`$IIMTA,13.3,C*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Air temperature, Celsius | `Celsius` |
| 1 | `C` | validated |

Obsolete in favour of XDR. The unit is validated here, unlike the fixed unit
letters in the depth sentences, because a Fahrenheit reading here is a 32 degree
error. `Fix.AirTempC` is separate from `Fix.WaterTempC`.

### MWD — Wind Direction and Speed

`$WIMWD,302.4,T,289.6,M,10.5,N,5.4,M*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Wind direction, degrees true | `DirectionTrue` |
| 1 | `T` | validated |
| 2 | Wind direction, degrees magnetic | `DirectionMagnetic` |
| 3 | `M` | validated |
| 4 | Wind speed | `SpeedKnots` |
| 5 | `N` knots, `K` km/h, `M` m/s | `SpeedUnit` |
| 6 | Wind speed in a second unit | `SpeedKnotsAlt` |
| 7 | second unit | `SpeedUnitAlt` |

Direction is the direction the wind comes **from**, the convention every
weather report uses. A direction of 90 is an easterly, blowing towards the
west.

### MWV — Wind Speed and Angle

`$WIMWV,214.8,R,10.5,N,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Wind angle, degrees | `AngleDegrees` |
| 1 | `R` relative to the bow, `T` true | `Relative` |
| 2 | Wind speed | `SpeedKnots` |
| 3 | `N`, `K`, or `M` | `SpeedUnit` |
| 4 | Status, `A` valid | `Status` |

The reference field matters: a relative angle and a true angle are both
"0 to 359", and reading one as the other points the wind entirely the wrong
way. `TrueDirection` needs the vessel's heading for a relative angle and
returns false without it.

### VWR / VWT — Relative and True Wind

`$IIVWR,75,R,1.0,N,0.51,M,1.85,K*hh`
`$IIVWT,75,R,1.0,N,0.51,M,1.85,K*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Angle off the bow, 0 to 180 | `AngleDegrees` |
| 1 | `R` starboard, `L` port | `Side` |
| 2 | Speed, knots | `SpeedKnots` |
| 3 | `N` | fixed by the format |
| 4 | Speed, m/s | `SpeedMps`, derived |
| 5 | `M` | fixed by the format |
| 6 | Speed, km/h | `SpeedKmh`, derived |
| 7 | `K` | fixed by the format |

The angle is unsigned with a separate side letter, so `75 R` and `75 L` are
different winds and collapsing them to a signed angle would lose which side it
came from. `SignedAngle()` makes the conversion, and `TrueDirection(heading,
hasHeading)` turns it into a compass bearing, refusing to do so when the
heading is unknown rather than returning a number measured from the wrong
datum.

The two sentences differ in what the angle is against: **VWR** is the apparent
wind a crew feels, **VWT** the wind over the water. The receiver does the
correction, so one cannot be derived from the other without the vessel's motion.

Both are on the standard's "not recommended for new designs" list, with MWV
preferred. The m/s and km/h figures are derived from the knots value rather than
read, because a receiver rounds each independently.

### DTM — Datum Reference

`$GPDTM,W84,,0.0,N,0.0,E,0.0,W84*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Local datum code | `LocalDatum` |
| 1 | Subcode, may be blank | `Subcode` |
| 2 | Latitude offset, **arc minutes** | `LatOffset` |
| 3 | `N` or `S` | sign applied |
| 4 | Longitude offset, **arc minutes** | `LonOffset` |
| 5 | `E` or `W` | sign applied |
| 6 | Altitude offset, metres | `AltOffset` |
| 7 | Datum name | `DatumName` |

The offsets are in arc minutes, not degrees. A position on WGS84 and the same
point on a local datum can differ by hundreds of metres, which matters
whenever positions are compared with a chart or a survey.

The only form reliably seen in the field is the short two-field one, so
everything after the code is optional and presence-flagged.

## Vessel

### OSD — Own Ship Data

`$IIOSD,100.0,A,100.0,T,10.5,N,10.5,0.5,N*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Heading, degrees true | `Heading` |
| 1 | Status, `A` valid | `Status` |
| 2 | Vessel course, degrees true | `Course` |
| 3 | Course reference, `B`/`M`/`W`/`R`/`P` | `CourseRef` |
| 4 | Vessel speed | `Speed` |
| 5 | Speed reference | `SpeedRef` |
| 6 | Vessel set, degrees true | `Set` |
| 7 | Vessel drift, the current speed | `Drift` |
| 8 | Speed units, `K` km/h or `N` knots | `SpeedUnit` |

### RSA — Rudder Sensor Angle

`$IIRSA,10.5,A,-3.2,A*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Starboard or single rudder, degrees; negative to port | `Starboard` |
| 1 | Status `A` | `StarboardStatus` |
| 2 | Port rudder, degrees | `Port` |
| 3 | Status `A` | `PortStatus` |

A single-rudder vessel leaves the port fields blank, and a decoder that
required both would miss every such boat. `RudderAngle` returns the mean and
the half-difference, the latter being the yaw the hull experiences.

## AIS and alerting

### VDM / VDO — AIS Data and Own Data

`!AIVDM,1,1,,B,177KQJ5000G?tO` + "`" + `K>RA1wUbN0TKH,0*hh`
`!AIVDO,1,1,,A,177KQJ5000G?tO` + "`" + `K>RA1wUbN0TKH,0*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Total fragments | `TotalFragments` |
| 1 | Fragment number | `FragmentNumber` |
| 2 | Message identifier | `SequenceID` |
| 3 | Channel, `A` or `B` | `Channel` |
| 4 | Payload, six-bit ASCII armouring | `Payload` |
| 5 | Fill bits, 0 to 5 | `FillBits` |

AIS sentences are framed with `!` rather than `$`, and are accepted. The
sequence id is what makes multi-fragment messages reassemblable, and a fragment
number outside its cycle is rejected rather than reported as a message that will
never complete.

The payload is six-bit ASCII armouring, decoded to bytes in both directions. Each
character carries six bits: values 0 to 39 sit at ASCII 48 to 87 and values 40 to
63 at ASCII 96 to 119. **The gap between them, ASCII 88 to 95, carries no value**,
and a character there means the payload is corrupt; it is rejected rather than
decoded to something plausible.

A payload that is not a whole number of bytes is the normal case, since AIS
payloads are 6, 8, 10 bits and so on. The trailing partial byte is dropped, not
zero-padded, because padding would append a fabricated byte to every message.

`OwnShip` distinguishes the two directions: false for VDM, a message from
another vessel, true for VDO, one this vessel sent.

### ABM — AIS Addressed Binary Message

`!AIABM,26,2,1,3381581370,3,8,177KQJ5000G?tO` + "`" + `K>RA1wUbN0TKH,0*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0, 1, 2 | Fragments, fragment number, message id | as `VDM` |
| 3 | MMSI of the destination station | `MMSI` |
| 4 | Channel | `Channel` |
| 5 | VDL message number, ITU-R M.1371 | `VDLMessageNumber` |
| 6 | Payload | `Payload` |
| 7 | Fill bits | `FillBits` |

The payload sits one field later than VDM's because of the addressee, and
getting that index wrong produces a message decoded to bytes shifted by a whole
field, which is plausible enough to be believed. The MMSI is a string, since it
is an identity and a ten-digit number does not fit an int32 everywhere.

### BBM — AIS Broadcast Binary Message

`!AIBBM,26,2,1,3,8,177KQJ5000G?tO` + "`" + `K>RA1wUbN0TKH,0*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0, 1, 2 | Fragments, fragment number, message id | as `VDM` |
| 3 | Channel | `Channel` |
| 4 | VDL message number | `VDLMessageNumber` |
| 5 | Payload | `Payload` |
| 6 | Fill bits | `FillBits` |

BBM is ABM without the addressee, so its payload sits one field earlier. That
single difference is the whole reason it is a separate sentence.

### ACK — Alert Acknowledgement

`$VRACK,001*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Alert identifier, 001 to 99999 | `AlertIdentifier` |

A bare number, and that is all it is: the identifier refers to an alert
sentence this library does not decode, so a program receiving one has an
acknowledgement it cannot resolve to anything. Decoded because it is one field
and appears on alert-capable equipment.

## Text and identification

### TXT — Text Transmission

`$GNTXT,01,01,02,u-blox AG - www.u-blox.com*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Total sentences in this message, 01–99 | `TotalMessages` |
| 1 | Sentence number, 01–99 | `MessageNumber` |
| 2 | Message id, 01–99 | `MessageID` |
| 3+ | Text, ASCII | `Text` |

A message is split across sentences, so it is only meaningful reassembled.
`TextAccumulator` does that, and resets when a new sequence starts so two
messages do not get spliced together.

The message id is defined only as a number. The sub-coding where 01 is an
error and 02 a warning is a **u-blox convention, not the standard**, so
`TextMessageID` offers it without asserting it.

### VER — Version and Model Identification

`$GPVER,SiRF,GSiRF03,1000,1.00*hh`

| Field | Meaning | Go field |
|---|---|---|
| 0 | Manufacturer | `Manufacturer` |
| 1 | Product | `Product` |
| 2 | Software version | `Software` |
| 3 | Receiver version, usually a build number | `Receiver` |

Field 3 is usually a build number, not a standard revision, so it is only
read as a version when it genuinely looks like one. Some receivers state the
standard revision in the software field with an explicit `NMEA` marker, which
is also recognised.

VER is the only sentence that states a version outright. Almost no receiver
sends one, which is why `Parser.Detector` infers the version from the shape
of the traffic instead.

## Deliberate omissions

The full standard defines over 130 sentences. What is missing here is
missing on purpose, not overlooked.

**No field layouts are publicly available** for these, and guessing indices
for integrity-relevant data would be worse than omitting it:

- `GIR`, `GRP`, `GGC`, `GCF`, `GSN` — new in NMEA 4.30
- `SMV` — new in NMEA 4.30
- `GFA` — GNSS fix accuracy and integrity, used for SOLAS

**Present in the standard, not implemented here**, because they are outside
this library's purpose: the AIS set beyond the envelope (`ABK` `ACA` `AIR`
`LR1`-`LRF` `TDS` `TRD` `TTD` `VSD`), the alert set beyond the acknowledgement
(`ACN` `ALA` `ALC` `ALF` `ALR` `ARC`), the rest of the marine-electronics set
(`MSK` `MSS` `RLM` `RSD` `SFI` `TPC` `TPR` `TPT` `VDR` `VSD`), and the
deprecated ones (`APA` `BWW` `DRU` `GLa` `GOa` `HCC`).

A note on the AIS set, because it is easy to over-read. `ABM`, `BBM`, `VDM`, and
`VDO` decode the **envelope**: the fragment count and number, the sequence id
that lets fragments be reassembled, the channel, and the six-bit-armoured
payload as bytes. The payload's *meaning* is a separate protocol, ITU-R M.1371,
and is not decoded here. Decoding it belongs in an AIS library, and a partial
implementation inside an NMEA parser would be the wrong place for it.

Adding any of them is one file and one line in the decoder list. See the
package documentation for the pattern.

## Sources and known source errors

Field layouts were verified against the NMEA 0183 documentation and
cross-checked against a second independent Go implementation. Each sentence's
own `sentences/<type>_test.go` turns that into executable assertions, so a
decoder whose indices are off by one fails the build rather than silently
producing plausible wrong numbers:
each sentence is fed the published example and every documented field is
checked, so a decoder whose indices are off by one fails the test rather
than producing plausible wrong numbers in an application.

**Checksums are computed locally rather than copied.** Many published
example sentences carry a checksum that does not verify against their own
body. `TestPublishedChecksumErrors` lists them, so the discrepancy is
recorded rather than quietly worked around. The checksum implementation
itself is validated against the checksums the standard prints for its own
examples, which is the strongest available check that the XOR covers
exactly the right bytes.

Two published examples also omit the leading `$`, and one published field
list (GBS) contradicts its own example; both are handled and documented at
the point of use.

