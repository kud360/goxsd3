package parser

import (
	"io"
	"sort"

	"github.com/kud360/goxsd3/xsd"
)

// LocatingReader wraps an io.Reader and tracks line positions so that
// byte offsets (from xml.Decoder.InputOffset) can be mapped to line:col
// locations. It records the byte offset of each newline as bytes flow
// through Read.
type LocatingReader struct {
	inner     io.Reader
	lines     []int64 // byte offset of each line start; lines[0] = 0
	totalRead int64
}

// NewLocatingReader creates a LocatingReader wrapping r. Line 1 starts
// at byte offset 0.
func NewLocatingReader(r io.Reader) *LocatingReader {
	return &LocatingReader{
		inner: r,
		lines: []int64{0}, // line 1 starts at offset 0
	}
}

// Read implements io.Reader. As bytes flow through, it records the
// positions of newline characters so that Location can later map byte
// offsets to line:col pairs.
func (lr *LocatingReader) Read(p []byte) (int, error) {
	n, err := lr.inner.Read(p)
	for i := 0; i < n; i++ {
		if p[i] == '\n' {
			lr.lines = append(lr.lines, lr.totalRead+int64(i)+1)
		}
	}
	lr.totalRead += int64(n)
	return n, err
}

// Location returns the xsd.Location for a given byte offset. It uses
// binary search over the recorded line starts to find the line number,
// then computes the column as the distance from the line start.
func (lr *LocatingReader) Location(offset int64, systemID string) xsd.Location {
	// Binary search: find the last line start <= offset.
	line := sort.Search(len(lr.lines), func(i int) bool {
		return lr.lines[i] > offset
	}) // returns the first line start > offset, so line-1 is our line index

	if line == 0 {
		line = 1
	}
	col := int(offset-lr.lines[line-1]) + 1

	return xsd.Location{
		SystemID: systemID,
		Line:     line,
		Col:      col,
		Offset:   offset,
	}
}
