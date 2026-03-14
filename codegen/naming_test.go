package codegen

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/kud360/goxsd3/parser"
	"github.com/kud360/goxsd3/xsd"
)

func newTestParser() *parser.Parser {
	return parser.New(parser.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))))
}

func newTestNamer() *Namer {
	return NewNamer(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
}

// parseAndName is a test helper that parses an XSD and assigns names.
func parseAndName(t *testing.T, file string) *NameMap {
	t.Helper()
	p := newTestParser()
	ss, err := p.Parse(file)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	namer := newTestNamer()
	nm, err := namer.AssignNames(ss)
	if err != nil {
		t.Fatalf("naming failed: %v", err)
	}
	return nm
}

func parseReaderAndName(t *testing.T, input, systemID string) (*xsd.SchemaSet, *NameMap) {
	t.Helper()
	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), systemID)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	namer := newTestNamer()
	nm, err := namer.AssignNames(ss)
	if err != nil {
		t.Fatalf("naming failed: %v", err)
	}
	return ss, nm
}

// ---------------------------------------------------------------------------
// Named types keep their XSD names
// ---------------------------------------------------------------------------

func TestNamedTypeUnchanged(t *testing.T) {
	nm := parseAndName(t, "../testdata/naming/named_types.xsd")

	pairs := nm.AllTypeNames()
	if len(pairs) != 2 {
		t.Fatalf("expected 2 types, got %d", len(pairs))
	}

	// ZipCode (SimpleType) keeps its name.
	if pairs[0].GoName != "ZipCode" {
		t.Errorf("expected ZipCode, got %s", pairs[0].GoName)
	}
	// AddressType (ComplexType) keeps its name.
	if pairs[1].GoName != "AddressType" {
		t.Errorf("expected AddressType, got %s", pairs[1].GoName)
	}
}

// ---------------------------------------------------------------------------
// Simple anonymous type naming: element → ElementType
// ---------------------------------------------------------------------------

func TestSimpleAnonymousTypeName(t *testing.T) {
	nm := parseAndName(t, "../testdata/naming/anonymous_simple.xsd")

	pairs := nm.AllTypeNames()
	if len(pairs) != 1 {
		t.Fatalf("expected 1 type, got %d", len(pairs))
	}

	// <xs:element name="person"><xs:complexType>... → "PersonType"
	if pairs[0].GoName != "PersonType" {
		t.Errorf("expected PersonType, got %s", pairs[0].GoName)
	}
}

// ---------------------------------------------------------------------------
// Nested anonymous types: parent.child → ParentChildType
// ---------------------------------------------------------------------------

func TestNestedAnonymousTypeName(t *testing.T) {
	nm := parseAndName(t, "../testdata/naming/anonymous_nested.xsd")

	pairs := nm.AllTypeNames()
	if len(pairs) != 3 {
		t.Fatalf("expected 3 types, got %d", len(pairs))
	}

	expected := []string{"OrderType", "OrderItemType", "OrderItemDetailsType"}
	for i, tc := range expected {
		if pairs[i].GoName != tc {
			t.Errorf("type %d: expected %s, got %s", i, tc, pairs[i].GoName)
		}
	}
}

// ---------------------------------------------------------------------------
// Deeply nested (4+ levels)
// ---------------------------------------------------------------------------

func TestDeeplyNestedAnonymousTypeName(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="level1">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="level2">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="level3">
                      <xs:complexType>
                        <xs:sequence>
                          <xs:element name="value" type="xs:string"/>
                        </xs:sequence>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	_, nm := parseReaderAndName(t, input, "deep.xsd")
	pairs := nm.AllTypeNames()
	if len(pairs) != 4 {
		t.Fatalf("expected 4 types, got %d", len(pairs))
	}

	expected := []string{
		"RootType",
		"RootLevel1Type",
		"RootLevel1Level2Type",
		"RootLevel1Level2Level3Type",
	}
	for i, tc := range expected {
		if pairs[i].GoName != tc {
			t.Errorf("type %d: expected %s, got %s", i, tc, pairs[i].GoName)
		}
	}
}

// ---------------------------------------------------------------------------
// Conflict resolution: same base name from different parents
// ---------------------------------------------------------------------------

func TestConflictResolutionQualify(t *testing.T) {
	nm := parseAndName(t, "../testdata/naming/conflict.xsd")

	pairs := nm.AllTypeNames()
	if len(pairs) != 3 {
		t.Fatalf("expected 3 types, got %d", len(pairs))
	}

	// First "item" element → "ItemType" (first occurrence wins).
	if pairs[0].GoName != "ItemType" {
		t.Errorf("type 0: expected ItemType, got %s", pairs[0].GoName)
	}

	// "order" element → "OrderType".
	if pairs[1].GoName != "OrderType" {
		t.Errorf("type 1: expected OrderType, got %s", pairs[1].GoName)
	}

	// Second "item" inside "order" → conflict with "ItemType", resolved
	// by qualifying with parent: "OrderItemType".
	if pairs[2].GoName != "OrderItemType" {
		t.Errorf("type 2: expected OrderItemType, got %s", pairs[2].GoName)
	}
}

