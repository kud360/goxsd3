package parser

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

func newTestParser() *Parser {
	return New(WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))))
}

// ---------------------------------------------------------------------------
// Simple element parsing
// ---------------------------------------------------------------------------

func TestParseSimpleElements(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/basic/simple_element.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if schema.TargetNamespace != "http://example.com/test" {
		t.Errorf("unexpected namespace: %s", schema.TargetNamespace)
	}

	if len(schema.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(schema.Elements))
	}

	tests := []struct {
		name     string
		typeName string
	}{
		{"firstName", "string"},
		{"age", "integer"},
		{"active", "boolean"},
	}

	for i, tc := range tests {
		elem := schema.Elements[i]
		if elem.Name != tc.name {
			t.Errorf("element %d: expected name %q, got %q", i, tc.name, elem.Name)
		}
		if elem.Type.Name.Local != tc.typeName {
			t.Errorf("element %q: expected type %q, got %q", tc.name, tc.typeName, elem.Type.Name.Local)
		}
		if elem.Type.Name.Namespace != xsd.XSDNS {
			t.Errorf("element %q: expected XSD namespace, got %q", tc.name, elem.Type.Name.Namespace)
		}
		if elem.Namespace != "http://example.com/test" {
			t.Errorf("element %q: expected namespace http://example.com/test, got %q", tc.name, elem.Namespace)
		}
	}
}

// ---------------------------------------------------------------------------
// Simple type parsing (restriction with enumeration, pattern, maxLength)
// ---------------------------------------------------------------------------

func TestParseSimpleType(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/basic/simple_type.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(schema.Types))
	}

	// ColorType
	colorType := schema.Types[0].(*xsd.SimpleType)
	if colorType.Name.Local != "ColorType" {
		t.Errorf("expected ColorType, got %s", colorType.Name.Local)
	}
	if colorType.Restriction == nil {
		t.Fatal("ColorType: expected restriction")
	}
	if colorType.Restriction.Base.Name.Local != "string" {
		t.Errorf("ColorType: expected base string, got %s", colorType.Restriction.Base.Name.Local)
	}
	if len(colorType.Restriction.Facets) != 3 {
		t.Fatalf("ColorType: expected 3 facets, got %d", len(colorType.Restriction.Facets))
	}
	for i, expected := range []string{"red", "green", "blue"} {
		f := colorType.Restriction.Facets[i]
		if f.Kind != xsd.FacetEnumeration {
			t.Errorf("facet %d: expected enumeration, got %s", i, f.Kind)
		}
		if f.Value != expected {
			t.Errorf("facet %d: expected %q, got %q", i, expected, f.Value)
		}
	}

	// ZipCode
	zipType := schema.Types[1].(*xsd.SimpleType)
	if zipType.Name.Local != "ZipCode" {
		t.Errorf("expected ZipCode, got %s", zipType.Name.Local)
	}
	if len(zipType.Restriction.Facets) != 2 {
		t.Fatalf("ZipCode: expected 2 facets, got %d", len(zipType.Restriction.Facets))
	}
	if zipType.Restriction.Facets[0].Kind != xsd.FacetPattern {
		t.Errorf("ZipCode facet 0: expected pattern, got %s", zipType.Restriction.Facets[0].Kind)
	}
	if zipType.Restriction.Facets[1].Kind != xsd.FacetMaxLength {
		t.Errorf("ZipCode facet 1: expected maxLength, got %s", zipType.Restriction.Facets[1].Kind)
	}
	if zipType.Restriction.Facets[1].Value != "10" {
		t.Errorf("ZipCode maxLength: expected 10, got %s", zipType.Restriction.Facets[1].Value)
	}
}

// ---------------------------------------------------------------------------
// Complex type with sequence
// ---------------------------------------------------------------------------

