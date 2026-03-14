package parser

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// ---------------------------------------------------------------------------
// Sprint 9: Import, Include & Schema Composition
// ---------------------------------------------------------------------------

func newTestParserWithResolver() *Parser {
	return New(
		WithLogger(testLogger()),
		WithResolver(NewFileResolver()),
	)
}

// ---------------------------------------------------------------------------
// Simple import: main.xsd imports types.xsd
// ---------------------------------------------------------------------------

func TestSimpleImport(t *testing.T) {
	p := newTestParserWithResolver()
	ss, err := p.Parse("../testdata/imports/simple/main.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should have 2 schemas: imported types + main.
	// (imports are followed during parsing, so imported schema finishes first)
	if len(ss.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(ss.Schemas))
	}

	// Verify both namespaces are present.
	namespaces := map[string]*xsd.Schema{}
	for _, s := range ss.Schemas {
		namespaces[s.TargetNamespace] = s
	}
	if _, ok := namespaces["http://example.com/main"]; !ok {
		t.Error("missing main schema")
	}
	if _, ok := namespaces["http://example.com/types"]; !ok {
		t.Error("missing types schema")
	}

	// Types from the imported schema should be in the SchemaSet index.
	nameType := ss.LookupType(xsd.NewQName("http://example.com/types", "NameType"))
	if nameType == nil {
		t.Error("LookupType: expected to find NameType")
	}

	addrType := ss.LookupType(xsd.NewQName("http://example.com/types", "AddressType"))
	if addrType == nil {
		t.Error("LookupType: expected to find AddressType")
	}
}

// TestSimpleImportCrossRef verifies that type references across schemas are resolved.
func TestSimpleImportCrossRef(t *testing.T) {
	p := newTestParserWithResolver()
	ss, err := p.Parse("../testdata/imports/simple/main.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Find the main schema by namespace.
	mainSchema := ss.SchemaByNamespace("http://example.com/main")
	if mainSchema == nil {
		t.Fatal("main schema not found")
	}
	if len(mainSchema.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(mainSchema.Elements))
	}

	person := mainSchema.Elements[0]
	if person.Name != "person" {
		t.Errorf("expected 'person', got %q", person.Name)
	}

	ct, ok := person.InlineType.(*xsd.ComplexType)
	if !ok {
		t.Fatalf("expected ComplexType, got %T", person.InlineType)
	}

	seq := ct.Content.(*xsd.Sequence)
	if len(seq.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(seq.Items))
	}

	// "name" element references t:NameType.
	nameElem := seq.Items[0].(*xsd.Element)
	if nameElem.Type.Name.Local != "NameType" {
		t.Errorf("expected type NameType, got %s", nameElem.Type.Name.Local)
	}
	if nameElem.Type.Name.Namespace != "http://example.com/types" {
		t.Errorf("expected namespace http://example.com/types, got %s", nameElem.Type.Name.Namespace)
	}
	// Cross-schema ref should be resolved.
	if nameElem.Type.Resolved == nil {
		t.Error("NameType ref should be resolved")
	}

	// "address" element references t:AddressType.
	addrElem := seq.Items[1].(*xsd.Element)
	if addrElem.Type.Name.Local != "AddressType" {
		t.Errorf("expected type AddressType, got %s", addrElem.Type.Name.Local)
	}
	if addrElem.Type.Resolved == nil {
		t.Error("AddressType ref should be resolved")
	}
}

// ---------------------------------------------------------------------------
// Circular import: a.xsd imports b.xsd, b.xsd imports a.xsd
// ---------------------------------------------------------------------------

func TestCircularImport(t *testing.T) {
	p := newTestParserWithResolver()
	ss, err := p.Parse("../testdata/imports/circular/a.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should have 2 schemas despite circular reference.
	if len(ss.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(ss.Schemas))
	}

	// Both schemas should be present.
	namespaces := map[string]bool{}
	for _, s := range ss.Schemas {
		namespaces[s.TargetNamespace] = true
	}
	if !namespaces["http://example.com/a"] {
		t.Error("missing schema for namespace http://example.com/a")
	}
	if !namespaces["http://example.com/b"] {
		t.Error("missing schema for namespace http://example.com/b")
	}

	// Elements from both schemas should be accessible.
	fromA := ss.LookupElement(xsd.NewQName("http://example.com/a", "fromA"))
	if fromA == nil {
		t.Error("expected to find element 'fromA'")
	}
	fromB := ss.LookupElement(xsd.NewQName("http://example.com/b", "fromB"))
	if fromB == nil {
		t.Error("expected to find element 'fromB'")
	}
}

// ---------------------------------------------------------------------------
// Diamond import: top imports left+right, both import base
// ---------------------------------------------------------------------------