// ---------------------------------------------------------------------------
// Conflict resolution: numeric suffix as last resort
// ---------------------------------------------------------------------------

func TestConflictResolutionSuffix(t *testing.T) {
	// Create a scenario where parent qualification still collides.
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="ItemType">
    <xs:sequence>
      <xs:element name="x" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:element name="item">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="y" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	_, nm := parseReaderAndName(t, input, "suffix.xsd")
	pairs := nm.AllTypeNames()

	// Named "ItemType" gets assigned first.
	if pairs[0].GoName != "ItemType" {
		t.Errorf("type 0: expected ItemType, got %s", pairs[0].GoName)
	}

	// Anonymous type from element "item" would be "ItemType" but it's taken.
	// Path is ["item"], so parent qualification produces just "ItemType" again.
	// Falls through to numeric suffix.
	if pairs[1].GoName != "ItemType2" {
		t.Errorf("type 1: expected ItemType2, got %s", pairs[1].GoName)
	}
}

// ---------------------------------------------------------------------------
// Anonymous types inside choice variants
// ---------------------------------------------------------------------------

func TestAnonymousTypeInChoice(t *testing.T) {
	nm := parseAndName(t, "../testdata/naming/choice_anonymous.xsd")

	pairs := nm.AllTypeNames()
	if len(pairs) != 3 {
		t.Fatalf("expected 3 types, got %d", len(pairs))
	}

	// ShapeType is named.
	if pairs[0].GoName != "ShapeType" {
		t.Errorf("type 0: expected ShapeType, got %s", pairs[0].GoName)
	}

	// circle's anonymous type → "CircleType" (derived from element path within ShapeType's content).
	if pairs[1].GoName != "CircleType" {
		t.Errorf("type 1: expected CircleType, got %s", pairs[1].GoName)
	}

	// square's anonymous type → "SquareType".
	if pairs[2].GoName != "SquareType" {
		t.Errorf("type 2: expected SquareType, got %s", pairs[2].GoName)
	}
}

// ---------------------------------------------------------------------------
// Determinism: parse same schema N times, verify identical names
// ---------------------------------------------------------------------------

