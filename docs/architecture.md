# Architecture

How this module is put together, and why. For the field-by-field reference to
each sentence, see [nmea.md](nmea.md). For how to use it, see the
[README](../README.md).

## The shape of it

Six packages. The dependency runs one way and never back, which is the single
constraint every other decision here follows from.

```
  fault  ←  value  ←  wire  ←  nmea  ←  sentences  ←  device
```

| Package | Directory | Depends on | Owns |
|---|---|---|---|
| `fault` | `fault/` | — | the sentinel errors, `MaxSentenceLength` |
| `value` | `value/` | `fault` | the values a field carries, the enums the standard gives them, the formatters |
| `wire` | `wire/` | `value`, `fault` | framing: `Sentence`, checksums, the line splitter, tag blocks, `Type` |
| `nmea` | `.` (root) | `wire`, `value`, `fault` | the parser, the decoder registry, the aggregate `Fix`, and the re-exports |
| `sentences` | `sentences/` | `nmea`, `wire`, `value`, `fault` | 56 decoders, one per sentence formatter |
| `device` | `device/` | `nmea`, `sentences` | serial ports |

Nothing imports upward. A value does not know what a sentence is; framing does
not know what a field means; a fault does not know about either. That is what
lets a decoder depend on what a field *means* without depending on the
machinery that delivered it.

**Framing is split out because it has to be right for every sentence**, not
just the ones a decoder understands. A malformed proprietary sentence still has
to be framed, checksummed, and counted correctly, so those rules are tested
against the recorded corpus with no decoder and no registry in the way.

## Why the root package is a façade

The three lower packages are re-exported by the root package. Those
re-exports are aliases:

```go
type Coordinate = value.Coordinate      // alias, not a definition
type Sentence   = wire.Sentence        // alias, not a definition
const QualityGPS  = value.QualityGPS   // the type is inferred
var ParseTOD      = value.ParseTOD     // Go has no func alias

var ErrBadChecksum = fault.ErrBadChecksum
```

An alias is not a new type, it is the *same* type under a second name. So:

- `nmea.Sentence` and `wire.Sentence` are interchangeable, and a decoder
  written against either compiles against the other.
- `nmea.Coordinate` and `value.Coordinate` are the same type.
- `errors.Is(err, nmea.ErrBadChecksum)` and `errors.Is(err, fault.ErrBadChecksum)`
  behave identically, because they are the same value.
- Every decoder, every example, and every line of documentation written before
  the split still compiles and still passes its tests unchanged.

This is asserted, not assumed: `alias_test.go` checks that the types are
identical, that the method sets are shared, and that errors match either way.

The layering is therefore available but never mandatory. Import `wire`,
`value` and `fault` when you are writing a decoder and want the structure
visible; ignore them entirely when you just want to read a serial port.

## The decode pipeline

One sentence's journey, and the file that owns each step:

```
  bytes
    │
    ▼  readLoop          nmea.go      the only read path; owns the buffer,
    │                               serialises itself so two Consume calls
    │                               cannot interleave their bytes
    ▼  feedBytes         nmea.go      bytes → complete lines
    │
    ▼  LineSplitter      wire/        buffers across reads, because a serial
    │                               read is an arbitrary slice of bytes
    ▼  ParseSentence     wire/        strip the tag block, find the body,
    │                               verify the checksum, split the fields
    ▼  handle            nmea.go  ◄── the heart of the module
    │
    ├── registry.Decode  registry.go  look up the decoder for this formatter
    ├── applyToFix       fix.go       fold the value into the merged Fix
    ├── detector.Observe value/       infer the receiver's NMEA revision
    ├── fireEvent        nmea.go      OnFix, OnEvent, On, OnValue handlers
    ├── record           nmea.go      the history ring buffer
    └── dispatch         nmea.go      offer to the Events channel
```

`handle` is where the judgement calls live, and each one is deliberate:

- **A bad checksum does not discard the payload.** Plenty of receivers get the
  checksum wrong while the data is fine, so the payload still updates the fix
  and `Event.Err` carries the warning. `WithStrictChecksum` opts out, and
  `WithRequireChecksum` is the stricter gate for untrusted input.
- **An unregistered formatter is not an error.** It is counted in
  `Stats.Unknown` and reported once, then the stream continues. Proprietary
  sentences are normal traffic, not failures.
