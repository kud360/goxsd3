package xsd_test

import (
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

func TestSchemaSetAddAndLookup(t *testing.T) {
	ss := xsd.NewSchemaSet()
	s := xsd.NewSchema("http://example.com/ns")
	ss.AddSchema(s)

	if len(ss.Schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(ss.Schemas))
	}
	if got := ss.SchemaByNamespace("http://example.com/ns"); got != s {
		t.Fatal("SchemaByNamespace did not return the added schema")
	}
	if got := ss.SchemaByNamespace("http://other.com"); got != nil {
		t.Fatal("expected nil for unknown namespace")
	}
}

func TestSchemaSetTypeLookup(t *testing.T) {
	ss := xsd.NewSchemaSet()
	st := &xsd.SimpleType{Name: xsd.NewQName("http://example.com", "MyString")}
	ss.RegisterType(st)

	if got := ss.LookupType(st.Name); got != st {
		t.Fatal("LookupType did not return the registered type")
	}
	if got := ss.LookupType(xsd.NewQName("", "missing")); got != nil {
		t.Fatal("expected nil for unknown type")
	}
}

func TestSchemaSetElementLookup(t *testing.T) {
	ss := xsd.NewSchemaSet()
	e := &xsd.Element{Name: "root", Namespace: "http://example.com"}
	ss.RegisterElement(e)

	if got := ss.LookupElement(xsd.NewQName("http://example.com", "root")); got != e {
		t.Fatal("LookupElement did not return the registered element")
	}
}

func TestSchemaAddElement(t *testing.T) {
	s := xsd.NewSchema("urn:test")
	e1 := &xsd.Element{Name: "first"}
	e2 := &xsd.Element{Name: "second"}
	s.AddElement(e1)
	s.AddElement(e2)

	if len(s.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(s.Elements))
	}
	// Document order preserved.
	if s.Elements[0] != e1 || s.Elements[1] != e2 {
		t.Fatal("element order not preserved")
	}
	if got := s.LookupElement("first"); got != e1 {
		t.Fatal("LookupElement by name failed")
	}
}

func TestSchemaAddType(t *testing.T) {
	s := xsd.NewSchema("urn:test")
	st := &xsd.SimpleType{Name: xsd.NewQName("urn:test", "Phone")}
	ct := &xsd.ComplexType{Name: xsd.NewQName("urn:test", "Address")}
	s.AddType(st)
	s.AddType(ct)

	if len(s.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(s.Types))
	}
	if got := s.LookupType(st.Name); got != st {
		t.Fatal("LookupType for SimpleType failed")
	}
	if got := s.LookupType(ct.Name); got != ct {
		t.Fatal("LookupType for ComplexType failed")
	}
}

func TestSchemaAddGroup(t *testing.T) {
	s := xsd.NewSchema("urn:test")
	g := &xsd.Group{Name: "personGroup"}
	s.AddGroup(g)

	if got := s.LookupGroup("personGroup"); got != g {
		t.Fatal("LookupGroup failed")
	}
	if got := s.LookupGroup("missing"); got != nil {
		t.Fatal("expected nil for unknown group")
	}
}

func TestTypeInterfaces(t *testing.T) {
	// Verify SimpleType and ComplexType satisfy the Type interface.
	var _ xsd.Type = &xsd.SimpleType{}
	var _ xsd.Type = &xsd.ComplexType{}

	st := &xsd.SimpleType{Name: xsd.NewQName("urn:test", "MyType")}
	if st.TypeName() != st.Name {
		t.Fatal("SimpleType.TypeName() mismatch")
	}

	ct := &xsd.ComplexType{Name: xsd.NewQName("urn:test", "MyComplex")}
	if ct.TypeName() != ct.Name {
		t.Fatal("ComplexType.TypeName() mismatch")
	}
}

