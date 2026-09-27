// Package device opens a serial port and wires it to a Parser.
//
// It exists so the common case, a GNSS receiver on a COM port or a USB
// serial adapter, is one call rather than a port library in the caller's own
// go.mod. The parser has no dependency on this package or on any serial
// library, so a program that reads from a socket, a file, or a Bluetooth
// characteristic never pulls the dependency in at all.
//
//	dev, err := device.Open(device.Config{Port: "COM3", BaudRate: 9600})
//	if err != nil {
//	    return err
//	}
//	defer dev.Close()
//
//	dev.OnFix(func(f *nmea.Fix) error {
//	    log.Println(f.Latitude, f.Longitude)
//	    return nil
//	})
//	return dev.Consume(ctx)
//
// Importing this package registers every built-in sentence decoder, so the
// sentences package does not need importing as well.
package device

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/tamalmaity-dev/nmea-go-parser"
	_ "github.com/tamalmaity-dev/nmea-go-parser/sentences" // registers the built-in decoders
)

// Errors returned by this package.
var (
	// ErrNoPort means Config.Port was empty.
	ErrNoPort = errors.New("device: no serial port specified")

	// ErrNotOpen means the device was used after Close, or a zero Device
	// was used without opening it.
	ErrNotOpen = errors.New("device: port is not open")

	// ErrDeviceClosed means the read loop was stopped because the port was
	// closed underneath it, which is the normal result of a Close racing a
	// running Consume.
	ErrDeviceClosed = errors.New("device: port closed")
)

// DefaultBaudRate is the NMEA 0183 standard rate and what most receivers
// ship with. There is no autodetect, because a wrong baud rate produces
// plausible-looking garbage rather than an error, so guessing would be worse
// than asking.
const DefaultBaudRate = 9600

// Config describes how to open a port. Only Port is required; the rest
// default to 9600 baud 8N1, which is what every GNSS receiver uses.
type Config struct {
	// Port is the device name. On Windows "COM3", or "\\.\COM17" for a
	// high-numbered port; on Linux "/dev/ttyUSB0" or "/dev/ttyACM0"; on
	// macOS "/dev/cu.usbmodem14201", using cu rather than tty so the port is
	// not held open by the terminal layer. Use Ports to discover the real
	// name instead of guessing it.
	Port string

	// BaudRate is bits per second. 9600 is the standard; 4800 and 38400
	// also occur in the field.
	BaudRate int

	// DataBits, Parity, and StopBits take their usual meanings. A zero
	// DataBits or StopBits becomes 8 and 1, since zero is not a valid
	// setting for either. A zero Parity is already no-parity, which is
	// correct.
	DataBits int
	Parity   serial.Parity
	StopBits serial.StopBits

	// ReadTimeout bounds a single Read. When it elapses the read returns a
	// timeout error, which the parse loop treats as "nothing arrived" and
	// not as end of stream. This is what makes a silent receiver
	// detectable rather than looking like a hung one. Zero means block
	// until data arrives.
	ReadTimeout time.Duration

	// MaxLineLength overrides the parser's sentence length limit for
	// receivers that emit unusually verbose sentences.
	MaxLineLength int
}

// Device is an open serial port with a Parser attached.
//
// A Device is safe for concurrent use, so the read loop, the write path, and
// handler registration do not need external coordination. Close is
// idempotent, so a deferred Close alongside an explicit one on an error path
// is fine.
type Device struct {
	port   serial.Port
	reader io.Reader
	name   string
	parser *nmea.Parser

	mu       sync.Mutex
	closed   bool
	closedCh chan struct{}
}

// newDevice wires up the fields every constructor needs, including the
// close-notification channel that the read loop watches.
func newDevice(name string, port serial.Port, reader io.Reader, parser *nmea.Parser) *Device {
	return &Device{
		port:     port,
		reader:   reader,
		name:     name,
		parser:   parser,
		closedCh: make(chan struct{}),
	}
}

// Open opens the port described by cfg and attaches a default parser. The
// caller owns the returned Device and must Close it.
func Open(cfg Config) (*Device, error) { return OpenParser(cfg, nil) }

