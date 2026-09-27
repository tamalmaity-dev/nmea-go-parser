package device

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// stubPort is a minimal serial.Port used to exercise the send and close paths
// without hardware. A real port cannot be opened in a unit test, so the parts
// that do not touch the OS are tested against this.
type stubPort struct {
	written   bytes.Buffer
	reads     [][]byte
	readErr   error
	closed    bool
	closeErr  error
	setTOut   time.Duration
	writeFail error
}

func (p *stubPort) SetMode(*serial.Mode) error { return nil }
func (p *stubPort) Read(b []byte) (int, error) {
	if len(p.reads) > 0 {
		n := copy(b, p.reads[0])
		p.reads = p.reads[1:]
		if len(p.reads) == 0 && p.readErr != nil {
			return n, p.readErr
		}
		return n, nil
	}
	return 0, p.readErr
}
func (p *stubPort) Write(b []byte) (int, error) {
	if p.writeFail != nil {
		return 0, p.writeFail
	}
	return p.written.Write(b)
}
func (p *stubPort) Drain() error             { return nil }
func (p *stubPort) ResetInputBuffer() error  { return nil }
func (p *stubPort) ResetOutputBuffer() error { return nil }
func (p *stubPort) SetDTR(bool) error        { return nil }
func (p *stubPort) SetRTS(bool) error        { return nil }
func (p *stubPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return nil, nil
}
func (p *stubPort) SetReadTimeout(d time.Duration) error {
	p.setTOut = d
	return nil
}
func (p *stubPort) Break(time.Duration) error { return nil }
func (p *stubPort) Close() error              { p.closed = true; return p.closeErr }

func TestOpenRequiresPort(t *testing.T) {
	// The most common configuration mistake, caught before any syscall.
	if _, err := Open(Config{}); !errors.Is(err, ErrNoPort) {
		t.Errorf("Open with no port = %v, want ErrNoPort", err)
	}
	if _, err := OpenParser(Config{}, nil); !errors.Is(err, ErrNoPort) {
		t.Errorf("OpenParser with no port = %v, want ErrNoPort", err)
	}
}

