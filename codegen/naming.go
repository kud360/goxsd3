// Package codegen provides XSD-to-Go code generation facilities.
package codegen

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/kud360/goxsd3/xsd"
)

// namingSource identifies how a Go name was derived.
type namingSource int

const (
	sourceNamed      namingSource = iota // explicit XSD name attribute
	sourceElement                        // derived from parent element name
	sourceAttribute                      // derived from parent attribute name
	sourceCompositor                     // derived from compositor context
	sourceConflict                       // renamed due to conflict
)

// namedEntry records the assignment of a Go name and how it was derived.
type namedEntry struct {
	GoName  string
	Source  namingSource
	XSDPath []string // element/type path from schema root
}

// Namer assigns stable, deterministic Go names to all types in a schema set.
// It walks the model in document order and uses conflict resolution to ensure
// unique names.
type Namer struct {
	registry  map[string]namedEntry // Go name → first assignment
	usedNames map[string]bool       // all assigned names (quick check)
	logger    *slog.Logger
}

// NewNamer creates a Namer with the given logger.
func NewNamer(logger *slog.Logger) *Namer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Namer{
		registry:  make(map[string]namedEntry),
		usedNames: make(map[string]bool),
		logger:    logger,
	}
}

// NameMap provides O(1) lookup from any Type or Element to its assigned Go name.
type NameMap struct {
	types     map[xsd.Type]string
	elements  map[*xsd.Element]string
	typeOrder []TypeNamePair // preserves assignment order
}

// TypeName returns the Go name assigned to the given type.
func (m *NameMap) TypeName(t xsd.Type) string {
	return m.types[t]
}

// ElementName returns the Go name assigned to the given element.
func (m *NameMap) ElementName(e *xsd.Element) string {
	return m.elements[e]
}

// AllTypeNames returns a slice of (Type, GoName) pairs in assignment order.
// This is useful for tests and determinism checks. The order matches the
// insertion order into the internal map via document-order walking.
func (m *NameMap) AllTypeNames() []TypeNamePair {
	return m.typeOrder
}

// TypeNamePair associates a type with its assigned Go name.
type TypeNamePair struct {
	Type   xsd.Type
	GoName string
}

// nameMapBuilder preserves insertion order while building the NameMap.
type nameMapBuilder struct {
	nm NameMap
}

func newNameMapBuilder() *nameMapBuilder {
	return &nameMapBuilder{
		nm: NameMap{
			types:    make(map[xsd.Type]string),
			elements: make(map[*xsd.Element]string),
		},
	}
}

func (b *nameMapBuilder) setType(t xsd.Type, name string) {
	b.nm.types[t] = name
	b.nm.typeOrder = append(b.nm.typeOrder, TypeNamePair{Type: t, GoName: name})
}

func (b *nameMapBuilder) setElement(e *xsd.Element, name string) {
	b.nm.elements[e] = name
}

// AssignNames walks the schema set in document order and assigns Go names
// to all types and elements. Returns a NameMap for code generation.
func (n *Namer) AssignNames(ss *xsd.SchemaSet) (*NameMap, error) {
	builder := newNameMapBuilder()

	for _, schema := range ss.Schemas {
		// Walk top-level types in document order.
		for _, t := range schema.Types {
			n.assignTypeName(t, nil, builder)
		}
		// Walk top-level elements in document order.
		for _, elem := range schema.Elements {
			name := exportedName(elem.Name)
			builder.setElement(elem, name)
			// If the element has an inline type, name it.
			if elem.InlineType != nil {
				n.assignTypeName(elem.InlineType, []string{elem.Name}, builder)
			}
		}
	}

	return &builder.nm, nil
}

// assignTypeName assigns a Go name to a type, handling named vs anonymous types,
// and recursing into nested content to find further anonymous types.
func (n *Namer) assignTypeName(t xsd.Type, path []string, builder *nameMapBuilder) {
	switch typ := t.(type) {
	case *xsd.ComplexType:
		goName := n.resolveTypeName(typ.Name, path)
		builder.setType(typ, goName)

		// Recurse into content to find nested anonymous types.
		n.walkContent(typ.Content, path, typ.Name, builder)

	case *xsd.SimpleType:
		goName := n.resolveTypeName(typ.Name, path)
		builder.setType(typ, goName)
	}
}

