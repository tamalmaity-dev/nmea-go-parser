package wire

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// sampleLines returns the recorded sentences from testdata/sample.nmea,
// skipping blanks and comments.
//
// The same file is used by the sentences package tests, so a decoder and the
// framing code are always checked against one shared corpus rather than two
// sets of literals that can drift apart.
func sampleLines(t *testing.T) []string {
	t.Helper()

	// The corpus lives at the module root, one level up from here.
	f, err := os.Open("../testdata/sample.nmea")
	if err != nil {
		t.Fatalf("opening sample data: %v", err)
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// A file written by a text editor can begin with a UTF-8 byte order
		// mark, and a serial link can prepend one at a power cycle. Neither
		// is part of the sentence.
		line := strings.TrimPrefix(strings.TrimSpace(sc.Text()), "\uFEFF")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading sample data: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("sample data is empty")
	}
	return out
}