func TestParticleInterfaces(t *testing.T) {
	// Verify all particle types satisfy the Particle interface.
	var _ xsd.Particle = &xsd.Element{}
	var _ xsd.Particle = &xsd.Sequence{}
	var _ xsd.Particle = &xsd.Choice{}
	var _ xsd.Particle = &xsd.All{}
	var _ xsd.Particle = &xsd.GroupRef{}
	var _ xsd.Particle = &xsd.Any{}
}

func TestCompositorInterfaces(t *testing.T) {
	// Verify all compositor types satisfy both Compositor and Content.
	var _ xsd.Compositor = &xsd.Sequence{}
	var _ xsd.Compositor = &xsd.Choice{}
	var _ xsd.Compositor = &xsd.All{}

	var _ xsd.Content = &xsd.Sequence{}
	var _ xsd.Content = &xsd.Choice{}
	var _ xsd.Content = &xsd.All{}
	var _ xsd.Content = &xsd.SimpleContent{}
	var _ xsd.Content = &xsd.ComplexContent{}
}

func TestCompositorParticles(t *testing.T) {
	e1 := &xsd.Element{Name: "a"}
	e2 := &xsd.Element{Name: "b"}

	seq := &xsd.Sequence{Items: []xsd.Particle{e1, e2}}
	if got := seq.Particles(); len(got) != 2 {
		t.Fatalf("Sequence.Particles() expected 2, got %d", len(got))
	}

	ch := &xsd.Choice{Items: []xsd.Particle{e1}}
	if got := ch.Particles(); len(got) != 1 {
		t.Fatalf("Choice.Particles() expected 1, got %d", len(got))
	}

	all := &xsd.All{Items: []xsd.Particle{e1, e2}}
	if got := all.Particles(); len(got) != 2 {
		t.Fatalf("All.Particles() expected 2, got %d", len(got))
	}
}

func TestModelConstruction(t *testing.T) {
	// Build a small model programmatically and verify traversal.
	s := xsd.NewSchema("http://example.com/order")

	// Simple type with restriction.
	zipType := &xsd.SimpleType{
		Name: xsd.NewQName(s.TargetNamespace, "ZipCode"),
		Restriction: &xsd.Restriction{
			Base: xsd.TypeRef{Name: xsd.XSDName("string")},
			Facets: []xsd.Facet{
				{Kind: xsd.FacetPattern, Value: `\d{5}(-\d{4})?`},
			},
		},
	}
	s.AddType(zipType)

	// Complex type with sequence.
	addrType := &xsd.ComplexType{
		Name: xsd.NewQName(s.TargetNamespace, "Address"),
		Content: &xsd.Sequence{
			MinOccurs: 1,
			MaxOccurs: 1,
			Items: []xsd.Particle{
				&xsd.Element{Name: "street", Type: xsd.TypeRef{Name: xsd.XSDName("string")}},
				&xsd.Element{Name: "zip", Type: xsd.TypeRef{Name: zipType.Name}},
			},
		},
		Attributes: []*xsd.Attribute{
			{Name: "country", Type: xsd.TypeRef{Name: xsd.XSDName("string")}, Use: xsd.AttributeOptional},
		},
	}
	s.AddType(addrType)

	// Top-level element.
	root := &xsd.Element{
		Name:      "order",
		Namespace: s.TargetNamespace,
		Type:      xsd.TypeRef{Name: addrType.Name},
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	s.AddElement(root)

	// Verify.
	if len(s.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(s.Types))
	}
	if len(s.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(s.Elements))
	}

	// Traverse complex type compositor.
	ct := s.LookupType(addrType.Name).(*xsd.ComplexType)
	seq := ct.Content.(*xsd.Sequence)
	if len(seq.Items) != 2 {
		t.Fatalf("expected 2 particles in sequence, got %d", len(seq.Items))
	}
	streetElem := seq.Items[0].(*xsd.Element)
	if streetElem.Name != "street" {
		t.Fatalf("expected first particle name 'street', got %q", streetElem.Name)
	}
}