func TestParseComplexType(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/basic/complex_type.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "AddressType" {
		t.Errorf("expected AddressType, got %s", ct.Name.Local)
	}

	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence content, got %T", ct.Content)
	}
	if len(seq.Items) != 4 {
		t.Fatalf("expected 4 elements in sequence, got %d", len(seq.Items))
	}

	// Check minOccurs on 'country' element.
	country := seq.Items[3].(*xsd.Element)
	if country.Name != "country" {
		t.Errorf("expected 'country', got %q", country.Name)
	}
	if country.MinOccurs != 0 {
		t.Errorf("country: expected minOccurs=0, got %d", country.MinOccurs)
	}

	// Check top-level element references the complex type.
	if len(schema.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(schema.Elements))
	}
	if schema.Elements[0].Name != "address" {
		t.Errorf("expected element 'address', got %q", schema.Elements[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Attributes
// ---------------------------------------------------------------------------

func TestParseAttributes(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/basic/attributes.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if len(ct.Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(ct.Attributes))
	}

	// id attribute — required
	idAttr := ct.Attributes[0]
	if idAttr.Name != "id" {
		t.Errorf("expected attribute 'id', got %q", idAttr.Name)
	}
	if idAttr.Use != xsd.AttributeRequired {
		t.Errorf("id: expected use=required, got %s", idAttr.Use)
	}

	// currency attribute — optional with default
	currAttr := ct.Attributes[1]
	if currAttr.Name != "currency" {
		t.Errorf("expected attribute 'currency', got %q", currAttr.Name)
	}
	if currAttr.Default == nil || *currAttr.Default != "USD" {
		t.Errorf("currency: expected default 'USD'")
	}
}

// ---------------------------------------------------------------------------
// Location tracking
// ---------------------------------------------------------------------------

func TestParseLocationAccuracy(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/parser/location_test.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(schema.Elements))
	}

	elem := schema.Elements[0]
	if elem.Location.SystemID != "../testdata/parser/location_test.xsd" {
		t.Errorf("unexpected systemID: %s", elem.Location.SystemID)
	}
	// The element is on line 4 of the test file.
	if elem.Location.Line != 4 {
		t.Errorf("expected element on line 4, got line %d", elem.Location.Line)
	}
	if elem.Location.Line < 1 {
		t.Error("line number should be >= 1")
	}
	if elem.Location.Col < 1 {
		t.Error("column should be >= 1")
	}
}

// ---------------------------------------------------------------------------
// ParseReader
// ---------------------------------------------------------------------------

func TestParseReader(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/inline">
  <xs:element name="test" type="xs:string"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "inline.xsd")
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	if len(ss.Schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(ss.Schemas))
	}
	if ss.Schemas[0].TargetNamespace != "http://example.com/inline" {
		t.Errorf("unexpected namespace: %s", ss.Schemas[0].TargetNamespace)
	}
	if len(ss.Schemas[0].Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(ss.Schemas[0].Elements))
	}
}

// ---------------------------------------------------------------------------
// Inline / anonymous types
// ---------------------------------------------------------------------------

func TestParseInlineType(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="person">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="age" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "inline.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(schema.Elements))
	}

	elem := schema.Elements[0]
	if elem.Name != "person" {
		t.Errorf("expected 'person', got %q", elem.Name)
	}
	if elem.InlineType == nil {
		t.Fatal("expected inline type")
	}

	ct, ok := elem.InlineType.(*xsd.ComplexType)
	if !ok {
		t.Fatalf("expected ComplexType, got %T", elem.InlineType)
	}

	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence, got %T", ct.Content)
	}
	if len(seq.Items) != 2 {
		t.Errorf("expected 2 items in sequence, got %d", len(seq.Items))
	}
}

// ---------------------------------------------------------------------------
// Import / Include recording (not followed in Sprint 3)
// ---------------------------------------------------------------------------

func TestParseImportIncludeRecording(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:addr="http://example.com/address"
           targetNamespace="http://example.com/order">
  <xs:import namespace="http://example.com/address" schemaLocation="address.xsd"/>
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="order" type="xs:string"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "order.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	if len(schema.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(schema.Imports))
	}
	imp := schema.Imports[0]
	if imp.Namespace != "http://example.com/address" {
		t.Errorf("import namespace: expected http://example.com/address, got %s", imp.Namespace)
	}
	if imp.SchemaLocation != "address.xsd" {
		t.Errorf("import location: expected address.xsd, got %s", imp.SchemaLocation)
	}

	if len(schema.Includes) != 1 {
		t.Fatalf("expected 1 include, got %d", len(schema.Includes))
	}
	inc := schema.Includes[0]
	if inc.SchemaLocation != "types.xsd" {
		t.Errorf("include location: expected types.xsd, got %s", inc.SchemaLocation)
	}
}

// ---------------------------------------------------------------------------
// Namespace prefix resolution
// ---------------------------------------------------------------------------

func TestParseNamespaceResolution(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test">
  <xs:element name="item" type="xs:string"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ns.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if schema.Namespaces["xs"] != xsd.XSDNS {
		t.Errorf("xs prefix not mapped correctly")
	}
	if schema.Namespaces["tns"] != "http://example.com/test" {
		t.Errorf("tns prefix not mapped correctly")
	}
}

// ---------------------------------------------------------------------------
// ComplexContent with extension
// ---------------------------------------------------------------------------

