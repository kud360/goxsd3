package parser

import (
	"strings"
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

// ---------------------------------------------------------------------------
// Sprint 6: Choice & Nested Compositors — comprehensive tests
// ---------------------------------------------------------------------------

// TestBasicChoice verifies a choice with three direct element children.
func TestBasicChoice(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/choice/basic_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "PaymentType" {
		t.Errorf("expected PaymentType, got %s", ct.Name.Local)
	}

	choice, ok := ct.Content.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice content, got %T", ct.Content)
	}
	if len(choice.Items) != 3 {
		t.Fatalf("expected 3 items in choice, got %d", len(choice.Items))
	}

	expected := []struct {
		name     string
		typeName string
	}{
		{"creditCard", "string"},
		{"bankTransfer", "string"},
		{"cash", "boolean"},
	}
	for i, tc := range expected {
		elem, ok := choice.Items[i].(*xsd.Element)
		if !ok {
			t.Fatalf("choice item %d: expected Element, got %T", i, choice.Items[i])
		}
		if elem.Name != tc.name {
			t.Errorf("choice item %d: expected name %q, got %q", i, tc.name, elem.Name)
		}
		if elem.Type.Name.Local != tc.typeName {
			t.Errorf("choice item %d (%s): expected type %q, got %q", i, tc.name, tc.typeName, elem.Type.Name.Local)
		}
	}
}

// TestChoiceInSequence verifies a choice nested inside a sequence.
func TestChoiceInSequence(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/choice/nested_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	// Find OrderType.
	var orderType *xsd.ComplexType
	for _, typ := range schema.Types {
		if ct, ok := typ.(*xsd.ComplexType); ok && ct.Name.Local == "OrderType" {
			orderType = ct
			break
		}
	}
	if orderType == nil {
		t.Fatal("OrderType not found")
	}

	seq, ok := orderType.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("OrderType: expected Sequence content, got %T", orderType.Content)
	}
	if len(seq.Items) != 3 {
		t.Fatalf("OrderType sequence: expected 3 items, got %d", len(seq.Items))
	}

	// First item: element "id".
	idElem, ok := seq.Items[0].(*xsd.Element)
	if !ok {
		t.Fatalf("item 0: expected Element, got %T", seq.Items[0])
	}
	if idElem.Name != "id" {
		t.Errorf("item 0: expected 'id', got %q", idElem.Name)
	}

	// Second item: choice with "domestic" and "international".
	choice, ok := seq.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("item 1: expected Choice, got %T", seq.Items[1])
	}
	if len(choice.Items) != 2 {
		t.Fatalf("choice: expected 2 items, got %d", len(choice.Items))
	}
	domesticElem := choice.Items[0].(*xsd.Element)
	if domesticElem.Name != "domestic" {
		t.Errorf("choice item 0: expected 'domestic', got %q", domesticElem.Name)
	}
	intlElem := choice.Items[1].(*xsd.Element)
	if intlElem.Name != "international" {
		t.Errorf("choice item 1: expected 'international', got %q", intlElem.Name)
	}

	// Third item: element "total".
	totalElem, ok := seq.Items[2].(*xsd.Element)
	if !ok {
		t.Fatalf("item 2: expected Element, got %T", seq.Items[2])
	}
	if totalElem.Name != "total" {
		t.Errorf("item 2: expected 'total', got %q", totalElem.Name)
	}
}

// TestSequenceInChoice verifies a sequence nested inside a choice.
func TestSequenceInChoice(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/choice/nested_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	// Find ContactType.
	var contactType *xsd.ComplexType
	for _, typ := range schema.Types {
		if ct, ok := typ.(*xsd.ComplexType); ok && ct.Name.Local == "ContactType" {
			contactType = ct
			break
		}
	}
	if contactType == nil {
		t.Fatal("ContactType not found")
	}

	choice, ok := contactType.Content.(*xsd.Choice)
	if !ok {
		t.Fatalf("ContactType: expected Choice content, got %T", contactType.Content)
	}
	if len(choice.Items) != 2 {
		t.Fatalf("ContactType choice: expected 2 items, got %d", len(choice.Items))
	}

	// First item: sequence with "street" and "city".
	seq, ok := choice.Items[0].(*xsd.Sequence)
	if !ok {
		t.Fatalf("choice item 0: expected Sequence, got %T", choice.Items[0])
	}
	if len(seq.Items) != 2 {
		t.Fatalf("nested sequence: expected 2 items, got %d", len(seq.Items))
	}
	streetElem := seq.Items[0].(*xsd.Element)
	if streetElem.Name != "street" {
		t.Errorf("seq item 0: expected 'street', got %q", streetElem.Name)
	}
	cityElem := seq.Items[1].(*xsd.Element)
	if cityElem.Name != "city" {
		t.Errorf("seq item 1: expected 'city', got %q", cityElem.Name)
	}

	// Second item: element "poBox".
	poBoxElem, ok := choice.Items[1].(*xsd.Element)
	if !ok {
		t.Fatalf("choice item 1: expected Element, got %T", choice.Items[1])
	}
	if poBoxElem.Name != "poBox" {
		t.Errorf("choice item 1: expected 'poBox', got %q", poBoxElem.Name)
	}
}