- **The version detector runs before the handlers**, so a handler can ask what
  the receiver has revealed so far and get a meaningful answer on the first
  sentence.

## Three decisions worth knowing about

**Blank is not zero.** Every optional value has a `Valid`, `Available`, or
`Has*` companion, because a blank NMEA field means "not reported" and that is
not the same as zero — latitude `0.0` and longitude `0.0` are real positions in
the Gulf of Guinea. No sentinel float is used anywhere.

**Handlers run on the read-loop goroutine.** A slow handler delays reading. The
alternative, a dispatch goroutine per handler, costs a queue and a reordering
that is harder to reason about than the documented constraint. Move slow work
to your own goroutine; the `Fix` is a snapshot, so it is safe to hold.

**Cancellation is cooperative.** `Consume` returns when the current `Read`
returns. A `Read` blocked on an idle serial port or socket is not woken by
cancelling the context, because the parser does not own the reader. Close the
device to interrupt it.

## Concurrency

| Safe to call from any goroutine | Must be configured before the stream starts |
|---|---|
| `Fix`, `Position`, `Stats`, `History` | `OnFix`, `OnEvent`, `OnError`, `On`, `OnValue`, `Use` |
| `Version`, `Receiver`, `Detector`, `Registry` | |

The second column is not a limitation so much as a statement of ownership: the
handler slices and the registry are read on the read loop without a lock, so
writing to them from another goroutine mid-stream is a data race. Set handlers
up, then start reading.

`device.Device`, `value.VersionDetector` and `nmea.LineSplitter` each carry
their own mutex and are safe for concurrent use in their own right.

## Where to add things

**A new sentence.** One file in `sentences/`: implement `Formatter()` and
`Decode()`, then add it to the `all` slice in `sentences/sentences.go`, which
is the single list both `Install` and the per-file `init` read. No change to
the parser, and no new import for the consumer.

**A new field parser.** Add it to `value`, so it is available to every decoder
without any of them importing anything new.

**A new fault.** Add it to `fault`, and alias it in `errors.go`. A decoder that
reports it then imports only `value` and `fault`, never the root package.

## Performance notes

The read loop is the one place that must never fall behind, so a few things are
measured rather than assumed. Current figures on a mid-range laptop, over the
bundled 3.6 kB sample capture:

| Benchmark | Before | After | Change |
|---|---|---|---|
| `Consume` (sustained stream) | 41.97 MB/s | 50.66 MB/s | **+21 %** |
| `LineSplitter.Append` | 556.98 MB/s, 819 allocs | 1016.40 MB/s, 709 allocs | **+82 %** |
| `indexTerminator` | 31.6 ns | 6.1 ns | **5.1×** |
| `record` at history 1000 | 5152 ns | 12.1 ns | **427×** |
| `FrameWith` | 148 ns, 3 allocs | 62 ns, 2 allocs | **2.4×** |

Three of those are worth explaining, because two contradict the obvious guess:

- **`indexTerminator` uses two `bytes.IndexByte` calls, not a hand-rolled
  loop.** The loop was there on the reasoning that short buffers favour simple
  code. Measurement said otherwise: the stdlib scans with SIMD, and a sentence
  is short enough that the loop's per-byte overhead dominates. `bytes.IndexAny`
  is slower still for a two-character set.
- **`isBlank` still uses `strings.TrimSpace`.** The obvious optimisation is a
  hand-rolled ASCII scan, and it measured *slower* — 27 ns against 23 ns —
  because `TrimSpace` checks only the two ends, which is O(1) for a field with
  no leading or trailing space. The loop was O(n). Kept as it was.
- **`record` is a ring buffer with a head index.** Shifting the slice to drop
  the oldest event is O(n) per sentence, which at a history of a thousand and a
  10 Hz receiver is on the order of a hundred kilobytes of copying a second.
  `History` unwraps the ring so callers still get chronological order.

The `LineSplitter` buffer is compacted back to offset zero after every read,
which is where most of that 82 % comes from. Advancing the slice instead
shrinks its capacity by every line consumed, so `append` had to grow and copy
every few sentences; keeping the start at zero means the array is reused. An
earlier attempt at this used `append(s[:0], s...)`, which copies in place and
therefore does **not** recover the capacity — the buffer kept sliding. It
needed an explicit `start` index.