func TestNamingDeterminism(t *testing.T) {
	const iterations = 50

	// Parse and name once to get the reference.
	ref := parseAndName(t, "../testdata/naming/conflict.xsd")
	refPairs := ref.AllTypeNames()

	for i := 0; i < iterations; i++ {
		nm := parseAndName(t, "../testdata/naming/conflict.xsd")
		pairs := nm.AllTypeNames()

		if len(pairs) != len(refPairs) {
			t.Fatalf("iteration %d: got %d types, expected %d", i, len(pairs), len(refPairs))
		}
		for j := range pairs {
			if pairs[j].GoName != refPairs[j].GoName {
				t.Errorf("iteration %d, type %d: got %s, expected %s",
					i, j, pairs[j].GoName, refPairs[j].GoName)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Element naming
// ---------------------------------------------------------------------------

func TestElementNaming(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="my-element" type="xs:string"/>
  <xs:element name="another_element" type="xs:integer"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "elem.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	namer := newTestNamer()
	nm, err := namer.AssignNames(ss)
	if err != nil {
		t.Fatalf("naming failed: %v", err)
	}

	// Hyphens and underscores are treated as word separators.
	if nm.ElementName(ss.Schemas[0].Elements[0]) != "MyElement" {
		t.Errorf("expected MyElement, got %s", nm.ElementName(ss.Schemas[0].Elements[0]))
	}
	if nm.ElementName(ss.Schemas[0].Elements[1]) != "AnotherElement" {
		t.Errorf("expected AnotherElement, got %s", nm.ElementName(ss.Schemas[0].Elements[1]))
	}
}

// ---------------------------------------------------------------------------
// exportedName unit tests
// ---------------------------------------------------------------------------

func TestExportedName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"person", "Person"},
		{"addressType", "AddressType"},
		{"my-element", "MyElement"},
		{"another_element", "AnotherElement"},
		{"XMLParser", "XMLParser"},
		{"", ""},
		{"a", "A"},
		{"dot.separated", "DotSeparated"},
	}

	for _, tc := range tests {
		got := exportedName(tc.input)
		if got != tc.expected {
			t.Errorf("exportedName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// deriveNameFromPath unit tests
// ---------------------------------------------------------------------------

func TestDeriveNameFromPath(t *testing.T) {
	tests := []struct {
		path     []string
		expected string
	}{
		{[]string{"person"}, "PersonType"},
		{[]string{"order", "item"}, "OrderItemType"},
		{[]string{"order", "item", "details"}, "OrderItemDetailsType"},
		{[]string{"my-elem"}, "MyElemType"},
	}

	for _, tc := range tests {
		got := deriveNameFromPath(tc.path)
		if got != tc.expected {
			t.Errorf("deriveNameFromPath(%v) = %q, want %q", tc.path, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Mixed named and anonymous types
// ---------------------------------------------------------------------------

func TestMixedNamedAndAnonymous(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="HeaderType">
    <xs:sequence>
      <xs:element name="title" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:element name="document">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="header" type="HeaderType"/>
        <xs:element name="body">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="content" type="xs:string"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	_, nm := parseReaderAndName(t, input, "mixed.xsd")
	pairs := nm.AllTypeNames()

	if len(pairs) != 3 {
		t.Fatalf("expected 3 types, got %d", len(pairs))
	}

	// Named type first (from schema.Types).
	if pairs[0].GoName != "HeaderType" {
		t.Errorf("type 0: expected HeaderType, got %s", pairs[0].GoName)
	}
	// Anonymous type from "document" element.
	if pairs[1].GoName != "DocumentType" {
		t.Errorf("type 1: expected DocumentType, got %s", pairs[1].GoName)
	}
	// Nested anonymous type from "body" inside "document".
	if pairs[2].GoName != "DocumentBodyType" {
		t.Errorf("type 2: expected DocumentBodyType, got %s", pairs[2].GoName)
	}
}

// ---------------------------------------------------------------------------
// Cross-namespace naming (same local name in different namespaces)
// ---------------------------------------------------------------------------

func TestCrossNamespaceNaming(t *testing.T) {
	// Simulate two schemas with same type name by parsing twice.
	input1 := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/ns1">
  <xs:complexType name="AddressType">
    <xs:sequence>
      <xs:element name="street" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	input2 := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/ns2">
  <xs:complexType name="AddressType">
    <xs:sequence>
      <xs:element name="line1" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss1, err := p.ParseReader(strings.NewReader(input1), "ns1.xsd")
	if err != nil {
		t.Fatalf("parse ns1 failed: %v", err)
	}

	// Parse second schema into a new parser since ParseReader finalizes.
	p2 := newTestParser()
	ss2, err := p2.ParseReader(strings.NewReader(input2), "ns2.xsd")
	if err != nil {
		t.Fatalf("parse ns2 failed: %v", err)
	}

	// Combine into one SchemaSet.
	combined := xsd.NewSchemaSet()
	for _, s := range ss1.Schemas {
		combined.AddSchema(s)
	}
	for _, s := range ss2.Schemas {
		combined.AddSchema(s)
	}

	namer := newTestNamer()
	nm, err := namer.AssignNames(combined)
	if err != nil {
		t.Fatalf("naming failed: %v", err)
	}

	pairs := nm.AllTypeNames()
	if len(pairs) != 2 {
		t.Fatalf("expected 2 types, got %d", len(pairs))
	}

	// First AddressType gets the name.
	if pairs[0].GoName != "AddressType" {
		t.Errorf("type 0: expected AddressType, got %s", pairs[0].GoName)
	}

	// Second AddressType gets numeric suffix (conflict resolution).
	if pairs[1].GoName != "AddressType2" {
		t.Errorf("type 1: expected AddressType2, got %s", pairs[1].GoName)
	}
}

// ---------------------------------------------------------------------------
// Empty schema — no types or elements
// ---------------------------------------------------------------------------

func TestEmptySchema(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
</xs:schema>`

	_, nm := parseReaderAndName(t, input, "empty.xsd")
	pairs := nm.AllTypeNames()
	if len(pairs) != 0 {
		t.Errorf("expected 0 types, got %d", len(pairs))
	}
}

// ---------------------------------------------------------------------------
// Compositor in extension with anonymous type
// ---------------------------------------------------------------------------

func TestAnonymousTypeInExtension(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="id" type="xs:integer"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="BaseType">
        <xs:sequence>
          <xs:element name="extra">
            <xs:complexType>
              <xs:sequence>
                <xs:element name="detail" type="xs:string"/>
              </xs:sequence>
            </xs:complexType>
          </xs:element>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	_, nm := parseReaderAndName(t, input, "ext.xsd")
	pairs := nm.AllTypeNames()
	if len(pairs) != 3 {
		t.Fatalf("expected 3 types, got %d", len(pairs))
	}

	if pairs[0].GoName != "BaseType" {
		t.Errorf("type 0: expected BaseType, got %s", pairs[0].GoName)
	}
	if pairs[1].GoName != "DerivedType" {
		t.Errorf("type 1: expected DerivedType, got %s", pairs[1].GoName)
	}
	// Anonymous type from "extra" element inside extension.
	if pairs[2].GoName != "ExtraType" {
		t.Errorf("type 2: expected ExtraType, got %s", pairs[2].GoName)
	}
}