// TestChoiceInChoice verifies nested choice compositors (choice containing choices).
func TestChoiceInChoice(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/nested/choice_in_choice.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "TransportType" {
		t.Errorf("expected TransportType, got %s", ct.Name.Local)
	}

	outerChoice, ok := ct.Content.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice content, got %T", ct.Content)
	}
	if len(outerChoice.Items) != 3 {
		t.Fatalf("outer choice: expected 3 items, got %d", len(outerChoice.Items))
	}

	// Item 0: nested choice with car/motorcycle.
	innerChoice1, ok := outerChoice.Items[0].(*xsd.Choice)
	if !ok {
		t.Fatalf("item 0: expected Choice, got %T", outerChoice.Items[0])
	}
	if len(innerChoice1.Items) != 2 {
		t.Fatalf("inner choice 1: expected 2 items, got %d", len(innerChoice1.Items))
	}
	if innerChoice1.Items[0].(*xsd.Element).Name != "car" {
		t.Errorf("inner choice 1 item 0: expected 'car', got %q", innerChoice1.Items[0].(*xsd.Element).Name)
	}
	if innerChoice1.Items[1].(*xsd.Element).Name != "motorcycle" {
		t.Errorf("inner choice 1 item 1: expected 'motorcycle', got %q", innerChoice1.Items[1].(*xsd.Element).Name)
	}

	// Item 1: nested choice with bus/train.
	innerChoice2, ok := outerChoice.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("item 1: expected Choice, got %T", outerChoice.Items[1])
	}
	if len(innerChoice2.Items) != 2 {
		t.Fatalf("inner choice 2: expected 2 items, got %d", len(innerChoice2.Items))
	}
	if innerChoice2.Items[0].(*xsd.Element).Name != "bus" {
		t.Errorf("inner choice 2 item 0: expected 'bus'")
	}
	if innerChoice2.Items[1].(*xsd.Element).Name != "train" {
		t.Errorf("inner choice 2 item 1: expected 'train'")
	}

	// Item 2: direct element "bicycle".
	bicycleElem, ok := outerChoice.Items[2].(*xsd.Element)
	if !ok {
		t.Fatalf("item 2: expected Element, got %T", outerChoice.Items[2])
	}
	if bicycleElem.Name != "bicycle" {
		t.Errorf("item 2: expected 'bicycle', got %q", bicycleElem.Name)
	}
}

// TestDeepNesting verifies 4+ levels of compositor nesting:
// sequence > choice > sequence > choice.
func TestDeepNesting(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/nested/deep_nesting.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "DeepType" {
		t.Errorf("expected DeepType, got %s", ct.Name.Local)
	}

	// Level 1: sequence with "level1" element and a choice.
	seq1, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("level 1: expected Sequence, got %T", ct.Content)
	}
	if len(seq1.Items) != 2 {
		t.Fatalf("level 1 sequence: expected 2 items, got %d", len(seq1.Items))
	}

	level1Elem := seq1.Items[0].(*xsd.Element)
	if level1Elem.Name != "level1" {
		t.Errorf("expected 'level1', got %q", level1Elem.Name)
	}

	// Level 2: choice with (sequence, element).
	choice2, ok := seq1.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("level 2: expected Choice, got %T", seq1.Items[1])
	}
	if len(choice2.Items) != 2 {
		t.Fatalf("level 2 choice: expected 2 items, got %d", len(choice2.Items))
	}

	// Level 3: sequence with "level3a" and inner choice.
	seq3, ok := choice2.Items[0].(*xsd.Sequence)
	if !ok {
		t.Fatalf("level 3: expected Sequence, got %T", choice2.Items[0])
	}
	if len(seq3.Items) != 2 {
		t.Fatalf("level 3 sequence: expected 2 items, got %d", len(seq3.Items))
	}

	level3aElem := seq3.Items[0].(*xsd.Element)
	if level3aElem.Name != "level3a" {
		t.Errorf("expected 'level3a', got %q", level3aElem.Name)
	}

	// Level 4: choice with "level4a" and "level4b".
	choice4, ok := seq3.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("level 4: expected Choice, got %T", seq3.Items[1])
	}
	if len(choice4.Items) != 2 {
		t.Fatalf("level 4 choice: expected 2 items, got %d", len(choice4.Items))
	}

	level4a := choice4.Items[0].(*xsd.Element)
	if level4a.Name != "level4a" {
		t.Errorf("expected 'level4a', got %q", level4a.Name)
	}
	if level4a.Type.Name.Local != "string" {
		t.Errorf("level4a: expected type string, got %s", level4a.Type.Name.Local)
	}

	level4b := choice4.Items[1].(*xsd.Element)
	if level4b.Name != "level4b" {
		t.Errorf("expected 'level4b', got %q", level4b.Name)
	}
	if level4b.Type.Name.Local != "integer" {
		t.Errorf("level4b: expected type integer, got %s", level4b.Type.Name.Local)
	}

	// The alternate branch: element "level2alt".
	altElem := choice2.Items[1].(*xsd.Element)
	if altElem.Name != "level2alt" {
		t.Errorf("expected 'level2alt', got %q", altElem.Name)
	}
}

