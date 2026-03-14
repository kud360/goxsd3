package parser

import (
	"strings"
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

// ---------------------------------------------------------------------------
// SimpleType restriction — base wired up to parent
// ---------------------------------------------------------------------------

func TestResolve_SimpleRestriction(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/simple_restriction.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(schema.Types))
	}

	st := schema.Types[0].(*xsd.SimpleType)
	if st.Name.Local != "ShortString" {
		t.Errorf("expected ShortString, got %s", st.Name.Local)
	}
	if st.Restriction == nil {
		t.Fatal("expected restriction")
	}

	// Base should be resolved to xs:string (built-in).
	base := st.Restriction.Base
	if base.Name.Local != "string" || base.Name.Namespace != xsd.XSDNS {
		t.Errorf("expected base xs:string, got %s", base.Name)
	}
	if base.Resolved == nil {
		t.Error("expected base to be resolved")
	}

	// Element referencing the user-defined type should be resolved.
	elem := schema.Elements[0]
	if elem.Type.Name.Local != "ShortString" {
		t.Errorf("expected element type ShortString, got %s", elem.Type.Name.Local)
	}
	if elem.Type.Resolved == nil {
		t.Error("expected element type ref to be resolved")
	}
}

// ---------------------------------------------------------------------------
// ComplexType extension — base wired up
// ---------------------------------------------------------------------------

func TestResolve_ComplexExtension(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/complex_extension.xsd")
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

	cc := derived.Content.(*xsd.ComplexContent)
	if cc.Extension == nil {
		t.Fatal("expected extension")
	}

	// BaseType should be resolved (it was defined before DerivedType).
	if cc.Extension.Base.Resolved == nil {
		t.Error("expected extension base to be resolved")
	}
	if cc.Extension.Base.Resolved.Local != "BaseType" {
		t.Errorf("expected resolved base BaseType, got %s", cc.Extension.Base.Resolved.Local)
	}

	// Extension should have attributes.
	if len(cc.Extension.Attributes) != 1 {
		t.Errorf("expected 1 attribute on extension, got %d", len(cc.Extension.Attributes))
	}

	// Verify BaseType is in symbol table.
	baseQN := xsd.NewQName("http://example.com/test", "BaseType")
	if p.Symbols().LookupType(baseQN) == nil {
		t.Error("BaseType not found in symbol table")
	}
}

// ---------------------------------------------------------------------------
// ComplexType restriction
// ---------------------------------------------------------------------------

func TestResolve_ComplexRestriction(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/complex_restriction.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	strict := ss.Schemas[0].Types[1].(*xsd.ComplexType)
	if strict.Name.Local != "StrictType" {
		t.Errorf("expected StrictType, got %s", strict.Name.Local)
	}

	cc := strict.Content.(*xsd.ComplexContent)
	if cc.Restriction == nil {
		t.Fatal("expected restriction in complex content")
	}
	if cc.Restriction.Base.Resolved == nil {
		t.Error("expected restriction base to be resolved")
	}
	if cc.Restriction.Base.Resolved.Local != "BaseType" {
		t.Errorf("expected resolved base BaseType, got %s", cc.Restriction.Base.Resolved.Local)
	}

	// Restriction should have an inline compositor (restricted sequence).
	if cc.Restriction.Content == nil {
		t.Error("expected restriction content (sequence)")
	}
}

// ---------------------------------------------------------------------------
// Chained restriction: A restricts string, B restricts A, C restricts B
// ---------------------------------------------------------------------------

func TestResolve_ChainedRestriction(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/chained_restriction.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(schema.Types))
	}

	// NonEmptyString restricts xs:string (built-in).
	nes := schema.Types[0].(*xsd.SimpleType)
	if nes.Restriction.Base.Resolved == nil {
		t.Error("NonEmptyString: base should be resolved (xs:string)")
	}

	// ShortNonEmptyString restricts NonEmptyString (user type, defined before).
	snes := schema.Types[1].(*xsd.SimpleType)
	if snes.Restriction.Base.Resolved == nil {
		t.Error("ShortNonEmptyString: base should be resolved (NonEmptyString)")
	}
	if snes.Restriction.Base.Resolved.Local != "NonEmptyString" {
		t.Errorf("expected NonEmptyString, got %s", snes.Restriction.Base.Resolved.Local)
	}

	// TinyString restricts ShortNonEmptyString.
	ts := schema.Types[2].(*xsd.SimpleType)
	if ts.Restriction.Base.Resolved == nil {
		t.Error("TinyString: base should be resolved (ShortNonEmptyString)")
	}
	if ts.Restriction.Base.Resolved.Local != "ShortNonEmptyString" {
		t.Errorf("expected ShortNonEmptyString, got %s", ts.Restriction.Base.Resolved.Local)
	}

	// All 3 types in symbol table.
	if p.Symbols().TypeCount() != 3 {
		t.Errorf("expected 3 types in symbol table, got %d", p.Symbols().TypeCount())
	}
}

