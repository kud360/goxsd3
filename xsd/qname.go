// Package xsd provides foundational types for representing XSD 1.1 schema
// components in Go.
package xsd

// XSDNS is the canonical namespace URI for XML Schema.
const XSDNS = "http://www.w3.org/2001/XMLSchema"

// QName represents a namespace-qualified name, analogous to xs:QName.
type QName struct {
	Namespace string
	Local     string
}

// String returns the Clark notation representation: {namespace}local.
func (q QName) String() string {
	if q.Namespace == "" {
		return q.Local
	}
	return "{" + q.Namespace + "}" + q.Local
}

// NewQName constructs a QName from a namespace URI and local name.
func NewQName(ns, local string) QName {
	return QName{Namespace: ns, Local: local}
}

// XSDName constructs a QName in the XSD namespace for the given local name.
func XSDName(local string) QName {
	return QName{Namespace: XSDNS, Local: local}
}

// TypeRef is a reference to a type by its qualified name. The Resolved pointer
// is populated during schema resolution to point to the actual type definition.
type TypeRef struct {
	Name     QName
	Resolved *QName
}

// WhiteSpaceRule represents the whiteSpace facet value for simple types.
type WhiteSpaceRule string

// WhiteSpaceRule values as defined by the XSD specification.
const (
	// WhiteSpacePreserve retains all whitespace as-is.
	WhiteSpacePreserve WhiteSpaceRule = "preserve"
	// WhiteSpaceReplace replaces tabs, newlines, and carriage returns with spaces.
	WhiteSpaceReplace WhiteSpaceRule = "replace"
	// WhiteSpaceCollapse replaces, then collapses runs of spaces to a single space,
	// and trims leading/trailing whitespace.
	WhiteSpaceCollapse WhiteSpaceRule = "collapse"
)

// Ordered represents the ordered property of a primitive type.
type Ordered string

// Ordered values for the fundamental ordered facet.
const (
	// OrderedFalse indicates no ordering is defined for the value space.
	OrderedFalse Ordered = "false"
	// OrderedPartial indicates the value space has a partial order.
	OrderedPartial Ordered = "partial"
	// OrderedTotal indicates the value space has a total order.
	OrderedTotal Ordered = "total"
)

// Cardinality represents the cardinality property of a value space.
type Cardinality string

// Cardinality values for the fundamental cardinality facet.
const (
	// CardinalityFinite indicates the value space has a finite number of values.
	CardinalityFinite Cardinality = "finite"
	// CardinalityCountablyInfinite indicates the value space is countably infinite.
	CardinalityCountablyInfinite Cardinality = "countablyInfinite"
)

// AttributeUse represents how an attribute may appear on an element.
type AttributeUse string

// AttributeUse values controlling attribute occurrence on elements.
const (
	// AttributeOptional means the attribute may or may not appear.
	AttributeOptional AttributeUse = "optional"
	// AttributeRequired means the attribute must appear.
	AttributeRequired AttributeUse = "required"
	// AttributeProhibited means the attribute must not appear.
	AttributeProhibited AttributeUse = "prohibited"
)

// Location records where a schema component was defined in a source document.
type Location struct {
	SystemID string
	Line     int
	Col      int
	Offset   int64
}