// TestMixedCompositors verifies all three compositor types used in one type:
// sequence containing a choice, an all, and direct elements.
func TestMixedCompositors(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/nested/mixed_compositors.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "MixedCompType" {
		t.Errorf("expected MixedCompType, got %s", ct.Name.Local)
	}

	seq, ok := ct.Content.(*xsd.Sequence)
	if !ok {
		t.Fatalf("expected Sequence content, got %T", ct.Content)
	}
	// header, choice, all, footer = 4 items.
	if len(seq.Items) != 4 {
		t.Fatalf("expected 4 items in sequence, got %d", len(seq.Items))
	}

	// Item 0: element "header".
	headerElem := seq.Items[0].(*xsd.Element)
	if headerElem.Name != "header" {
		t.Errorf("item 0: expected 'header', got %q", headerElem.Name)
	}

	// Item 1: choice with "optA" and "optB".
	choice, ok := seq.Items[1].(*xsd.Choice)
	if !ok {
		t.Fatalf("item 1: expected Choice, got %T", seq.Items[1])
	}
	if len(choice.Items) != 2 {
		t.Fatalf("choice: expected 2 items, got %d", len(choice.Items))
	}
	if choice.Items[0].(*xsd.Element).Name != "optA" {
		t.Errorf("choice item 0: expected 'optA'")
	}
	if choice.Items[1].(*xsd.Element).Name != "optB" {
		t.Errorf("choice item 1: expected 'optB'")
	}

	// Item 2: all with "x" and "y".
	all, ok := seq.Items[2].(*xsd.All)
	if !ok {
		t.Fatalf("item 2: expected All, got %T", seq.Items[2])
	}
	if len(all.Items) != 2 {
		t.Fatalf("all: expected 2 items, got %d", len(all.Items))
	}
	if all.Items[0].(*xsd.Element).Name != "x" {
		t.Errorf("all item 0: expected 'x'")
	}
	if all.Items[1].(*xsd.Element).Name != "y" {
		t.Errorf("all item 1: expected 'y'")
	}

	// Item 3: element "footer".
	footerElem := seq.Items[3].(*xsd.Element)
	if footerElem.Name != "footer" {
		t.Errorf("item 3: expected 'footer', got %q", footerElem.Name)
	}
}

// TestAllCompositor verifies parsing of xs:all as the top-level compositor.
func TestAllCompositor(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/nested/sequence_in_all.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	ct := schema.Types[0].(*xsd.ComplexType)
	if ct.Name.Local != "FlexibleType" {
		t.Errorf("expected FlexibleType, got %s", ct.Name.Local)
	}

	all, ok := ct.Content.(*xsd.All)
	if !ok {
		t.Fatalf("expected All content, got %T", ct.Content)
	}
	if len(all.Items) != 3 {
		t.Fatalf("expected 3 items in all, got %d", len(all.Items))
	}

	expected := []struct {
		name     string
		typeName string
	}{
		{"name", "string"},
		{"value", "string"},
		{"extra", "string"},
	}
	for i, tc := range expected {
		elem, ok := all.Items[i].(*xsd.Element)
		if !ok {
			t.Fatalf("all item %d: expected Element, got %T", i, all.Items[i])
		}
		if elem.Name != tc.name {
			t.Errorf("all item %d: expected name %q, got %q", i, tc.name, elem.Name)
		}
		if elem.Type.Name.Local != tc.typeName {
			t.Errorf("all item %d: expected type %q, got %q", i, tc.typeName, elem.Type.Name.Local)
		}
	}

	// Verify minOccurs on the optional element "extra".
	extraElem := all.Items[2].(*xsd.Element)
	if extraElem.MinOccurs != 0 {
		t.Errorf("extra: expected minOccurs=0, got %d", extraElem.MinOccurs)
	}
}