// ---------------------------------------------------------------------------
// Multi-level inheritance: Shape → Rectangle → Square
// ---------------------------------------------------------------------------

func TestResolve_MultiLevelInheritance(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/multi_level.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(schema.Types))
	}

	// Rectangle extends Shape.
	rect := schema.Types[1].(*xsd.ComplexType)
	cc := rect.Content.(*xsd.ComplexContent)
	if cc.Extension.Base.Resolved == nil {
		t.Error("Rectangle: base Shape should be resolved")
	}
	if cc.Extension.Base.Resolved.Local != "Shape" {
		t.Errorf("Rectangle base: expected Shape, got %s", cc.Extension.Base.Resolved.Local)
	}

	// Square extends Rectangle.
	sq := schema.Types[2].(*xsd.ComplexType)
	cc2 := sq.Content.(*xsd.ComplexContent)
	if cc2.Extension.Base.Resolved == nil {
		t.Error("Square: base Rectangle should be resolved")
	}
	if cc2.Extension.Base.Resolved.Local != "Rectangle" {
		t.Errorf("Square base: expected Rectangle, got %s", cc2.Extension.Base.Resolved.Local)
	}
}

// ---------------------------------------------------------------------------
// Abstract types with concrete subtypes
// ---------------------------------------------------------------------------

func TestResolve_AbstractBase(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/derivation/abstract_base.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(schema.Types))
	}

	vehicle := schema.Types[0].(*xsd.ComplexType)
	if !vehicle.Abstract {
		t.Error("Vehicle should be abstract")
	}

	car := schema.Types[1].(*xsd.ComplexType)
	if car.Name.Local != "Car" {
		t.Errorf("expected Car, got %s", car.Name.Local)
	}
	cc := car.Content.(*xsd.ComplexContent)
	if cc.Extension.Base.Resolved == nil {
		t.Error("Car: base Vehicle should be resolved")
	}
	if cc.Extension.Base.Resolved.Local != "Vehicle" {
		t.Errorf("Car base: expected Vehicle, got %s", cc.Extension.Base.Resolved.Local)
	}

	truck := schema.Types[2].(*xsd.ComplexType)
	if truck.Name.Local != "Truck" {
		t.Errorf("expected Truck, got %s", truck.Name.Local)
	}
	cc2 := truck.Content.(*xsd.ComplexContent)
	if cc2.Extension.Base.Resolved == nil {
		t.Error("Truck: base Vehicle should be resolved")
	}
}

// ---------------------------------------------------------------------------
// Forward + backward reference resolution (incremental_refs.xsd)
// ---------------------------------------------------------------------------

func TestResolve_IncrementalRefs(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/parser/incremental_refs.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]

	// Elements: order (forward), customer (forward), pastOrder (backward)
	// Types: OrderType, CustomerType
	if len(schema.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(schema.Elements))
	}
	if len(schema.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(schema.Types))
	}

	// "order" element references OrderType — forward reference at parse time,
	// should be resolved after forward ref resolution pass.
	order := schema.Elements[0]
	if order.Name != "order" {
		t.Errorf("expected 'order', got %q", order.Name)
	}
	if order.Type.Resolved == nil {
		t.Fatal("order: forward ref to OrderType should be resolved")
	}
	if order.Type.Resolved.Local != "OrderType" {
		t.Errorf("order: expected resolved OrderType, got %s", order.Type.Resolved.Local)
	}

	// "customer" element references CustomerType — also forward.
	customer := schema.Elements[1]
	if customer.Type.Resolved == nil {
		t.Fatal("customer: forward ref to CustomerType should be resolved")
	}
	if customer.Type.Resolved.Local != "CustomerType" {
		t.Errorf("customer: expected resolved CustomerType, got %s", customer.Type.Resolved.Local)
	}

	// "pastOrder" element references OrderType — backward reference (already defined).
	pastOrder := schema.Elements[2]
	if pastOrder.Type.Resolved == nil {
		t.Error("pastOrder: backward ref to OrderType should be resolved")
	}

	// OrderType contains an element "buyer" with type CustomerType (forward ref within type).
	orderType := schema.Types[0].(*xsd.ComplexType)
	seq := orderType.Content.(*xsd.Sequence)
	buyer := seq.Items[1].(*xsd.Element)
	if buyer.Name != "buyer" {
		t.Errorf("expected 'buyer', got %q", buyer.Name)
	}
	if buyer.Type.Resolved == nil {
		t.Error("buyer: forward ref to CustomerType should be resolved")
	}
}

// ---------------------------------------------------------------------------
// Built-in type references are always resolved
// ---------------------------------------------------------------------------