// OpenParser is Open with control over the parser, for a program that needs
// history, a sentence channel, or handlers installed before the first
// sentence arrives.
//
//	dev, err := device.OpenParser(cfg, []nmea.Option{
//	    nmea.WithChannelBuffer(32),
//	    nmea.WithKeepHistory(50),
//	})
func OpenParser(cfg Config, parserOpts []nmea.Option) (*Device, error) {
	if cfg.Port == "" {
		return nil, ErrNoPort
	}
	if cfg.BaudRate == 0 {
		cfg.BaudRate = DefaultBaudRate
	}

	mode := &serial.Mode{
		BaudRate: cfg.BaudRate,
		DataBits: cfg.DataBits,
		Parity:   cfg.Parity,
		StopBits: cfg.StopBits,
	}
	if mode.DataBits == 0 {
		mode.DataBits = 8
	}
	if mode.StopBits == 0 {
		mode.StopBits = 1
	}

	port, err := serial.Open(cfg.Port, mode)
	if err != nil {
		return nil, fmt.Errorf("device: opening %s at %d baud: %w", cfg.Port, cfg.BaudRate, err)
	}

	if cfg.ReadTimeout > 0 {
		if err := port.SetReadTimeout(cfg.ReadTimeout); err != nil {
			port.Close()
			return nil, fmt.Errorf("device: setting read timeout on %s: %w", cfg.Port, err)
		}
	}

	opts := parserOpts
	if cfg.MaxLineLength > 0 {
		opts = append(opts, nmea.WithMaxLineLength(cfg.MaxLineLength))
	}

	return newDevice(cfg.Port, port, nil, nmea.New(opts...)), nil
}

// New wraps an already-open io.Reader, for a source that is not a serial
// port: a TCP stream, a Bluetooth socket, or a file. It attaches a parser
// and brings in the built-in decoders, so it is the bridge for every
// non-serial case.
//
//	dev, err := device.NewReader(tcpConn, nil)
func NewReader(r io.Reader, parserOpts []nmea.Option) (*Device, error) {
	if r == nil {
		return nil, errors.New("device: nil reader")
	}
	return newDevice("reader", nil, r, nmea.New(parserOpts...)), nil
}

// Port returns the underlying serial port, for programs that need to
// configure it further. It is nil for a Device created by NewReader.
func (d *Device) Port() serial.Port { return d.port }

// Parser returns the attached parser, so the Fix can be polled and
// per-sentence handlers registered.
func (d *Device) Parser() *nmea.Parser { return d.parser }

// Fix returns the current merged fix.
func (d *Device) Fix() nmea.Fix { return d.parser.Fix() }

// OnFix registers a handler called for every sentence that updates the fix.
// The handler runs on the read loop, so keep it short.
func (d *Device) OnFix(fn func(*nmea.Fix) error) { d.parser.OnFix(fn) }

// On registers a handler for one sentence formatter, e.g. "GGA".
func (d *Device) On(typ string, fn func(nmea.Event) error) { d.parser.On(typ, fn) }

// OnEvent registers a handler for every framed sentence.
func (d *Device) OnEvent(fn func(nmea.Event) error) { d.parser.OnEvent(fn) }

// OnError registers a handler for bad checksums and decode failures.
func (d *Device) OnError(fn func(error)) { d.parser.OnError(fn) }

// Consume reads from the device until ctx is done, the port fails, or the
// device is closed. It blocks, so run it in its own goroutine:
//
//	go func() { dev.Consume(ctx) }()
//
// It returns nil on a clean shutdown, whether that came from context
// cancellation or from Close, so neither is reported as a failure.
//
// A read timeout is not an error: it means the receiver is quiet, which is
// an ordinary state for a device with no fix, so the loop keeps waiting. A
// genuine port failure is returned, since retrying in place would not help.
func (d *Device) Consume(ctx context.Context) error {
	if d == nil {
		return ErrNotOpen
	}
	d.mu.Lock()
	closed, reader := d.closed, d.reader
	d.mu.Unlock()
	if closed {
		// Already closed, so there is nothing left to read and nothing went
		// wrong. Returning ErrDeviceClosed here would contradict the shutdown
		// contract above, and would turn the ordinary "Close from another
		// goroutine while Consume is blocked" pattern into a spurious error.
		return nil
	}
	if reader == nil && d.port == nil {
		return ErrNotOpen
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Watch for Close so a read blocked on an idle port is released rather
	// than waiting out the read timeout.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		case <-d.done():
			cancel()
		}
	}()

	for {
		err := d.parser.Consume(ctx, d.source())
		switch {
		case err == nil:
			return nil
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			if d.Closed() {
				return nil
			}
			return nil
		case errors.Is(err, ErrDeviceClosed):
			return nil
		case isTimeout(err):
			// Nothing arrived within the window. Wait a moment and go round
			// again, so a dead device does not spin the CPU retrying a port
			// that will never answer.
			if d.Closed() {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryPause):
			}
			continue
		default:
			if d.Closed() {
				return nil
			}
			return fmt.Errorf("device: reading %s: %w", d.name, err)
		}
	}
}

