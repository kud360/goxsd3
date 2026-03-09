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

const (
	WhiteSpacePreserve WhiteSpaceRule = "preserve"
	WhiteSpaceReplace  WhiteSpaceRule = "replace"
	WhiteSpaceCollapse WhiteSpaceRule = "collapse"
)

// Ordered represents the ordered property of a primitive type.
type Ordered string

const (
	OrderedFalse   Ordered = "false"
	OrderedPartial Ordered = "partial"
	OrderedTotal   Ordered = "total"
)

// Cardinality represents the cardinality property of a value space.
type Cardinality string

const (
	CardinalityFinite            Cardinality = "finite"
	CardinalityCountablyInfinite Cardinality = "countablyInfinite"
)

// AttributeUse represents how an attribute may appear on an element.
type AttributeUse string

const (
	AttributeOptional   AttributeUse = "optional"
	AttributeRequired   AttributeUse = "required"
	AttributeProhibited AttributeUse = "prohibited"
)

// Location records where a schema component was defined in a source document.
type Location struct {
	SystemID string
	Line     int
	Col      int
	Offset   int64
}