// TestCompositorOccurs verifies minOccurs/maxOccurs on compositor elements.
func TestCompositorOccurs(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="OccursType">
    <xs:sequence>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "occurs.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	seq := ct.Content.(*xsd.Sequence)
	choice := seq.Items[0].(*xsd.Choice)

	if choice.MinOccurs != 0 {
		t.Errorf("choice: expected minOccurs=0, got %d", choice.MinOccurs)
	}
	if choice.MaxOccurs != -1 {
		t.Errorf("choice: expected maxOccurs=-1 (unbounded), got %d", choice.MaxOccurs)
	}
}

// TestSequenceDefaultOccurs verifies sequence defaults to minOccurs=1, maxOccurs=1.
func TestSequenceDefaultOccurs(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="DefaultType">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "defaults.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	seq := ct.Content.(*xsd.Sequence)

	if seq.MinOccurs != 1 {
		t.Errorf("sequence: expected minOccurs=1, got %d", seq.MinOccurs)
	}
	if seq.MaxOccurs != 1 {
		t.Errorf("sequence: expected maxOccurs=1, got %d", seq.MaxOccurs)
	}
}

// TestEmptyChoice verifies parsing an empty choice compositor.
func TestEmptyChoice(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="EmptyChoiceType">
    <xs:choice/>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "empty.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	choice, ok := ct.Content.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice content, got %T", ct.Content)
	}
	if len(choice.Items) != 0 {
		t.Errorf("expected 0 items in empty choice, got %d", len(choice.Items))
	}
}

// TestCompositorParticlesInterface verifies that Compositor.Particles()
// returns the correct particles.
func TestCompositorParticlesInterface(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="TestType">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "iface.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	comp, ok := ct.Content.(xsd.Compositor)
	if !ok {
		t.Fatalf("expected Compositor, got %T", ct.Content)
	}

	particles := comp.Particles()
	if len(particles) != 2 {
		t.Fatalf("expected 2 particles, got %d", len(particles))
	}

	for i, p := range particles {
		elem, ok := p.(*xsd.Element)
		if !ok {
			t.Fatalf("particle %d: expected Element, got %T", i, p)
		}
		if elem.Name == "" {
			t.Errorf("particle %d: empty name", i)
		}
	}
}

// TestCompositorInExtension verifies a compositor inside a complexContent extension.
func TestCompositorInExtension(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="id" type="xs:integer"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="ExtType">
    <xs:complexContent>
      <xs:extension base="BaseType">
        <xs:choice>
          <xs:element name="optX" type="xs:string"/>
          <xs:element name="optY" type="xs:string"/>
        </xs:choice>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ext_comp.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(schema.Types))
	}

	extType := schema.Types[1].(*xsd.ComplexType)
	cc := extType.Content.(*xsd.ComplexContent)
	if cc.Extension == nil {
		t.Fatal("expected extension")
	}

	choice, ok := cc.Extension.Compositor.(*xsd.Choice)
	if !ok {
		t.Fatalf("expected Choice in extension, got %T", cc.Extension.Compositor)
	}
	if len(choice.Items) != 2 {
		t.Fatalf("expected 2 choice items, got %d", len(choice.Items))
	}
	if choice.Items[0].(*xsd.Element).Name != "optX" {
		t.Errorf("expected 'optX', got %q", choice.Items[0].(*xsd.Element).Name)
	}
}

// TestCompositorTypeResolution verifies that type references inside nested
// compositors are properly resolved.
func TestCompositorTypeResolution(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/nested/deep_nesting.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ct := ss.Schemas[0].Types[0].(*xsd.ComplexType)
	seq := ct.Content.(*xsd.Sequence)

	// Walk down to level4b (integer type) and verify resolution.
	choice := seq.Items[1].(*xsd.Choice)
	innerSeq := choice.Items[0].(*xsd.Sequence)
	innerChoice := innerSeq.Items[1].(*xsd.Choice)
	level4b := innerChoice.Items[1].(*xsd.Element)

	if level4b.Type.Name.Local != "integer" {
		t.Errorf("level4b: expected type 'integer', got %q", level4b.Type.Name.Local)
	}
	if level4b.Type.Resolved == nil {
		t.Error("level4b: type ref should be resolved (built-in integer)")
	}
}