func TestParseComplexContentExtension(t *testing.T) {
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
          <xs:element name="extra" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ext.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(schema.Types))
	}

	derived := schema.Types[1].(*xsd.ComplexType)
	if derived.Name.Local != "DerivedType" {
		t.Errorf("expected DerivedType, got %s", derived.Name.Local)
	}

	cc, ok := derived.Content.(*xsd.ComplexContent)
	if !ok {
		t.Fatalf("expected ComplexContent, got %T", derived.Content)
	}
	if cc.Extension == nil {
		t.Fatal("expected extension in complex content")
	}
	if cc.Extension.Base.Name.Local != "BaseType" {
		t.Errorf("expected base BaseType, got %s", cc.Extension.Base.Name.Local)
	}
	if cc.Extension.Compositor == nil {
		t.Fatal("expected compositor in extension")
	}
	seq, ok := cc.Extension.Compositor.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence in extension, got %T", cc.Extension.Compositor)
	}
	if len(seq.Items) != 1 {
		t.Errorf("expected 1 item in extension sequence, got %d", len(seq.Items))
	}
}

// ---------------------------------------------------------------------------
// SimpleContent with restriction
// ---------------------------------------------------------------------------

func TestParseSimpleContentRestriction(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="PriceType">
    <xs:simpleContent>
      <xs:restriction base="xs:decimal">
        <xs:minInclusive value="0"/>
        <xs:maxInclusive value="999.99"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "sc.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	sc, ok := ct.Content.(*xsd.SimpleContent)
	if !ok {
		t.Fatalf("expected SimpleContent, got %T", ct.Content)
	}
	if sc.Restriction == nil {
		t.Fatal("expected restriction in simple content")
	}
	if sc.Restriction.Base.Name.Local != "decimal" {
		t.Errorf("expected base decimal, got %s", sc.Restriction.Base.Name.Local)
	}
	if len(sc.Restriction.Facets) != 2 {
		t.Fatalf("expected 2 facets, got %d", len(sc.Restriction.Facets))
	}
}

// ---------------------------------------------------------------------------
// Nested compositors
// ---------------------------------------------------------------------------

func TestParseNestedCompositors(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="NestedType">
    <xs:sequence>
      <xs:element name="first" type="xs:string"/>
      <xs:choice>
        <xs:element name="optA" type="xs:string"/>
        <xs:element name="optB" type="xs:integer"/>
      </xs:choice>
      <xs:element name="last" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "nested.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence, got %T", ct.Content)
	}
	if len(seq.Items) != 3 {
		t.Fatalf("expected 3 items in sequence, got %d", len(seq.Items))
	}

	// Second item should be a Choice.
	choice, ok := seq.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice as second item, got %T", seq.Items[1])
	}
	if len(choice.Items) != 2 {
		t.Errorf("expected 2 items in choice, got %d", len(choice.Items))
	}
}

// ---------------------------------------------------------------------------
// Annotation parsing
// ---------------------------------------------------------------------------

func TestParseAnnotation(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="item" type="xs:string">
    <xs:annotation>
      <xs:documentation>This is a test element.</xs:documentation>
    </xs:annotation>
  </xs:element>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ann.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	elem := ss.Schemas[0].Elements[0]
	if len(elem.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(elem.Annotations))
	}
	if len(elem.Annotations[0].Documentation) != 1 {
		t.Fatalf("expected 1 documentation, got %d", len(elem.Annotations[0].Documentation))
	}
	if elem.Annotations[0].Documentation[0] != "This is a test element." {
		t.Errorf("unexpected documentation: %q", elem.Annotations[0].Documentation[0])
	}
}

// ---------------------------------------------------------------------------
// Element attributes: nillable, abstract, default, fixed
// ---------------------------------------------------------------------------

func TestParseElementAttributes(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="nullable" type="xs:string" nillable="true"/>
  <xs:element name="abs" type="xs:string" abstract="true"/>
  <xs:element name="withDefault" type="xs:string" default="hello"/>
  <xs:element name="withFixed" type="xs:string" fixed="constant"/>
  <xs:element name="multi" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "attrs.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	elems := ss.Schemas[0].Elements
	if len(elems) != 5 {
		t.Fatalf("expected 5 elements, got %d", len(elems))
	}

	if !elems[0].Nillable {
		t.Error("nullable: expected nillable=true")
	}
	if !elems[1].Abstract {
		t.Error("abs: expected abstract=true")
	}
	if elems[2].Default == nil || *elems[2].Default != "hello" {
		t.Error("withDefault: expected default='hello'")
	}
	if elems[3].Fixed == nil || *elems[3].Fixed != "constant" {
		t.Error("withFixed: expected fixed='constant'")
	}
	if elems[4].MinOccurs != 0 || elems[4].MaxOccurs != -1 {
		t.Errorf("multi: expected 0..unbounded, got %d..%d", elems[4].MinOccurs, elems[4].MaxOccurs)
	}
}