// retryPause is how long the read loop pauses after a read timeout.
const retryPause = 100 * time.Millisecond

// source returns the reader to consume: the serial port, or the plain
// reader a Device was created with. It does not check the closed flag, so
// callers must hold the lock or check it first.
func (d *Device) source() io.Reader {
	if d.reader != nil {
		return d.reader
	}
	if d.port == nil {
		return nil
	}
	return d.port
}

// Read makes a Device an io.Reader, so a caller that wants to drive the parse
// loop itself can hand the device straight to a parser:
//
//	err := nmea.New().Consume(ctx, dev)
//
// Prefer Consume, which handles the read timeout and reports a closed port as
// a clean shutdown rather than an error.
func (d *Device) Read(p []byte) (int, error) {
	if d == nil {
		return 0, ErrNotOpen
	}
	d.mu.Lock()
	closed, r := d.closed, d.source()
	d.mu.Unlock()
	if closed {
		return 0, ErrDeviceClosed
	}
	if r == nil {
		return 0, ErrNotOpen
	}
	return r.Read(p)
}

// Send writes a sentence to the receiver, adding the '$' prefix and '*hh'
// checksum if the caller left them off. It is how a program configures a
// receiver or loads a route.
//
//	dev.Send("PUBX,00")          // sent as $PUBX,00*33
//	dev.Send(nmea.Frame(body))  // already framed, sent as-is
//
// No newline is appended: NMEA has no line terminator, and some receivers
// treat a trailing one as a second, malformed command.
func (d *Device) Send(sentence string) error {
	if d == nil {
		return ErrNotOpen
	}
	if sentence == "" {
		return errors.New("device: refusing to send an empty sentence")
	}
	w, err := d.writer()
	if err != nil {
		return err
	}
	if sentence[0] != '$' {
		sentence = nmea.Frame(sentence)
	}
	if _, err := io.WriteString(w, sentence); err != nil {
		return fmt.Errorf("device: writing %q: %w", sentence, err)
	}
	return nil
}

func (d *Device) writer() (io.Writer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, ErrDeviceClosed
	}
	if d.reader != nil {
		if w, ok := d.reader.(io.Writer); ok {
			return w, nil
		}
		return nil, errors.New("device: reader is not writable")
	}
	if d.port == nil {
		return nil, ErrNotOpen
	}
	return d.port, nil
}

// Close closes the port. It is idempotent, so a deferred Close alongside an
// explicit one on an error path will not fail.
func (d *Device) Close() error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	close(d.closedCh)
	port := d.port
	d.mu.Unlock()

	if port == nil {
		return nil
	}
	if err := port.Close(); err != nil {
		return fmt.Errorf("device: closing %s: %w", d.name, err)
	}
	return nil
}

// Closed reports whether Close has been called.
func (d *Device) Closed() bool {
	if d == nil {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// done returns a channel closed when the device is closed.
func (d *Device) done() <-chan struct{} { return d.closedCh }

// String describes the device, which makes it usable directly in a log line.
func (d *Device) String() string {
	if d == nil {
		return "device: nil"
	}
	if d.Closed() {
		return "device: " + d.name + " (closed)"
	}
	return "device: " + d.name
}

// Ports lists the serial ports available on this machine. It is the first
// thing to call when Open fails, since "COM3" and "/dev/ttyUSB0" are guesses
// and the real name is a system fact.
//
// On Linux a port held open by another process may be absent, because the
// kernel restricts /dev entries by owner.
func Ports() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("device: listing serial ports: %w", err)
	}
	return ports, nil
}

// isTimeout reports whether an error is an ordinary read timeout. The
// serial package does not export a sentinel for it, and the wording is
// stable across its platforms.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") || strings.Contains(s, "Timeout")
}
