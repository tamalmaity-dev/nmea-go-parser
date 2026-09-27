package wire

import (
	"errors"
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser/fault"
)

func TestChecksumStatusVerdicts(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantStatus ChecksumStatus
		wantOK     bool
		wantErr    error
	}{
		{
			name:   "valid",
			in:     "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18",
			wantOK: true, wantStatus: ChecksumValid,
		},
		{
			name:   "absent is acceptable",
			in:     "$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,",
			wantOK: true, wantStatus: ChecksumAbsent,
		},
		{
			name:   "invalid",
			in:     "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*19",
			wantOK: false, wantStatus: ChecksumInvalid, wantErr: fault.ErrBadChecksum,
		},
		{
			name:   "not hexadecimal",
			in:     "$GPGGA,123519*XY",
			wantOK: false, wantStatus: ChecksumMalformed, wantErr: fault.ErrMalformedChecksum,
		},
		{
			name:   "one digit",
			in:     "$GPGGA,123519*1",
			wantOK: false, wantStatus: ChecksumMalformed, wantErr: fault.ErrMalformedChecksum,
		},
		{
			name:   "star with nothing after it",
			in:     "$GPGGA,123519*",
			wantOK: false, wantStatus: ChecksumMalformed, wantErr: fault.ErrMalformedChecksum,
		},
		{
			name:   "trailing junk after a valid checksum is tolerated",
			in:     "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\x00\r\n",
			wantOK: true, wantStatus: ChecksumValid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, status := Split(tc.in)
			if status != tc.wantStatus {
				t.Errorf("Split status = %v, want %v", status, tc.wantStatus)
			}
			if status.OK() != tc.wantOK {
				t.Errorf("status.OK() = %v, want %v", status.OK(), tc.wantOK)
			}

			err := Validate(tc.in)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("Validate = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate = %v, want %v", err, tc.wantErr)
			}
			if Valid(tc.in) {
				t.Error("Valid = true, want false")
			}
		})
	}
}

func TestSplitReturnsExactBody(t *testing.T) {
	// Split's body must be precisely the string Checksum expects, or a
	// caller validating by hand gets a different answer from the library.
	const framed = "$GPWPL,4917.16,N,12310.64,W,003*65"
	body, received, status := Split(framed)

	if status != ChecksumValid {
		t.Fatalf("status = %v, want valid", status)
	}
	if want := "GPWPL,4917.16,N,12310.64,W,003"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if received != 0x65 {
		t.Errorf("received = %02X, want 65", received)
	}
	if got := Checksum(body); got != received {
		t.Errorf("Checksum(body) = %02X, does not match received %02X", got, received)
	}
}

func TestSplitProprietaryWithStar(t *testing.T) {
	// A proprietary payload can contain its own '*', so only the last one
	// may be treated as the checksum separator. Getting this wrong truncates
	// the sentence and silently drops fields.
	body, _, status := Split("$PMTK001,1,0*32")
	if status != ChecksumValid {
		t.Fatalf("status = %v, want valid", status)
	}
	if want := "PMTK001,1,0"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestFrameValidateRoundTrip(t *testing.T) {
	// Every sentence in the sample data must survive Frame and Validate
	// unchanged. This is the property a program relies on when it reads a
	// route out of a receiver and writes it back.
	for _, line := range sampleLines(t) {
		body, _, status := Split(line)
		if status != ChecksumValid {
			t.Errorf("%s: status = %v, want valid", line, status)
			continue
		}
		reframed := FrameWith(line[:1], body)
		if reframed != line {
			t.Errorf("round trip changed the sentence:\n got %s\nwant %s", reframed, line)
		}
	}
}

func TestValidateRejectsNonNMEA(t *testing.T) {
	for _, in := range []string{"hello", "", "   ", "12345"} {
		if err := Validate(in); !errors.Is(err, fault.ErrNotSentence) {
			t.Errorf("Validate(%q) = %v, want fault.ErrNotSentence", in, err)
		}
	}
}

func TestChecksumIsOrderIndependentOfTerminator(t *testing.T) {
	// CRLF, LF, and CR must all be stripped before the checksum is read,
	// because a serial driver that translates line endings changes the
	// bytes but not the sentence.
	const body = "$GPRTE,1,1,c,0*07"
	for _, suffix := range []string{"", "\r\n", "\n", "\r", "\x00\x00", "  \r\n"} {
		line := body + suffix
		if err := Validate(line); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", line, err)
		}
	}
}

func TestChecksumExcludesDollarAndStar(t *testing.T) {
	// The standard is explicit that the '$' and '*' are excluded but the
	// commas are included. A one-character change in any of those must
	// change the result, which is what makes the checksum a real check.
	base := "GPGGA,1,2,3"
	if Checksum(base) == Checksum("$"+base) {
		t.Error("including the '$' did not change the checksum")
	}
	if Checksum(base) == Checksum(base+"*") {
		t.Error("including the '*' did not change the checksum")
	}
	if Checksum(base) == Checksum("GPGGA-1,2,3") {
		t.Error("replacing a comma did not change the checksum")
	}
}

func TestChecksumLongSentence(t *testing.T) {
	// A body of unusual length must still XOR cleanly, including one
	// containing high-bit and non-ASCII bytes that a receiver might send.
	body := strings.Repeat("A", 1000)
	if Checksum(body) != 0 { // 1000 is even, so A XOR A cancels out
		t.Errorf("Checksum of 1000 A's = %02X, want 00", Checksum(body))
	}
	if Checksum("ÿ") == 0 {
		t.Error("Checksum of a high-bit byte = 00, want non-zero")
	}
}
