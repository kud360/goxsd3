package xsd

// SchemaSet is the top-level container for a collection of parsed schemas.
// Schemas are stored in parse order for deterministic iteration; the index
// maps provide O(1) lookup and are never iterated.
type SchemaSet struct {
	Schemas     []*Schema       // parse order
	byNamespace map[string]*Schema
	allTypes    map[QName]Type
	allElements map[QName]*Element
}

// NewSchemaSet creates an empty SchemaSet.
func NewSchemaSet() *SchemaSet {
	return &SchemaSet{
		byNamespace: make(map[string]*Schema),
		allTypes:    make(map[QName]Type),
		allElements: make(map[QName]*Element),
	}
}

// AddSchema registers a schema in the set, indexing it by target namespace.
func (ss *SchemaSet) AddSchema(s *Schema) {
	ss.Schemas = append(ss.Schemas, s)
	if s.TargetNamespace != "" {
		ss.byNamespace[s.TargetNamespace] = s
	}
}

// SchemaByNamespace returns the schema for the given namespace, or nil.
func (ss *SchemaSet) SchemaByNamespace(ns string) *Schema {
	return ss.byNamespace[ns]
}

// RegisterType adds a type to the cross-schema index.
func (ss *SchemaSet) RegisterType(t Type) {
	ss.allTypes[t.TypeName()] = t
}

// LookupType returns the type with the given qualified name, or nil.
func (ss *SchemaSet) LookupType(name QName) Type {
	return ss.allTypes[name]
}

// RegisterElement adds an element to the cross-schema index.
func (ss *SchemaSet) RegisterElement(e *Element) {
	ss.allElements[NewQName(e.Namespace, e.Name)] = e
}

// LookupElement returns the element with the given qualified name, or nil.
func (ss *SchemaSet) LookupElement(name QName) *Element {
	return ss.allElements[name]
}

// Schema represents a single parsed XSD document. Slices preserve document
// order; index maps provide O(1) lookup and are never iterated.
type Schema struct {
	TargetNamespace string
	Location        string            // source file path or URI
	Namespaces      map[string]string // prefix → URI (lookup only)
	Elements        []*Element
	Types           []Type
	Groups          []*Group
	AttributeGroups []*AttributeGroup
	Imports         []*Import
	Includes        []*Include
	Annotations     []*Annotation

	// Internal indexes — lookup only, never iterated.
	elementIndex map[string]*Element
	typeIndex    map[QName]Type
	groupIndex   map[string]*Group
}

// NewSchema creates an empty Schema for the given target namespace.
func NewSchema(targetNS string) *Schema {
	return &Schema{
		TargetNamespace: targetNS,
		Namespaces:      make(map[string]string),
		elementIndex:    make(map[string]*Element),
		typeIndex:       make(map[QName]Type),
		groupIndex:      make(map[string]*Group),
	}
}

// AddElement appends an element and indexes it by local name.
func (s *Schema) AddElement(e *Element) {
	s.Elements = append(s.Elements, e)
	s.elementIndex[e.Name] = e
}

// LookupElement returns the element with the given local name, or nil.
func (s *Schema) LookupElement(name string) *Element {
	return s.elementIndex[name]
}

// AddType appends a type and indexes it by qualified name.
func (s *Schema) AddType(t Type) {
	s.Types = append(s.Types, t)
	s.typeIndex[t.TypeName()] = t
}

// LookupType returns the type with the given qualified name, or nil.
func (s *Schema) LookupType(name QName) Type {
	return s.typeIndex[name]
}

// AddGroup appends a group and indexes it by name.
func (s *Schema) AddGroup(g *Group) {
	s.Groups = append(s.Groups, g)
	s.groupIndex[g.Name] = g
}

// LookupGroup returns the group with the given name, or nil.
func (s *Schema) LookupGroup(name string) *Group {
	return s.groupIndex[name]
}

// Element represents an xs:element declaration.
type Element struct {
	Name              string
	Namespace         string // target namespace this element belongs to
	Type              TypeRef
	MinOccurs         int
	MaxOccurs         int // -1 = unbounded
	Nillable          bool
	Abstract          bool
	Default           *string
	Fixed             *string
	SubstitutionGroup *QName
	InlineType        Type // anonymous type defined inline
	Annotations       []*Annotation
	Location          Location
}

// Attribute represents an xs:attribute declaration.
type Attribute struct {
	Name        string
	Type        TypeRef
	Use         AttributeUse
	Default     *string
	Fixed       *string
	Inheritable bool // XSD 1.1
	Annotations []*Annotation
	Location    Location
}

// Import represents an xs:import directive.
type Import struct {
	Namespace      string
	SchemaLocation string
	Annotations    []*Annotation
	Location       Location
}

// Include represents an xs:include directive.
type Include struct {
	SchemaLocation string
	Annotations    []*Annotation
	Location       Location
}

// Annotation represents an xs:annotation element containing documentation
// or application information.
type Annotation struct {
	Documentation []string // xs:documentation content
	AppInfo       []string // xs:appinfo content
}

// Group represents an xs:group definition (model group).
type Group struct {
	Name       string
	Compositor Compositor
	Location   Location
}

// GroupRef is a reference to a named group.
type GroupRef struct {
	Ref       QName
	MinOccurs int
	MaxOccurs int // -1 = unbounded
}

func (GroupRef) isParticle() {}

// AttributeGroup represents an xs:attributeGroup definition.
type AttributeGroup struct {
	Name       string
	Attributes []*Attribute
	Location   Location
}

// AttributeGroupRef is a reference to a named attribute group.
type AttributeGroupRef struct {
	Ref QName
}

// Any represents an xs:any wildcard.
type Any struct {
	Namespace       string // "##any", "##other", "##local", "##targetNamespace", or URI list
	ProcessContents string // "strict", "lax", "skip"
	MinOccurs       int
	MaxOccurs       int // -1 = unbounded
}

func (Any) isParticle() {}

// AnyAttribute represents an xs:anyAttribute wildcard.
type AnyAttribute struct {
	Namespace       string
	ProcessContents string
}