func TestDiamondImport(t *testing.T) {
	p := newTestParserWithResolver()
	ss, err := p.Parse("../testdata/imports/diamond/top.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should have 4 schemas: top, left, base, right.
	// (base is imported by both left and right, but parsed only once)
	if len(ss.Schemas) != 4 {
		t.Fatalf("expected 4 schemas, got %d", len(ss.Schemas))
	}

	// All four namespaces should be present.
	namespaces := map[string]bool{}
	for _, s := range ss.Schemas {
		namespaces[s.TargetNamespace] = true
	}
	for _, ns := range []string{
		"http://example.com/top",
		"http://example.com/left",
		"http://example.com/right",
		"http://example.com/base",
	} {
		if !namespaces[ns] {
			t.Errorf("missing schema for namespace %s", ns)
		}
	}

	// Base type should be found exactly once.
	idType := ss.LookupType(xsd.NewQName("http://example.com/base", "IDType"))
	if idType == nil {
		t.Error("expected to find IDType from base schema")
	}
}

// ---------------------------------------------------------------------------
// Chameleon include: included schema adopts including schema's namespace
// ---------------------------------------------------------------------------

func TestChameleonInclude(t *testing.T) {
	p := newTestParserWithResolver()
	ss, err := p.Parse("../testdata/imports/chameleon/main.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should have 2 schemas: main + included.
	if len(ss.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(ss.Schemas))
	}

	// The included schema should have adopted the main namespace.
	includedSchema := ss.Schemas[1]
	if includedSchema.TargetNamespace != "http://example.com/contact" {
		t.Errorf("chameleon schema: expected namespace http://example.com/contact, got %s",
			includedSchema.TargetNamespace)
	}
}

// ---------------------------------------------------------------------------
// FileResolver tests
// ---------------------------------------------------------------------------

func TestFileResolverRelative(t *testing.T) {
	r := NewFileResolver()
	data, err := r.Resolve("simple/types.xsd", "../testdata/imports/dummy.xsd", "")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
	if !strings.Contains(string(data), "NameType") {
		t.Error("resolved file should contain NameType")
	}
}

func TestFileResolverNotFound(t *testing.T) {
	r := NewFileResolver()
	_, err := r.Resolve("nonexistent.xsd", "/tmp/base.xsd", "")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileResolverEmptyLocation(t *testing.T) {
	r := NewFileResolver()
	_, err := r.Resolve("", "/tmp/base.xsd", "")
	if err == nil {
		t.Error("expected error for empty location")
	}
}

// ---------------------------------------------------------------------------
// MultiResolver tests
// ---------------------------------------------------------------------------

func TestMultiResolverFirstWins(t *testing.T) {
	failing := ResolverFunc(func(_, _, _ string) ([]byte, error) {
		return nil, fmt.Errorf("fail")
	})
	succeeding := ResolverFunc(func(_, _, _ string) ([]byte, error) {
		return []byte("ok"), nil
	})

	r := NewMultiResolver(failing, succeeding)
	data, err := r.Resolve("test.xsd", "", "")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("expected 'ok', got %q", string(data))
	}
}

func TestMultiResolverAllFail(t *testing.T) {
	failing := ResolverFunc(func(_, _, _ string) ([]byte, error) {
		return nil, fmt.Errorf("fail")
	})

	r := NewMultiResolver(failing)
	_, err := r.Resolve("test.xsd", "", "")
	if err == nil {
		t.Error("expected error when all resolvers fail")
	}
}

// ---------------------------------------------------------------------------
// No resolver: imports/includes are just recorded, not followed
// ---------------------------------------------------------------------------

func TestImportWithoutResolver(t *testing.T) {
	p := newTestParser() // no resolver
	ss, err := p.Parse("../testdata/imports/simple/main.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Only 1 schema (imports not followed without resolver).
	if len(ss.Schemas) != 1 {
		t.Fatalf("expected 1 schema (no resolver), got %d", len(ss.Schemas))
	}

	// Import should still be recorded.
	if len(ss.Schemas[0].Imports) != 1 {
		t.Fatalf("expected 1 import recorded, got %d", len(ss.Schemas[0].Imports))
	}
}

// ---------------------------------------------------------------------------
// Parse multiple files directly
// ---------------------------------------------------------------------------

func TestParseMultipleFiles(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse(
		"../testdata/imports/simple/types.xsd",
		"../testdata/imports/simple/main.xsd",
	)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(ss.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(ss.Schemas))
	}

	// Types from types.xsd should be resolvable.
	nameType := ss.LookupType(xsd.NewQName("http://example.com/types", "NameType"))
	if nameType == nil {
		t.Error("expected to find NameType")
	}
}

// ---------------------------------------------------------------------------
// resolveLocation helper
// ---------------------------------------------------------------------------

func TestResolveLocation(t *testing.T) {
	tests := []struct {
		location string
		baseURI  string
		want     string
	}{
		{"types.xsd", "/path/to/main.xsd", "/path/to/types.xsd"},
		{"/absolute/types.xsd", "/path/to/main.xsd", "/absolute/types.xsd"},
		{"types.xsd", "", "types.xsd"},
		{"", "/path/to/main.xsd", "/path/to/main.xsd"},
		{"sub/types.xsd", "/path/to/main.xsd", "/path/to/sub/types.xsd"},
	}

	for _, tc := range tests {
		got := resolveLocation(tc.location, tc.baseURI)
		if got != tc.want {
			t.Errorf("resolveLocation(%q, %q) = %q, want %q",
				tc.location, tc.baseURI, got, tc.want)
		}
	}
}
