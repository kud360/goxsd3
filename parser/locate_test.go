package parser

import (
	"io"
	"strings"
	"testing"
)

func TestLocatingReader_SingleLine(t *testing.T) {
	r := NewLocatingReader(strings.NewReader("hello"))
	buf := make([]byte, 64)
	n, _ := r.Read(buf)
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	loc := r.Location(0, "test.xsd")
	if loc.Line != 1 || loc.Col != 1 {
		t.Errorf("offset 0: expected 1:1, got %d:%d", loc.Line, loc.Col)
	}
	loc = r.Location(4, "test.xsd")
	if loc.Line != 1 || loc.Col != 5 {
		t.Errorf("offset 4: expected 1:5, got %d:%d", loc.Line, loc.Col)
	}
}

func TestLocatingReader_MultiLine(t *testing.T) {
	// "ab\ncd\nef" — 3 lines
	input := "ab\ncd\nef"
	r := NewLocatingReader(strings.NewReader(input))
	buf := make([]byte, 64)
	io.ReadAll(r) // drain through the locating reader
	_ = buf

	tests := []struct {
		offset int64
		line   int
		col    int
	}{
		{0, 1, 1}, // 'a'
		{1, 1, 2}, // 'b'
		{2, 1, 3}, // '\n'
		{3, 2, 1}, // 'c'
		{4, 2, 2}, // 'd'
		{5, 2, 3}, // '\n'
		{6, 3, 1}, // 'e'
		{7, 3, 2}, // 'f'
	}
	for _, tc := range tests {
		loc := r.Location(tc.offset, "test.xsd")
		if loc.Line != tc.line || loc.Col != tc.col {
			t.Errorf("offset %d: expected %d:%d, got %d:%d", tc.offset, tc.line, tc.col, loc.Line, loc.Col)
		}
	}
}

func TestLocatingReader_SmallReads(t *testing.T) {
	// Force multiple small reads to verify line tracking works across Read calls.
	input := "ab\ncd\nef"
	r := NewLocatingReader(strings.NewReader(input))

	buf := make([]byte, 2)
	for {
		_, err := r.Read(buf)
		if err != nil {
			break
		}
	}

	loc := r.Location(6, "test.xsd")
	if loc.Line != 3 || loc.Col != 1 {
		t.Errorf("offset 6 after small reads: expected 3:1, got %d:%d", loc.Line, loc.Col)
	}
}

func TestLocatingReader_SystemID(t *testing.T) {
	r := NewLocatingReader(strings.NewReader("x"))
	io.ReadAll(r)

	loc := r.Location(0, "my/schema.xsd")
	if loc.SystemID != "my/schema.xsd" {
		t.Errorf("expected systemID 'my/schema.xsd', got %q", loc.SystemID)
	}
}

func TestLocatingReader_EmptyInput(t *testing.T) {
	r := NewLocatingReader(strings.NewReader(""))
	io.ReadAll(r)

	loc := r.Location(0, "empty.xsd")
	if loc.Line != 1 || loc.Col != 1 {
		t.Errorf("offset 0 on empty: expected 1:1, got %d:%d", loc.Line, loc.Col)
	}
}

func TestLocatingReader_MultiByte(t *testing.T) {
	// UTF-8 multi-byte: "é\nà" → é is 2 bytes
	input := "é\nà"
	r := NewLocatingReader(strings.NewReader(input))
	io.ReadAll(r)

	// 'é' = bytes 0,1; '\n' = byte 2; 'à' = bytes 3,4
	loc := r.Location(3, "utf8.xsd")
	if loc.Line != 2 || loc.Col != 1 {
		t.Errorf("offset 3 (after multi-byte): expected 2:1, got %d:%d", loc.Line, loc.Col)
	}
}