func TestResolve_BuiltinTypes(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="name" type="xs:string"/>
  <xs:element name="age" type="xs:integer"/>
  <xs:element name="price" type="xs:decimal"/>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "builtin.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	for _, elem := range ss.Schemas[0].Elements {
		if elem.Type.Resolved == nil {
			t.Errorf("element %q: built-in type %s should be resolved",
				elem.Name, elem.Type.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// List type with itemType resolution
// ---------------------------------------------------------------------------

func TestResolve_ListType(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/builtin/list_type.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(schema.Types))
	}

	// IntList — list of xs:integer.
	intList := schema.Types[0].(*xsd.SimpleType)
	if intList.List == nil {
		t.Fatal("expected list type")
	}
	if intList.List.ItemType.Resolved == nil {
		t.Error("IntList itemType should be resolved to xs:integer")
	}

	// BoundedIntList restricts IntList.
	bounded := schema.Types[1].(*xsd.SimpleType)
	if bounded.Restriction == nil {
		t.Fatal("expected restriction on BoundedIntList")
	}
	if bounded.Restriction.Base.Resolved == nil {
		t.Error("BoundedIntList base should be resolved to IntList")
	}
	if bounded.Restriction.Base.Resolved.Local != "IntList" {
		t.Errorf("expected IntList, got %s", bounded.Restriction.Base.Resolved.Local)
	}
}

// ---------------------------------------------------------------------------
// Union type with memberTypes resolution
// ---------------------------------------------------------------------------

func TestResolve_UnionType(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/builtin/union_type.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	schema := ss.Schemas[0]
	if len(schema.Types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(schema.Types))
	}

	// StringOrInt — union of xs:string and xs:integer.
	strOrInt := schema.Types[0].(*xsd.SimpleType)
	if strOrInt.Union == nil {
		t.Fatal("expected union type")
	}
	for i, mt := range strOrInt.Union.MemberTypes {
		if mt.Resolved == nil {
			t.Errorf("StringOrInt member %d (%s) should be resolved", i, mt.Name)
		}
	}

	// StatusOrNumber — union of StatusCode (user type) and xs:integer.
	statusOrNum := schema.Types[2].(*xsd.SimpleType)
	if statusOrNum.Union == nil {
		t.Fatal("expected union type for StatusOrNumber")
	}
	// StatusCode is defined before StatusOrNumber, so should be resolved.
	if statusOrNum.Union.MemberTypes[0].Resolved == nil {
		t.Error("StatusOrNumber member StatusCode should be resolved")
	}
	if statusOrNum.Union.MemberTypes[0].Resolved.Local != "StatusCode" {
		t.Errorf("expected StatusCode, got %s", statusOrNum.Union.MemberTypes[0].Resolved.Local)
	}
	// xs:integer should be resolved.
	if statusOrNum.Union.MemberTypes[1].Resolved == nil {
		t.Error("StatusOrNumber member xs:integer should be resolved")
	}
}

// ---------------------------------------------------------------------------
// Symbol table populated during parse
// ---------------------------------------------------------------------------

func TestResolve_SymbolTablePopulated(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="A">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:complexType name="B">
    <xs:sequence>
      <xs:element name="x" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="C">
    <xs:complexContent>
      <xs:extension base="B">
        <xs:sequence>
          <xs:element name="y" type="A"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "sym.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ns := "http://example.com/test"
	for _, name := range []string{"A", "B", "C"} {
		qn := xsd.NewQName(ns, name)
		if p.Symbols().LookupType(qn) == nil {
			t.Errorf("type %s not found in symbol table", name)
		}
	}
	if p.Symbols().TypeCount() != 3 {
		t.Errorf("expected 3 types in symbol table, got %d", p.Symbols().TypeCount())
	}
}

// ---------------------------------------------------------------------------
// Type ref within extension resolves to user-defined type
// ---------------------------------------------------------------------------

func TestResolve_ExtensionRefToUserType(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="NameType">
    <xs:restriction base="xs:string">
      <xs:maxLength value="100"/>
    </xs:restriction>
  </xs:simpleType>

  <xs:complexType name="PersonType">
    <xs:sequence>
      <xs:element name="name" type="NameType"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="EmployeeType">
    <xs:complexContent>
      <xs:extension base="PersonType">
        <xs:sequence>
          <xs:element name="title" type="NameType"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	p := newTestParser()
	ss, err := p.ParseReader(strings.NewReader(input), "ref.xsd")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// EmployeeType → extension base PersonType should be resolved.
	emp := ss.Schemas[0].Types[2].(*xsd.ComplexType)
	cc := emp.Content.(*xsd.ComplexContent)
	if cc.Extension.Base.Resolved == nil {
		t.Error("EmployeeType: base PersonType should be resolved")
	}

	// EmployeeType → extension → sequence → title element → type NameType resolved.
	seq := cc.Extension.Compositor.(*xsd.Sequence)
	title := seq.Items[0].(*xsd.Element)
	if title.Type.Resolved == nil {
		t.Error("title: type NameType should be resolved")
	}
	if title.Type.Resolved.Local != "NameType" {
		t.Errorf("title: expected NameType, got %s", title.Type.Resolved.Local)
	}
}