func TestNewReader(t *testing.T) {
	if _, err := NewReader(nil, nil); err == nil {
		t.Error("NewReader(nil) succeeded, want an error")
	}

	d, err := NewReader(strings.NewReader("$GPRMC,123519,A*6A\r\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// A reader-backed device has no serial port behind it.
	if d.Port() != nil {
		t.Error("Port is non-nil for a reader-backed device")
	}
	// The built-in decoders must be available, since importing this package
	// is supposed to bring them in.
	if d.Parser().Registry().Len() == 0 {
		t.Error("the attached parser has no decoders; importing device should register them all")
	}
}

func TestNewReaderConsumes(t *testing.T) {
	stream := "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n" +
		"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10\r\n"

	d, err := NewReader(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatal(err)
	}

	var positions int
	d.OnFix(func(f *nmea.Fix) error {
		if f.HasPosition {
			positions++
		}
		return nil
	})

	if err := d.Consume(context.Background()); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if positions == 0 {
		t.Error("no positions were reported")
	}
	if !d.Fix().HasPosition {
		t.Error("Fix has no position after consuming a stream")
	}
	// Fix() is the shorthand for the parser's own.
	if d.Fix().Latitude != d.Parser().Fix().Latitude {
		t.Error("Device.Fix and Parser.Fix disagree")
	}
}

func TestNewReaderCloseIsIdempotent(t *testing.T) {
	d, err := NewReader(strings.NewReader(""), nil)
	if err != nil {
		t.Fatal(err)
	}
	// A deferred Close alongside an explicit one must not fail.
	if err := d.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if !d.Closed() {
		t.Error("Closed = false after Close")
	}
	// Consuming a closed device is a clean shutdown, not a failure.
	if err := d.Consume(context.Background()); err != nil {
		t.Errorf("Consume after Close = %v, want nil", err)
	}
}

func TestSendAddsFraming(t *testing.T) {
	d, err := NewReader(nil, nil)
	if err == nil {
		// NewReader rejects a nil reader, so build the device directly.
		d = newDevice("stub", nil, nil, nmea.New())
	}

	if err := d.Send(""); err == nil {
		t.Error("Send(\"\") succeeded, want an error")
	}

	// A nil reader-backed device cannot be written to.
	if err := d.Send("PUBX,00"); err == nil {
		t.Error("Send on a device with no writer succeeded, want an error")
	}
}

func TestSendOnStubPort(t *testing.T) {
	port := &stubPort{}
	d := newDevice("COM3", port, nil, nmea.New())

	// A body without framing gets a '$' and a checksum.
	if err := d.Send("PUBX,00"); err != nil {
		t.Fatal(err)
	}
	if got, want := port.written.String(), "$PUBX,00*33"; got != want {
		t.Errorf("Send wrote %q, want %q", got, want)
	}

	// An already-framed sentence is sent unchanged, so a program that built
	// it with nmea.Frame does not get it double-framed.
	port.written.Reset()
	if err := d.Send("$PUBX,00*33"); err != nil {
		t.Fatal(err)
	}
	if got := port.written.String(); got != "$PUBX,00*33" {
		t.Errorf("Send wrote %q, want the sentence unchanged", got)
	}

	// No newline is appended: NMEA has no line terminator and some receivers
	// treat one as a second, malformed command.
	if strings.Contains(port.written.String(), "\n") {
		t.Error("Send appended a line terminator")
	}
}

func TestSendOnClosedDevice(t *testing.T) {
	port := &stubPort{}
	d := newDevice("COM3", port, nil, nmea.New())
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Send("PUBX,00"); !errors.Is(err, ErrDeviceClosed) {
		t.Errorf("Send after Close = %v, want ErrDeviceClosed", err)
	}
	// The port really is closed.
	if !port.closed {
		t.Error("the underlying port was not closed")
	}
}

func TestDeviceReadSatisfiesIOReader(t *testing.T) {
	port := &stubPort{reads: [][]byte{[]byte("hello\n")}}
	d := newDevice("COM3", port, nil, nmea.New())

	var r io.Reader = d
	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello\n" {
		t.Errorf("Read = %q, want %q", buf[:n], "hello\n")
	}

	// Reading a closed device reports it rather than blocking.
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Read(buf); !errors.Is(err, ErrDeviceClosed) {
		t.Errorf("Read after Close = %v, want ErrDeviceClosed", err)
	}
}

func TestIsTimeout(t *testing.T) {
	// The serial package does not export a timeout sentinel, so this is the
	// check that keeps a quiet receiver from being reported as a dead one.
	tests := []struct {
		in   error
		want bool
	}{
		{in: nil, want: false},
		{in: errors.New("read tcp: i/o timeout"), want: true},
		{in: errors.New("Timeout"), want: true},
		{in: errors.New("something else"), want: false},
		{in: io.EOF, want: false},
	}
	for _, tc := range tests {
		if got := isTimeout(tc.in); got != tc.want {
			t.Errorf("isTimeout(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestClosedDeviceIsUsableAsNil(t *testing.T) {
	// Every method tolerates a nil receiver, so a program can hold a
	// *Device that was never opened without a nil check at each use.
	var d *Device
	if !d.Closed() {
		t.Error("a nil device does not report closed")
	}
	if err := d.Close(); err != nil {
		t.Errorf("Close on nil: %v", err)
	}
	if err := d.Consume(context.Background()); !errors.Is(err, ErrNotOpen) {
		t.Errorf("Consume on nil = %v, want ErrNotOpen", err)
	}
	if err := d.Send("x"); !errors.Is(err, ErrNotOpen) {
		t.Errorf("Send on nil = %v, want ErrNotOpen", err)
	}
	if _, err := d.Read(make([]byte, 1)); !errors.Is(err, ErrNotOpen) {
		t.Errorf("Read on nil = %v, want ErrNotOpen", err)
	}
}

func TestStringIsLoggable(t *testing.T) {
	d, err := NewReader(strings.NewReader(""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "reader") {
		t.Errorf("String() = %q, want it to name the device", d.String())
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "closed") {
		t.Errorf("String() after Close = %q, want it to say closed", d.String())
	}
}
