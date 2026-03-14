package parser

import (
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

func TestSymbolTable_AddAndLookupType(t *testing.T) {
	st := NewSymbolTable()
	name := xsd.NewQName("http://example.com", "MyType")

	ct := &xsd.ComplexType{Name: name}
	st.AddType(ct)

	got := st.LookupType(name)
	if got == nil {
		t.Fatal("expected to find MyType")
	}
	if got.TypeName() != name {
		t.Errorf("expected %s, got %s", name, got.TypeName())
	}
}

func TestSymbolTable_LookupMissing(t *testing.T) {
	st := NewSymbolTable()
	name := xsd.NewQName("http://example.com", "Missing")
	if st.LookupType(name) != nil {
		t.Error("expected nil for missing type")
	}
}

func TestSymbolTable_AnonymousTypeNotIndexed(t *testing.T) {
	st := NewSymbolTable()
	ct := &xsd.ComplexType{} // empty name
	st.AddType(ct)
	if st.TypeCount() != 0 {
		t.Errorf("expected 0 types, got %d", st.TypeCount())
	}
}

func TestSymbolTable_AddAndLookupElement(t *testing.T) {
	st := NewSymbolTable()
	elem := &xsd.Element{Name: "root", Namespace: "http://example.com"}
	st.AddElement(elem)

	name := xsd.NewQName("http://example.com", "root")
	got := st.LookupElement(name)
	if got == nil {
		t.Fatal("expected to find root element")
	}
	if got.Name != "root" {
		t.Errorf("expected root, got %s", got.Name)
	}
}

func TestSymbolTable_TypeCount(t *testing.T) {
	st := NewSymbolTable()
	st.AddType(&xsd.SimpleType{Name: xsd.NewQName("ns", "A")})
	st.AddType(&xsd.SimpleType{Name: xsd.NewQName("ns", "B")})
	st.AddType(&xsd.ComplexType{Name: xsd.NewQName("ns", "C")})
	if st.TypeCount() != 3 {
		t.Errorf("expected 3, got %d", st.TypeCount())
	}
}
