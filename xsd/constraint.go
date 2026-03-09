package xsd

// Restriction represents an xs:restriction derivation, applicable to both
// simple and complex types.
type Restriction struct {
	Base    TypeRef
	Facets  []Facet
	Content Content // for complex type restriction (inline compositor)
}

// ListType represents an xs:list derivation of a simple type.
type ListType struct {
	ItemType TypeRef
}

// UnionType represents an xs:union derivation of a simple type.
type UnionType struct {
	MemberTypes []TypeRef
}

// Assertion represents an xs:assert element (XSD 1.1).
type Assertion struct {
	Test        string // XPath expression
	Annotations []*Annotation
}
