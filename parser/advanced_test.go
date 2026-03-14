package parser

import (
	"strings"
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

// ---------------------------------------------------------------------------
// Sprint 8: Advanced Features — model groups, attribute groups, any, substitution
// ---------------------------------------------------------------------------

// TestGroupDefinition verifies xs:group definition parsing with a sequence compositor.
func TestGroupDefinition(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/group.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	// Should have one group: AddressFields.
	if len(schema.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(schema.Groups))
	}

	g := schema.Groups[0]
	if g.Name != "AddressFields" {
		t.Errorf("expected group name 'AddressFields', got %q", g.Name)
	}

	// Group should contain a sequence with 3 elements.
	seq, ok := g.Compositor.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence compositor, got %T", g.Compositor)
	}
	if len(seq.Items) != 3 {
		t.Fatalf("expected 3 items in group sequence, got %d", len(seq.Items))
	}

	expected := []string{"street", "city", "zip"}
	for i, name := range expected {
		elem, ok := seq.Items[i].(*xsd.Element)
		if !ok {
			t.Fatalf("item %d: expected Element, got %T", i, seq.Items[i])
		}
		if elem.Name != name {
			t.Errorf("item %d: expected %q, got %q", i, name, elem.Name)
		}
	}
}

// TestGroupReference verifies xs:group ref parsing inside a complexType.
func TestGroupReference(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/group.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	// PersonType should have a sequence with "name" element and a group ref.
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "PersonType" {
		t.Errorf("expected PersonType, got %s", ct.Name.Local)
	}

	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence content, got %T", ct.Content)
	}
	if len(seq.Items) != 2 {
		t.Fatalf("expected 2 items in sequence, got %d", len(seq.Items))
	}

	// First item: element "name".
	nameElem := seq.Items[0].(*xsd.Element)
	if nameElem.Name != "name" {
		t.Errorf("item 0: expected 'name', got %q", nameElem.Name)
	}

	// Second item: group reference.
	groupRef, ok := seq.Items[1].(*xsd.GroupRef)
	if !ok {
		t.Fatalf("item 1: expected GroupRef, got %T", seq.Items[1])
	}
	if groupRef.Ref.Local != "AddressFields" {
		t.Errorf("group ref: expected 'AddressFields', got %q", groupRef.Ref.Local)
	}
}

// TestGroupLookup verifies schema.LookupGroup works.
func TestGroupLookup(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/group.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	g := schema.LookupGroup("AddressFields")
	if g == nil {
		t.Fatal("LookupGroup: expected to find AddressFields")
	}
	if g.Name != "AddressFields" {
		t.Errorf("expected AddressFields, got %s", g.Name)
	}

	// Non-existent group.
	if schema.LookupGroup("NonExistent") != nil {
		t.Error("LookupGroup: should return nil for non-existent group")
	}
}

// TestGroupWithChoice verifies a group containing a choice compositor.
func TestGroupWithChoice(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:group name="PaymentChoice">
    <xs:choice>
      <xs:element name="cash" type="xs:string"/>
      <xs:element name="card" type="xs:string"/>
    </xs:choice>
  </xs:group>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "group_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(schema.Groups))
	}

	g := schema.Groups[0]
	choice, ok := g.Compositor.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice compositor, got %T", g.Compositor)
	}
	if len(choice.Items) != 2 {
		t.Fatalf("expected 2 items in choice, got %d", len(choice.Items))
	}
}

// ---------------------------------------------------------------------------
// Attribute groups
// ---------------------------------------------------------------------------

// TestAttributeGroupDefinition verifies xs:attributeGroup parsing.
func TestAttributeGroupDefinition(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/attribute_group.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	if len(schema.AttributeGroups) != 1 {
		t.Fatalf("expected 1 attribute group, got %d", len(schema.AttributeGroups))
	}

	ag := schema.AttributeGroups[0]
	if ag.Name != "CommonAttrs" {
		t.Errorf("expected 'CommonAttrs', got %q", ag.Name)
	}
	if len(ag.Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(ag.Attributes))
	}

	// id attribute — required.
	if ag.Attributes[0].Name != "id" {
		t.Errorf("attr 0: expected 'id', got %q", ag.Attributes[0].Name)
	}
	if ag.Attributes[0].Use != xsd.AttributeRequired {
		t.Errorf("attr 0: expected use=required, got %s", ag.Attributes[0].Use)
	}

	// version attribute — optional (default).
	if ag.Attributes[1].Name != "version" {
		t.Errorf("attr 1: expected 'version', got %q", ag.Attributes[1].Name)
	}
}

