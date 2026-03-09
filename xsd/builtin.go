package xsd

import "fmt"

// BuiltinTypeInfo describes a built-in XSD type including its Go mapping,
// derivation base, facet applicability, and fundamental facet properties.
type BuiltinTypeInfo struct {
	Name             QName
	GoType           string
	Base             *QName
	Family           string // type family for facet lookup (e.g., "string", "decimal")
	ApplicableFacets []FacetKind
	Properties       TypeProperties
	WhiteSpace       WhiteSpaceRule
}

// TypeProperties holds the four fundamental facets defined by the XSD spec.
type TypeProperties struct {
	Ordered     Ordered
	Bounded     bool
	Cardinality Cardinality
	Numeric     bool
}

// BuiltinRegistry is a registry of all 50 built-in XSD types.
type BuiltinRegistry struct {
	types map[QName]*BuiltinTypeInfo
}

// NewBuiltinRegistry creates a registry populated with all 50 built-in XSD
// types (2 ur-types + 1 XSD 1.1 intermediate + 19 primitives + 25 derived + 3 list types).
func NewBuiltinRegistry() *BuiltinRegistry {
	r := &BuiltinRegistry{
		types: make(map[QName]*BuiltinTypeInfo, 50),
	}

	// Fundamental facet property sets, grouped by shared characteristics.
	unorderedProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	booleanProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityFinite, Numeric: false}
	decimalProps := TypeProperties{Ordered: OrderedTotal, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	integerProps := TypeProperties{Ordered: OrderedTotal, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	boundedIntProps := TypeProperties{Ordered: OrderedTotal, Bounded: true, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	floatProps := TypeProperties{Ordered: OrderedPartial, Bounded: true, Cardinality: CardinalityFinite, Numeric: true}
	partialOrderProps := TypeProperties{Ordered: OrderedPartial, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}

	// Helper to create a QName pointer.
	base := func(local string) *QName {
		q := XSDName(local)
		return &q
	}

	// register adds a type to the registry.
	register := func(local, goType string, baseType *QName, family string, props TypeProperties, ws WhiteSpaceRule) {
		name := XSDName(local)
		info := &BuiltinTypeInfo{
			Name:             name,
			GoType:           goType,
			Base:             baseType,
			Family:           family,
			ApplicableFacets: FacetApplicability(family),
			Properties:       props,
			WhiteSpace:       ws,
		}
		r.types[name] = info
	}

	// --- ur-types ---
	register("anyType", "any", nil, "", TypeProperties{}, WhiteSpacePreserve)
	register("anySimpleType", "string", base("anyType"), "", TypeProperties{}, WhiteSpacePreserve)
	register("anyAtomicType", "string", base("anySimpleType"), "", TypeProperties{}, WhiteSpacePreserve)

	// --- string family ---
	register("string", "string", base("anyAtomicType"), "string", unorderedProps, WhiteSpacePreserve)
	register("normalizedString", "string", base("string"), "string", unorderedProps, WhiteSpaceReplace)
	register("token", "string", base("normalizedString"), "string", unorderedProps, WhiteSpaceCollapse)
	register("language", "string", base("token"), "string", unorderedProps, WhiteSpaceCollapse)
	register("NMTOKEN", "string", base("token"), "string", unorderedProps, WhiteSpaceCollapse)
	register("Name", "string", base("token"), "string", unorderedProps, WhiteSpaceCollapse)
	register("NCName", "string", base("Name"), "string", unorderedProps, WhiteSpaceCollapse)
	register("ID", "string", base("NCName"), "string", unorderedProps, WhiteSpaceCollapse)
	register("IDREF", "string", base("NCName"), "string", unorderedProps, WhiteSpaceCollapse)
	register("ENTITY", "string", base("NCName"), "string", unorderedProps, WhiteSpaceCollapse)

	// --- boolean ---
	register("boolean", "bool", base("anyAtomicType"), "boolean", booleanProps, WhiteSpaceCollapse)

	// --- decimal / integer family ---
	register("decimal", "float64", base("anyAtomicType"), "decimal", decimalProps, WhiteSpaceCollapse)
	register("integer", "int64", base("decimal"), "decimal", integerProps, WhiteSpaceCollapse)
	register("nonPositiveInteger", "int64", base("integer"), "decimal", integerProps, WhiteSpaceCollapse)
	register("negativeInteger", "int64", base("nonPositiveInteger"), "decimal", integerProps, WhiteSpaceCollapse)
	register("long", "int64", base("integer"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("int", "int32", base("long"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("short", "int16", base("int"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("byte", "int8", base("short"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("nonNegativeInteger", "uint64", base("integer"), "decimal", integerProps, WhiteSpaceCollapse)
	register("unsignedLong", "uint64", base("nonNegativeInteger"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("unsignedInt", "uint32", base("unsignedLong"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("unsignedShort", "uint16", base("unsignedInt"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("unsignedByte", "uint8", base("unsignedShort"), "decimal", boundedIntProps, WhiteSpaceCollapse)
	register("positiveInteger", "uint64", base("nonNegativeInteger"), "decimal", integerProps, WhiteSpaceCollapse)

	// --- float / double ---
	register("float", "float32", base("anyAtomicType"), "float", floatProps, WhiteSpaceCollapse)
	register("double", "float64", base("anyAtomicType"), "double", floatProps, WhiteSpaceCollapse)

	// --- duration ---
	register("duration", "string", base("anyAtomicType"), "duration", partialOrderProps, WhiteSpaceCollapse)
	register("yearMonthDuration", "string", base("duration"), "duration", partialOrderProps, WhiteSpaceCollapse)
	register("dayTimeDuration", "string", base("duration"), "duration", partialOrderProps, WhiteSpaceCollapse)

	// --- date/time ---
	register("dateTime", "string", base("anyAtomicType"), "dateTime", partialOrderProps, WhiteSpaceCollapse)
	register("dateTimeStamp", "string", base("dateTime"), "dateTime", partialOrderProps, WhiteSpaceCollapse)
	register("time", "string", base("anyAtomicType"), "time", partialOrderProps, WhiteSpaceCollapse)
	register("date", "string", base("anyAtomicType"), "date", partialOrderProps, WhiteSpaceCollapse)
	register("gYearMonth", "string", base("anyAtomicType"), "gYearMonth", partialOrderProps, WhiteSpaceCollapse)
	register("gYear", "string", base("anyAtomicType"), "gYear", partialOrderProps, WhiteSpaceCollapse)
	register("gMonthDay", "string", base("anyAtomicType"), "gMonthDay", partialOrderProps, WhiteSpaceCollapse)
	register("gDay", "string", base("anyAtomicType"), "gDay", partialOrderProps, WhiteSpaceCollapse)
	register("gMonth", "string", base("anyAtomicType"), "gMonth", partialOrderProps, WhiteSpaceCollapse)

	// --- binary ---
	register("hexBinary", "[]byte", base("anyAtomicType"), "hexBinary", unorderedProps, WhiteSpaceCollapse)
	register("base64Binary", "[]byte", base("anyAtomicType"), "base64Binary", unorderedProps, WhiteSpaceCollapse)

	// --- anyURI ---
	register("anyURI", "string", base("anyAtomicType"), "anyURI", unorderedProps, WhiteSpaceCollapse)

	// --- QName / NOTATION ---
	register("QName", "string", base("anyAtomicType"), "QName", unorderedProps, WhiteSpaceCollapse)
	register("NOTATION", "string", base("anyAtomicType"), "NOTATION", unorderedProps, WhiteSpaceCollapse)

	// --- list types ---
	register("NMTOKENS", "[]string", base("anySimpleType"), "list", unorderedProps, WhiteSpaceCollapse)
	register("IDREFS", "[]string", base("anySimpleType"), "list", unorderedProps, WhiteSpaceCollapse)
	register("ENTITIES", "[]string", base("anySimpleType"), "list", unorderedProps, WhiteSpaceCollapse)

	return r
}

// Lookup returns the BuiltinTypeInfo for the given QName, or nil if not found.
func (r *BuiltinRegistry) Lookup(name QName) *BuiltinTypeInfo {
	return r.types[name]
}

// ApplicableFacets returns the applicable facet kinds for the given type,
// or nil if the type is not found.
func (r *BuiltinRegistry) ApplicableFacets(name QName) []FacetKind {
	info := r.types[name]
	if info == nil {
		return nil
	}
	return info.ApplicableFacets
}

// IsValidRestriction checks that every facet is applicable to the base type.
// It returns an error listing any inapplicable facets, or nil if all are valid.
func (r *BuiltinRegistry) IsValidRestriction(base QName, facets []Facet) error {
	info := r.types[base]
	if info == nil {
		return fmt.Errorf("unknown base type %s", base.Local)
	}

	var bad []FacetKind
	for _, f := range facets {
		if !IsFacetApplicable(info.Family, f.Kind) {
			bad = append(bad, f.Kind)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("facets not applicable to %s (family %q): %v", base.Local, info.Family, bad)
	}
	return nil
}

// GoType returns the Go type string for the given XSD type, or "" if not found.
func (r *BuiltinRegistry) GoType(name QName) string {
	info := r.types[name]
	if info == nil {
		return ""
	}
	return info.GoType
}
