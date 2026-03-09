package xsd

// Type is the interface implemented by all XSD type definitions.
type Type interface {
	TypeName() QName
	isType()
}

// Content is the interface for complex type content models.
type Content interface {
	isContent()
}

// SimpleType represents an xs:simpleType definition, derived by restriction,
// list, or union.
type SimpleType struct {
	Name        QName
	Restriction *Restriction
	List        *ListType
	Union       *UnionType
	Annotations []*Annotation
	Location    Location
}

func (t *SimpleType) TypeName() QName { return t.Name }
func (*SimpleType) isType()           {}

// ComplexType represents an xs:complexType definition.
type ComplexType struct {
	Name            QName
	Abstract        bool
	Mixed           bool
	Content         Content // SimpleContent, ComplexContent, or direct compositor
	Attributes      []*Attribute
	AttributeGroups []*AttributeGroupRef
	AnyAttribute    *AnyAttribute
	Assertions      []*Assertion // XSD 1.1
	Annotations     []*Annotation
	Location        Location
}

func (t *ComplexType) TypeName() QName { return t.Name }
func (*ComplexType) isType()           {}

// SimpleContent represents xs:simpleContent within a complex type.
type SimpleContent struct {
	Restriction *Restriction
	Extension   *Extension
}

func (*SimpleContent) isContent() {}

// ComplexContent represents xs:complexContent within a complex type.
type ComplexContent struct {
	Mixed       bool
	Restriction *Restriction
	Extension   *Extension
}

func (*ComplexContent) isContent() {}

// Extension represents an xs:extension derivation.
type Extension struct {
	Base            TypeRef
	Compositor      Compositor // additional content model
	Attributes      []*Attribute
	AttributeGroups []*AttributeGroupRef
	AnyAttribute    *AnyAttribute
}

// Alternative represents an xs:alternative element (XSD 1.1 conditional
// type assignment).
type Alternative struct {
	Test string // XPath expression
	Type TypeRef
}