// ---------------------------------------------------------------------------
// SchemaSet cross-schema index
// ---------------------------------------------------------------------------

func TestSchemaSetIndex(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="MyString">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "idx.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	qn := xsd.NewQName("http://example.com/test", "MyString")
	if ss.LookupType(qn) == nil {
		t.Error("LookupType: expected to find MyString")
	}

	elemQN := xsd.NewQName("http://example.com/test", "root")
	if ss.LookupElement(elemQN) == nil {
		t.Error("LookupElement: expected to find root")
	}
}

// ---------------------------------------------------------------------------
// Error on malformed XML
// ---------------------------------------------------------------------------

func TestParseMalformedXML(t *testing.T) {
	input := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="broken"
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "bad.xsd")
	if err == nil {
		t.Fatal("expected error for malformed XML")
	}
	// Error should contain file:line:col.
	if !strings.Contains(err.Error(), "bad.xsd") {
		t.Errorf("error should mention systemID, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// List and union simpleType
// ---------------------------------------------------------------------------

func TestParseListType(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:integer"/>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "list.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	st := ss.Schemas[0].Types[0].(*xsd.SimpleType)
	if st.List == nil {
		t.Fatal("expected list type")
	}
	if st.List.ItemType.Name.Local != "integer" {
		t.Errorf("expected itemType integer, got %s", st.List.ItemType.Name.Local)
	}
}

func TestParseUnionType(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="StringOrInt">
    <xs:union memberTypes="xs:string xs:integer"/>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "union.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	st := ss.Schemas[0].Types[0].(*xsd.SimpleType)
	if st.Union == nil {
		t.Fatal("expected union type")
	}
	if len(st.Union.MemberTypes) != 2 {
		t.Fatalf("expected 2 member types, got %d", len(st.Union.MemberTypes))
	}
	if st.Union.MemberTypes[0].Name.Local != "string" {
		t.Errorf("member 0: expected string, got %s", st.Union.MemberTypes[0].Name.Local)
	}
	if st.Union.MemberTypes[1].Name.Local != "integer" {
		t.Errorf("member 1: expected integer, got %s", st.Union.MemberTypes[1].Name.Local)
	}
}

// ---------------------------------------------------------------------------
// Mixed content complexType
// ---------------------------------------------------------------------------

func TestParseMixedContent(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="MixedType" mixed="true">
    <xs:sequence>
      <xs:element name="bold" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "mixed.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	if !ct.Mixed {
		t.Error("expected mixed=true")
	}
}

// ---------------------------------------------------------------------------
// All 49 built-in types: parse elements referencing them
// ---------------------------------------------------------------------------

func TestParseAllBuiltinTypeReferences(t *testing.T) {
	builtins := []string{
		"string", "boolean", "decimal", "float", "double",
		"duration", "dateTime", "time", "date",
		"gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth",
		"hexBinary", "base64Binary", "anyURI", "QName", "NOTATION",
		"normalizedString", "token", "language", "NMTOKEN", "Name",
		"NCName", "ID", "IDREF", "ENTITY",
		"integer", "nonPositiveInteger", "negativeInteger",
		"long", "int", "short", "byte",
		"nonNegativeInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "positiveInteger",
		"yearMonthDuration", "dayTimeDuration", "dateTimeStamp",
		"NMTOKENS", "IDREFS", "ENTITIES",
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
`)
	for _, bt := range builtins {
		fmt.Fprintf(&sb, `  <xs:element name="el_%s" type="xs:%s"/>
`, bt, bt)
	}
	sb.WriteString(`</xs:schema>`)

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(sb.String()), "builtins.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Elements) != len(builtins) {
		t.Fatalf("expected %d elements, got %d", len(builtins), len(schema.Elements))
	}

	for i, bt := range builtins {
		elem := schema.Elements[i]
		if elem.Type.Name.Local != bt {
			t.Errorf("element %d: expected type %q, got %q", i, bt, elem.Type.Name.Local)
		}
		if elem.Type.Name.Namespace != xsd.XSDNS {
			t.Errorf("element %d: expected XSD namespace, got %q", i, elem.Type.Name.Namespace)
		}
	}
}
