package parser

import (
	"github.com/kud360/goxsd3/xsd"
)

// SymbolTable provides O(1) lookup of types and elements by QName.
// Types are registered as they are parsed so that backward references
// resolve immediately. Forward references are collected in a pending
// list and resolved after the token loop completes.
type SymbolTable struct {
	types    map[xsd.QName]xsd.Type
	elements map[xsd.QName]*xsd.Element
}

// NewSymbolTable creates an empty SymbolTable.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		types:    make(map[xsd.QName]xsd.Type),
		elements: make(map[xsd.QName]*xsd.Element),
	}
}

// AddType registers a type definition. If the type has an empty name
// (anonymous), it is not indexed.
func (st *SymbolTable) AddType(t xsd.Type) {
	name := t.TypeName()
	if name.Local == "" {
		return
	}
	st.types[name] = t
}

// LookupType returns the type with the given QName, or nil.
func (st *SymbolTable) LookupType(name xsd.QName) xsd.Type {
	return st.types[name]
}

// AddElement registers a top-level element definition.
func (st *SymbolTable) AddElement(e *xsd.Element) {
	name := xsd.NewQName(e.Namespace, e.Name)
	if name.Local == "" {
		return
	}
	st.elements[name] = e
}

// LookupElement returns the element with the given QName, or nil.
func (st *SymbolTable) LookupElement(name xsd.QName) *xsd.Element {
	return st.elements[name]
}

// TypeCount returns the number of registered types.
func (st *SymbolTable) TypeCount() int {
	return len(st.types)
}

// pendingRef represents a type reference that could not be resolved
// immediately during parsing (forward reference). After the token loop
// completes, the parser sweeps through pending refs and resolves them.
type pendingRef struct {
	ref    *xsd.TypeRef
	qname  xsd.QName
}
