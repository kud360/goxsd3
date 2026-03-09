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

// BuiltinRegistry is a registry of all 49 built-in XSD types.
type BuiltinRegistry struct {
	types map[QName]*BuiltinTypeInfo
}

// NewBuiltinRegistry creates a registry populated with all 49 built-in XSD types.
func NewBuiltinRegistry() *BuiltinRegistry {
	r := &BuiltinRegistry{
		types: make(map[QName]*BuiltinTypeInfo, 49),
	}

	// Properties by family.
	stringProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	booleanProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityFinite, Numeric: false}
	decimalProps := TypeProperties{Ordered: OrderedTotal, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	integerProps := TypeProperties{Ordered: OrderedTotal, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	boundedIntProps := TypeProperties{Ordered: OrderedTotal, Bounded: true, Cardinality: CardinalityCountablyInfinite, Numeric: true}
	floatProps := TypeProperties{Ordered: OrderedPartial, Bounded: true, Cardinality: CardinalityFinite, Numeric: true}
	durationProps := TypeProperties{Ordered: OrderedPartial, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	dateTimeProps := TypeProperties{Ordered: OrderedPartial, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	binaryProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	uriProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	qnameProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}
	listProps := TypeProperties{Ordered: OrderedFalse, Bounded: false, Cardinality: CardinalityCountablyInfinite, Numeric: false}

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
	register("string", "string", base("anyAtomicType"), "string", stringProps, WhiteSpacePreserve)
	register("normalizedString", "string", base("string"), "string", stringProps, WhiteSpaceReplace)
	register("token", "string", base("normalizedString"), "string", stringProps, WhiteSpaceCollapse)
	register("language", "string", base("token"), "string", stringProps, WhiteSpaceCollapse)
	register("NMTOKEN", "string", base("token"), "string", stringProps, WhiteSpaceCollapse)
	register("Name", "string", base("token"), "string", stringProps, WhiteSpaceCollapse)
	register("NCName", "string", base("Name"), "string", stringProps, WhiteSpaceCollapse)
	register("ID", "string", base("NCName"), "string", stringProps, WhiteSpaceCollapse)
	register("IDREF", "string", base("NCName"), "string", stringProps, WhiteSpaceCollapse)
	register("ENTITY", "string", base("NCName"), "string", stringProps, WhiteSpaceCollapse)

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
	register("duration", "string", base("anyAtomicType"), "duration", durationProps, WhiteSpaceCollapse)
	register("yearMonthDuration", "string", base("duration"), "duration", durationProps, WhiteSpaceCollapse)
	register("dayTimeDuration", "string", base("duration"), "duration", durationProps, WhiteSpaceCollapse)

	// --- date/time ---
	register("dateTime", "string", base("anyAtomicType"), "dateTime", dateTimeProps, WhiteSpaceCollapse)
	register("dateTimeStamp", "string", base("dateTime"), "dateTime", dateTimeProps, WhiteSpaceCollapse)
	register("time", "string", base("anyAtomicType"), "time", dateTimeProps, WhiteSpaceCollapse)
	register("date", "string", base("anyAtomicType"), "date", dateTimeProps, WhiteSpaceCollapse)
	register("gYearMonth", "string", base("anyAtomicType"), "gYearMonth", dateTimeProps, WhiteSpaceCollapse)
	register("gYear", "string", base("anyAtomicType"), "gYear", dateTimeProps, WhiteSpaceCollapse)
	register("gMonthDay", "string", base("anyAtomicType"), "gMonthDay", dateTimeProps, WhiteSpaceCollapse)
	register("gDay", "string", base("anyAtomicType"), "gDay", dateTimeProps, WhiteSpaceCollapse)
	register("gMonth", "string", base("anyAtomicType"), "gMonth", dateTimeProps, WhiteSpaceCollapse)

	// --- binary ---
	register("hexBinary", "[]byte", base("anyAtomicType"), "hexBinary", binaryProps, WhiteSpaceCollapse)
	register("base64Binary", "[]byte", base("anyAtomicType"), "base64Binary", binaryProps, WhiteSpaceCollapse)

	// --- anyURI ---
	register("anyURI", "string", base("anyAtomicType"), "anyURI", uriProps, WhiteSpaceCollapse)

	// --- QName / NOTATION ---
	register("QName", "string", base("anyAtomicType"), "QName", qnameProps, WhiteSpaceCollapse)
	register("NOTATION", "string", base("anyAtomicType"), "NOTATION", qnameProps, WhiteSpaceCollapse)

	// --- list types ---
	register("NMTOKENS", "[]string", base("anySimpleType"), "list", listProps, WhiteSpaceCollapse)
	register("IDREFS", "[]string", base("anySimpleType"), "list", listProps, WhiteSpaceCollapse)
	register("ENTITIES", "[]string", base("anySimpleType"), "list", listProps, WhiteSpaceCollapse)

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
