package wire

import (
	value "github.com/tamalmaity-dev/nmea-go-parser/value"
)

// Type is a sentence formatter, the three characters that follow the talker:
// "GGA", "RMC", "GSV", and so on. It is a plain string so it compares
// directly against a constant and prints readably.
type Type = string

// The sentence formatters this library decodes. A formatter identifies a
// sentence type across talkers: "GNGGA", "GPGGA", and "GAGGA" are all TypeGGA
// from three different systems.
const (
	TypeAAM = "AAM"
	TypeAPB = "APB"
	TypeBOD = "BOD"
	TypeBWC = "BWC"
	TypeBWR = "BWR"
	TypeDPT = "DPT"
	TypeDTM = "DTM"
	TypeGBS = "GBS"
	TypeGGA = "GGA"
	TypeGLL = "GLL"
	TypeGNS = "GNS"
	TypeGRS = "GRS"
	TypeGSA = "GSA"
	TypeGST = "GST"
	TypeGSV = "GSV"
	TypeHDG = "HDG"
	TypeHDT = "HDT"
	TypeMTW = "MTW"
	TypeMWD = "MWD"
	TypeMWV = "MWV"
	TypeOSD = "OSD"
	TypeRMB = "RMB"
	TypeRMC = "RMC"
	TypeROT = "ROT"
	TypeRSA = "RSA"
	TypeRTE = "RTE"
	TypeTHS = "THS"
	TypeTXT = "TXT"
	TypeVBW = "VBW"
	TypeVER = "value.VER"
	TypeVHW = "VHW"
	TypeVLW = "VLW"
	TypeVTG = "VTG"
	TypeWCV = "WCV"
	TypeWPL = "WPL"
	TypeXTE = "XTE"
	TypeZDA = "ZDA"
)

// DecodedSentence is implemented by every decoded sentence value. It is the
// minimum
// a program needs to route a sentence without knowing its concrete type,
// which makes a type switch on DataType possible:
//
//	for s := range events {
//	    switch s.DataType() {
//	    case nmea.TypeRMC:
//	        m := s.(nmea.RMC)
//	        use(m)
//	    case nmea.TypeGGA:
//	        g := s.(nmea.GGA)
//	        use(g)
//	    }
//	}
//
// The concrete types live in the sentences subpackage, so the assertions
// read nmea.RMC after importing that package under the name nmea, or
// sentences.RMC with its own name. See the package documentation for a
// working example.
type DecodedSentence interface {
	// DataType is the sentence formatter, one of the Type constants.
	DataType() Type
	// Talker is the two-character source designator, e.g. "GP" or "GN".
	Talker() string
	// Address is the talker and formatter together, e.g. "GNGGA".
	Address() string
	// Raw is the sentence exactly as it arrived.
	Raw() string
	// Constellation is the satellite system the sentence came from, taken
	// from the talker. It is part of this interface because a handler that
	// routes on the sentence type almost always wants it as well, and
	// deriving it means a second type assertion.
	Constellation() value.Constellation
	// String returns the raw sentence, so a decoded value prints as its wire
	// form in a log line rather than as a struct dump.
	String() string
}

// DataTypeOf returns the formatter of an arbitrary decoded value, or "" when
// it is not a sentence. It lets a caller branch on a value without asserting
// it is a Sentence first.
func DataTypeOf(value any) Type {
	if s, ok := value.(DecodedSentence); ok {
		return s.DataType()
	}
	return ""
}