// TestAttributeGroupReference verifies xs:attributeGroup ref parsing.
func TestAttributeGroupReference(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/attribute_group.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "DocumentType" {
		t.Errorf("expected DocumentType, got %s", ct.Name.Local)
	}

	if len(ct.AttributeGroups) != 1 {
		t.Fatalf("expected 1 attributeGroup ref, got %d", len(ct.AttributeGroups))
	}

	agRef := ct.AttributeGroups[0]
	if agRef.Ref.Local != "CommonAttrs" {
		t.Errorf("expected ref 'CommonAttrs', got %q", agRef.Ref.Local)
	}
}

// ---------------------------------------------------------------------------
// xs:any and xs:anyAttribute
// ---------------------------------------------------------------------------

// TestAnyWildcard verifies xs:any parsing with attributes.
func TestAnyWildcard(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/any.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "ExtensibleType" {
		t.Errorf("expected ExtensibleType, got %s", ct.Name.Local)
	}

	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence, got %T", ct.Content)
	}
	if len(seq.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(seq.Items))
	}

	// First: element "known".
	knownElem := seq.Items[0].(*xsd.Element)
	if knownElem.Name != "known" {
		t.Errorf("item 0: expected 'known', got %q", knownElem.Name)
	}

	// Second: xs:any wildcard.
	any, ok := seq.Items[1].(*xsd.Any)
	if !ok {
		t.Fatalf("item 1: expected Any, got %T", seq.Items[1])
	}
	if any.Namespace != "##other" {
		t.Errorf("any namespace: expected '##other', got %q", any.Namespace)
	}
	if any.ProcessContents != "lax" {
		t.Errorf("any processContents: expected 'lax', got %q", any.ProcessContents)
	}
	if any.MinOccurs != 0 {
		t.Errorf("any minOccurs: expected 0, got %d", any.MinOccurs)
	}
	if any.MaxOccurs != -1 {
		t.Errorf("any maxOccurs: expected -1 (unbounded), got %d", any.MaxOccurs)
	}
}

// TestAnyAttribute verifies xs:anyAttribute parsing.
func TestAnyAttribute(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/any.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	if ct.AnyAttribute == nil {
		t.Fatal("expected anyAttribute to be set")
	}
	if ct.AnyAttribute.Namespace != "##any" {
		t.Errorf("anyAttribute namespace: expected '##any', got %q", ct.AnyAttribute.Namespace)
	}
	if ct.AnyAttribute.ProcessContents != "skip" {
		t.Errorf("anyAttribute processContents: expected 'skip', got %q", ct.AnyAttribute.ProcessContents)
	}
}

// TestAnyAttributeDefault verifies xs:anyAttribute with default processContents.
func TestAnyAttributeDefault(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="OpenType">
    <xs:sequence>
      <xs:element name="data" type="xs:string"/>
    </xs:sequence>
    <xs:anyAttribute/>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "anyattr.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	if ct.AnyAttribute == nil {
		t.Fatal("expected anyAttribute")
	}
	// Default processContents is "strict".
	if ct.AnyAttribute.ProcessContents != "strict" {
		t.Errorf("expected processContents 'strict', got %q", ct.AnyAttribute.ProcessContents)
	}
}

// ---------------------------------------------------------------------------
// Substitution groups
// ---------------------------------------------------------------------------

// TestSubstitutionGroupParsing verifies substitutionGroup attribute on elements.
func TestSubstitutionGroupParsing(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/complex/substitution.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Elements) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(schema.Elements))
	}

	// shape: abstract, no substitutionGroup.
	shape := schema.Elements[0]
	if shape.Name != "shape" {
		t.Errorf("element 0: expected 'shape', got %q", shape.Name)
	}
	if !shape.Abstract {
		t.Error("shape: expected abstract=true")
	}
	if shape.SubstitutionGroup != nil {
		t.Error("shape: should have no substitutionGroup")
	}

	// circle, square, triangle: all point to "shape" substitutionGroup.
	for i, name := range []string{"circle", "square", "triangle"} {
		elem := schema.Elements[i+1]
		if elem.Name != name {
			t.Errorf("element %d: expected %q, got %q", i+1, name, elem.Name)
		}
		if elem.SubstitutionGroup == nil {
			t.Errorf("%s: expected substitutionGroup", name)
			continue
		}
		if elem.SubstitutionGroup.Local != "shape" {
			t.Errorf("%s: expected substitutionGroup 'shape', got %q", name, elem.SubstitutionGroup.Local)
		}
	}
}