// resolveTypeName determines the Go name for a type given its QName and context path.
func (n *Namer) resolveTypeName(name xsd.QName, path []string) string {
	var baseName string
	var source namingSource

	if name.Local != "" {
		// Named type — use the XSD name directly.
		baseName = exportedName(name.Local)
		source = sourceNamed
	} else if len(path) > 0 {
		// Anonymous type — derive from context path.
		baseName = deriveNameFromPath(path)
		source = sourceElement
	} else {
		// Fallback — shouldn't happen in valid schemas.
		baseName = "AnonymousType"
		source = sourceCompositor
	}

	// Resolve conflicts.
	finalName := n.resolveConflict(baseName, path, source)
	return finalName
}

// resolveConflict ensures uniqueness by trying progressively more qualified names.
func (n *Namer) resolveConflict(baseName string, path []string, source namingSource) string {
	// Try the base name first.
	if !n.usedNames[baseName] {
		n.register(baseName, source, path)
		return baseName
	}

	// If this is a named type colliding with a named type (cross-namespace),
	// or an anonymous type colliding, try qualifying with parent context.
	if len(path) >= 2 {
		qualified := exportedName(path[len(path)-2]) + exportedName(path[len(path)-1]) + "Type"
		if !n.usedNames[qualified] {
			n.register(qualified, sourceConflict, path)
			return qualified
		}
	}

	// Try qualifying with grandparent.
	if len(path) >= 3 {
		qualified := exportedName(path[len(path)-3]) + exportedName(path[len(path)-2]) + exportedName(path[len(path)-1]) + "Type"
		if !n.usedNames[qualified] {
			n.register(qualified, sourceConflict, path)
			return qualified
		}
	}

	// Last resort: numeric suffix.
	for i := 2; ; i++ {
		suffixed := fmt.Sprintf("%s%d", baseName, i)
		if !n.usedNames[suffixed] {
			n.register(suffixed, sourceConflict, path)
			return suffixed
		}
	}
}

func (n *Namer) register(goName string, source namingSource, path []string) {
	n.usedNames[goName] = true
	n.registry[goName] = namedEntry{
		GoName:  goName,
		Source:  source,
		XSDPath: append([]string{}, path...), // defensive copy
	}
	n.logger.Debug("assigned name",
		slog.String("goName", goName),
		slog.Any("path", path))
}

// walkContent walks a Content node looking for elements with inline types.
func (n *Namer) walkContent(content xsd.Content, parentPath []string, parentName xsd.QName, builder *nameMapBuilder) {
	if content == nil {
		return
	}

	switch c := content.(type) {
	case *xsd.Sequence:
		n.walkParticles(c.Items, parentPath, parentName, builder)
	case *xsd.Choice:
		n.walkParticles(c.Items, parentPath, parentName, builder)
	case *xsd.All:
		n.walkParticles(c.Items, parentPath, parentName, builder)
	case *xsd.SimpleContent:
		// No nested elements in simple content.
	case *xsd.ComplexContent:
		if c.Extension != nil && c.Extension.Compositor != nil {
			n.walkContent(c.Extension.Compositor, parentPath, parentName, builder)
		}
		if c.Restriction != nil && c.Restriction.Content != nil {
			n.walkContent(c.Restriction.Content, parentPath, parentName, builder)
		}
	}
}

// walkParticles walks compositor particles looking for elements with inline types.
func (n *Namer) walkParticles(particles []xsd.Particle, parentPath []string, parentName xsd.QName, builder *nameMapBuilder) {
	for _, p := range particles {
		switch item := p.(type) {
		case *xsd.Element:
			if item.InlineType != nil {
				elemPath := appendPath(parentPath, item.Name)
				n.assignTypeName(item.InlineType, elemPath, builder)
			}
		case *xsd.Sequence:
			n.walkParticles(item.Items, parentPath, parentName, builder)
		case *xsd.Choice:
			n.walkParticles(item.Items, parentPath, parentName, builder)
		case *xsd.All:
			n.walkParticles(item.Items, parentPath, parentName, builder)
		}
	}
}

// deriveNameFromPath creates a Go type name from a context path.
// e.g., ["order", "item"] → "OrderItemType"
// e.g., ["person"] → "PersonType"
func deriveNameFromPath(path []string) string {
	var sb strings.Builder
	for _, p := range path {
		sb.WriteString(exportedName(p))
	}
	sb.WriteString("Type")
	return sb.String()
}

// exportedName converts a name to an exported Go identifier by uppercasing
// the first letter of each segment.
func exportedName(name string) string {
	if name == "" {
		return ""
	}

	var sb strings.Builder
	upper := true
	for _, r := range name {
		if r == '_' || r == '-' || r == '.' {
			upper = true
			continue
		}
		if upper {
			sb.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// appendPath creates a new path slice with the extra segment appended.
func appendPath(parent []string, segment string) []string {
	result := make([]string, len(parent)+1)
	copy(result, parent)
	result[len(parent)] = segment
	return result
}