// ---------------------------------------------------------------------------
// xs:any inside choice
// ---------------------------------------------------------------------------

func TestAnyInChoice(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="FlexType">
    <xs:choice>
      <xs:element name="known" type="xs:string"/>
      <xs:any namespace="##any" processContents="skip"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "any_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	choice, ok := ct.Content.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice, got %T", ct.Content)
	}
	if len(choice.Items) != 2 {
		t.Fatalf("expected 2 items in choice, got %d", len(choice.Items))
	}

	// First: element.
	if _, ok := choice.Items[0].(*xsd.Element); !ok {
		t.Errorf("item 0: expected Element, got %T", choice.Items[0])
	}

	// Second: any.
	any, ok := choice.Items[1].(*xsd.Any)
	if !ok {
		t.Fatalf("item 1: expected Any, got %T", choice.Items[1])
	}
	if any.Namespace != "##any" {
		t.Errorf("any: expected namespace '##any', got %q", any.Namespace)
	}
}

// ---------------------------------------------------------------------------
// Group ref with occurrence constraints
// ---------------------------------------------------------------------------

func TestGroupRefOccurs(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:group name="Items">
    <xs:sequence>
      <xs:element name="item" type="xs:string"/>
    </xs:sequence>
  </xs:group>

  <xs:complexType name="ListType">
    <xs:sequence>
      <xs:group ref="Items" minOccurs="0" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "groupref.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	seq := ct.Content.(*xsd.Sequence)
	gref := seq.Items[0].(*xsd.GroupRef)

	if gref.Ref.Local != "Items" {
		t.Errorf("expected ref 'Items', got %q", gref.Ref.Local)
	}
	if gref.MinOccurs != 0 {
		t.Errorf("expected minOccurs=0, got %d", gref.MinOccurs)
	}
	if gref.MaxOccurs != -1 {
		t.Errorf("expected maxOccurs=-1 (unbounded), got %d", gref.MaxOccurs)
	}
}

// ---------------------------------------------------------------------------
// AnyAttribute inside extension
// ---------------------------------------------------------------------------

func TestAnyAttributeInExtension(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="x" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="ExtType">
    <xs:complexContent>
      <xs:extension base="BaseType">
        <xs:anyAttribute namespace="##local"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ext_anyattr.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	ext := schema.Types[1].(*xsd.ComplexType)
	cc := ext.Content.(*xsd.ComplexContent)
	if cc.Extension == nil {
		t.Fatal("expected extension")
	}
	if cc.Extension.AnyAttribute == nil {
		t.Fatal("expected anyAttribute in extension")
	}
	if cc.Extension.AnyAttribute.Namespace != "##local" {
		t.Errorf("expected namespace '##local', got %q", cc.Extension.AnyAttribute.Namespace)
	}
}

// ---------------------------------------------------------------------------
// AttributeGroup ref inside extension
// ---------------------------------------------------------------------------

func TestAttributeGroupRefInExtension(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:attributeGroup name="MetaAttrs">
    <xs:attribute name="lang" type="xs:string"/>
  </xs:attributeGroup>

  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="x" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="ExtType">
    <xs:complexContent>
      <xs:extension base="BaseType">
        <xs:attributeGroup ref="MetaAttrs"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ext_agref.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ext := ss.Schemas[0].Types[1].(*xsd.ComplexType)
	cc := ext.Content.(*xsd.ComplexContent)
	if cc.Extension == nil {
		t.Fatal("expected extension")
	}
	if len(cc.Extension.AttributeGroups) != 1 {
		t.Fatalf("expected 1 attributeGroup ref, got %d", len(cc.Extension.AttributeGroups))
	}
	if cc.Extension.AttributeGroups[0].Ref.Local != "MetaAttrs" {
		t.Errorf("expected ref 'MetaAttrs', got %q", cc.Extension.AttributeGroups[0].Ref.Local)
	}
}
