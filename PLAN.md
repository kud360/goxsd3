# goxsd3 — XSD 1.1 Parser & Code Generator for Go

## Vision

A Go library and CLI tool that:
1. **Parses** XSD 1.1 schemas into a rich in-memory model (AST)
2. **Generates** idiomatic Go types with `xs:choice` mapped to type switches (interfaces + concrete types)
3. **Optionally generates** JSON, XML, and BER marshallers/unmarshallers
4. **Exposes** the model as a library with hook points for plugins/extensions at every phase

---

## Architecture Overview

**Streaming-first**: The streaming parser is the foundation. The DOM parser is built
on top of it via `CollectingHandler`. This ensures a single XML reading path and
makes the streaming parser the most tested, most exercised code path.

```
                     ┌───────────────────┐
                     │  Built-in Types   │
                     │  (hfp: registry)  │
                     └────────┬──────────┘
                              │ bootstrap
┌──────────────┐     ┌───────▼──────────────────┐     ┌──────────────────┐
│  XSD Files   │────▶│  Streaming Parser         │────▶│   Schema Model   │
│  (.xsd)      │     │  (SAX-style events)       │     │   (AST / IR)     │
└──────────────┘     │                            │     └────────┬─────────┘
       │             │  ┌─ CollectingHandler ──┐  │              │
  ┌────▼────┐        │  │  (builds full DOM)   │  │     ┌────────▼─────────┐
  │ import/ │        │  └──────────────────────┘  │     │     CodeGen      │
  │ include │        │  ┌─ TypeListHandler ────┐  │     └────────┬─────────┘
  └─────────┘        │  │  (fast introspection)│  │              │
                     │  └──────────────────────┘  │     ┌────────┼─────────┐
                     │  ┌─ FilteringHandler ───┐  │     ▼        ▼         ▼
                     │  │  (selective events)  │  │   XML      JSON      BER
                     │  └──────────────────────┘  │  Marshal  Marshal  Marshal
                     └────────────────────────────┘
                              │
                         ┌────▼─────┐
                         │   slog   │  (structured logging throughout)
                         └──────────┘
```

**Key insight**: `parser.Parse()` internally creates a `StreamParser` + `CollectingHandler`,
so the DOM parser is just a convenience wrapper. Users who need streaming get the same
battle-tested tokenizer. The `CollectingHandler` connects references as type definitions
are encountered during the stream, maintaining a symbol table that grows incrementally.

### Package Layout

```
goxsd3/
├── go.mod
├── cmd/
│   └── goxsd3/          # CLI entry point
│       └── main.go
├── xsd/                 # XSD model types (the AST / IR)
│   ├── model.go         # Core schema model types
│   ├── types.go         # Simple/complex type definitions
│   ├── builtin.go       # Built-in type registry (hfp: definitions)
│   ├── builtin_test.go  # Comprehensive built-in type tests
│   ├── facets.go        # Facet definitions, applicability, cross-validation
│   ├── validate.go      # Default/fixed value validation, facet narrowing checks
│   ├── compositor.go    # Sequence, Choice, All (nested support)
│   ├── constraint.go    # Facets, assertions, identity constraints
│   └── namespace.go     # Namespace & import resolution
├── parser/              # XSD parser (XML → model)
│   ├── streaming.go     # Streaming parser (SAX-style event callbacks) — THE FOUNDATION
│   ├── handler.go       # CollectingHandler (stream → DOM), FilteringHandler, etc.
│   ├── parser.go        # DOM parser (convenience wrapper: StreamParser + CollectingHandler)
│   ├── resolve.go       # Type/ref resolution, import/include/redefine
│   ├── import.go        # xs:import handler (cross-namespace)
│   ├── include.go       # xs:include handler (same-namespace)
│   ├── catalog.go       # XML Catalog support for schema resolution
│   ├── options.go       # Parser options (including slog.Logger)
│   └── hooks.go         # Parser hook interfaces
├── codegen/             # Go code generation (model → Go source)
│   ├── codegen.go       # Main code generator
│   ├── naming.go        # Contextual naming system (anonymous type names)
│   ├── types.go         # Type mapping (XSD → Go)
│   ├── choice.go        # Choice → interface + type switch
│   ├── templates/       # text/template files
│   │   ├── struct.go.tmpl
│   │   ├── choice.go.tmpl
│   │   ├── enum.go.tmpl
│   │   └── marshal.go.tmpl
│   ├── options.go       # Codegen options
│   └── hooks.go         # Codegen hook interfaces
├── marshal/             # Marshaller/unmarshaller generation
│   ├── xml.go           # XML marshal/unmarshal codegen
│   ├── json.go          # JSON marshal/unmarshal codegen
│   ├── ber.go           # BER/ASN.1 marshal/unmarshal codegen
│   └── hooks.go         # Marshal hook interfaces
├── config/              # Config file loading
│   ├── config.go        # Load/parse goxsd3.yaml
│   └── validation.go    # ValidationConfig, ValidationLevel, ValidationRule
├── plugin/              # Plugin system
│   ├── plugin.go        # Plugin interface & registry
│   └── loader.go        # Plugin discovery/loading
└── testdata/            # XSD test fixtures
    ├── basic/
    ├── builtin/         # Built-in type tests (all 49 types)
    ├── choice/
    ├── complex/
    ├── derivation/      # SimpleType restriction, ComplexType extension
    ├── imports/         # import/include/redefine multi-file tests
    ├── nested/          # Deeply nested compositor tests
    ├── streaming/       # Streaming parser test fixtures
    ├── xsd11/
    ├── naming/          # Anonymous type naming tests
    ├── w3c/             # W3C XSD Test Suite (XSTS) subset
    └── realworld/       # Real-world schemas (SOAP, GPX, KML, XBRL, etc.)
```

---

## Phase 0: Built-in Type System (hfp: Registry)

The XSD spec defines all 49 built-in types in its own `datatypes.xsd` using the
`hfp:` namespace (`http://www.w3.org/2001/XMLSchema-hasFacetAndProperty`). Our
type system must bootstrap from these definitions so that user-defined types
inherit the correct facet applicability.

### 0.1 The hfp: Mechanism

In the W3C's `datatypes.xsd`, each built-in type is annotated with:
- `hfp:hasFacet name="..."` — declares which constraining facets apply
- `hfp:hasProperty name="..." value="..."` — declares type properties (ordered, bounded, cardinality, numeric)

Example from the spec:
```xml
<xs:simpleType name="string" id="string">
  <xs:annotation>
    <xs:appinfo>
      <hfp:hasFacet name="length"/>
      <hfp:hasFacet name="minLength"/>
      <hfp:hasFacet name="maxLength"/>
      <hfp:hasFacet name="pattern"/>
      <hfp:hasFacet name="enumeration"/>
      <hfp:hasFacet name="whiteSpace"/>
    </xs:appinfo>
  </xs:annotation>
  <xs:restriction base="xs:anySimpleType">
    <xs:whiteSpace value="preserve"/>
  </xs:restriction>
</xs:simpleType>
```

### 0.2 Built-in Type Registry (`xsd/builtin.go`)

```go
package xsd

// BuiltinTypeInfo describes a built-in XSD type's facet support and properties.
type BuiltinTypeInfo struct {
    Name           QName
    GoType         string          // Go type mapping (string, int64, float64, etc.)
    Base           *QName          // Parent type in derivation hierarchy (nil for anyType)
    ApplicableFacets []FacetKind   // From hfp:hasFacet
    Properties     TypeProperties  // From hfp:hasProperty
    WhiteSpace     WhiteSpaceRule  // Inherited or fixed
}

type TypeProperties struct {
    Ordered     Ordered     // false, partial, total
    Bounded     bool
    Cardinality Cardinality // finite, countably infinite
    Numeric     bool
}

// BuiltinRegistry holds all 49 built-in types and their derivation chains.
type BuiltinRegistry struct {
    types map[QName]*BuiltinTypeInfo
}

// NewBuiltinRegistry creates a registry pre-populated with all XSD built-in types.
func NewBuiltinRegistry() *BuiltinRegistry

// Lookup returns type info for a built-in type (nil if not built-in).
func (r *BuiltinRegistry) Lookup(name QName) *BuiltinTypeInfo

// ApplicableFacets returns all facets applicable to a type, including inherited.
func (r *BuiltinRegistry) ApplicableFacets(name QName) []FacetKind

// IsValidRestriction checks if applying the given facets to derive from base is valid.
func (r *BuiltinRegistry) IsValidRestriction(base QName, facets []Facet) error

// GoType returns the Go type for a built-in XSD type.
func (r *BuiltinRegistry) GoType(name QName) string
```

### 0.3 Complete Built-in Type Hierarchy

**Total: 49 named types** (2 ur-types + 1 XSD 1.1 intermediate + 19 primitives + 24 derived + 3 list types)

```
anyType                                    (ur-type, root of all types)
├── anySimpleType                          (ur-type for all simple types)
│   ├── anyAtomicType                      (XSD 1.1, base of all 19 primitives)
│   │   ├── string                         ← PRIMITIVE
│   │   │   └── normalizedString           (whiteSpace=replace)
│   │   │       └── token                  (whiteSpace=collapse)
│   │   │           ├── language           (pattern=[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*)
│   │   │           ├── NMTOKEN            (pattern=\c+)
│   │   │           ├── Name               (pattern=...)
│   │   │           │   └── NCName         (pattern=no colons)
│   │   │           │       ├── ID
│   │   │           │       ├── IDREF
│   │   │           │       └── ENTITY
│   │   │           └── (token-derived)
│   │   ├── boolean                        ← PRIMITIVE
│   │   ├── decimal                        ← PRIMITIVE
│   │   │   └── integer                    (fractionDigits=0)
│   │   │       ├── nonPositiveInteger     (maxInclusive=0)
│   │   │       │   └── negativeInteger    (maxInclusive=-1)
│   │   │       ├── long                   (min/maxInclusive=±2^63)
│   │   │       │   └── int               (min/maxInclusive=±2^31)
│   │   │       │       └── short          (min/maxInclusive=±2^15)
│   │   │       │           └── byte       (min/maxInclusive=±2^7)
│   │   │       ├── nonNegativeInteger     (minInclusive=0)
│   │   │       │   ├── unsignedLong       (maxInclusive=2^64-1)
│   │   │       │   │   └── unsignedInt    (maxInclusive=2^32-1)
│   │   │       │   │       └── unsignedShort (maxInclusive=2^16-1)
│   │   │       │   │           └── unsignedByte (maxInclusive=2^8-1)
│   │   │       │   └── positiveInteger    (minInclusive=1)
│   │   │       └── (integer-derived)
│   │   ├── float                          ← PRIMITIVE
│   │   ├── double                         ← PRIMITIVE
│   │   ├── duration                       ← PRIMITIVE
│   │   │   ├── yearMonthDuration          (XSD 1.1, pattern restricts to P...Y...M)
│   │   │   └── dayTimeDuration            (XSD 1.1, pattern restricts to P...DT...H...M...S)
│   │   ├── dateTime                       ← PRIMITIVE
│   │   │   └── dateTimeStamp             (XSD 1.1, explicitTimezone=required)
│   │   ├── time                           ← PRIMITIVE
│   │   ├── date                           ← PRIMITIVE
│   │   ├── gYearMonth                     ← PRIMITIVE
│   │   ├── gYear                          ← PRIMITIVE
│   │   ├── gMonthDay                      ← PRIMITIVE
│   │   ├── gDay                           ← PRIMITIVE
│   │   ├── gMonth                         ← PRIMITIVE
│   │   ├── hexBinary                      ← PRIMITIVE
│   │   ├── base64Binary                   ← PRIMITIVE
│   │   ├── anyURI                         ← PRIMITIVE
│   │   ├── QName                          ← PRIMITIVE
│   │   └── NOTATION                       ← PRIMITIVE
│   │
│   └── Built-in List Types (derived by list, NOT restriction):
│       ├── NMTOKENS                       (list of NMTOKEN, minLength=1)
│       ├── IDREFS                         (list of IDREF, minLength=1)
│       └── ENTITIES                       (list of ENTITY, minLength=1)
│
└── (complex types derive from anyType)
```

### 0.4 Facet Applicability Table

**XSD 1.0 Facets (14 total):**

| Type Family     | length | minLen | maxLen | pattern | enum | whiteSpace | maxIncl | maxExcl | minIncl | minExcl | totalDig | fracDig |
|-----------------|--------|--------|--------|---------|------|------------|---------|---------|---------|---------|----------|---------|
| string          |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓      |         |         |         |         |          |         |
| boolean         |        |        |        |    ✓    |      |     ✓†     |         |         |         |         |          |         |
| decimal         |        |        |        |    ✓    |  ✓   |     ✓†     |    ✓    |    ✓    |    ✓    |    ✓    |    ✓     |    ✓    |
| float/double    |        |        |        |    ✓    |  ✓   |     ✓†     |    ✓    |    ✓    |    ✓    |    ✓    |          |         |
| duration        |        |        |        |    ✓    |  ✓   |     ✓†     |    ✓    |    ✓    |    ✓    |    ✓    |          |         |
| dateTime/date.. |        |        |        |    ✓    |  ✓   |     ✓†     |    ✓    |    ✓    |    ✓    |    ✓    |          |         |
| hexBinary       |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓†     |         |         |         |         |          |         |
| base64Binary    |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓†     |         |         |         |         |          |         |
| anyURI          |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓†     |         |         |         |         |          |         |
| QName/NOTATION  |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓†     |         |         |         |         |          |         |
| integer (deriv) |        |        |        |    ✓    |  ✓   |     ✓†     |    ✓    |    ✓    |    ✓    |    ✓    |    ✓     |    ✓‡   |
| list types      |   ✓    |   ✓    |   ✓    |    ✓    |  ✓   |     ✓†     |         |         |         |         |          |         |

†whiteSpace is fixed at `collapse` for all non-string primitive types (cannot be changed).
‡fractionDigits is fixed to 0 for integer and all integer-derived types.

**XSD 1.1 Additional Facets:**

| Type Family     | explicitTimezone | minScale | maxScale |
|-----------------|------------------|----------|----------|
| dateTime/date.. |        ✓         |          |          |
| decimal         |                  |    ✓     |    ✓     |

**Key rules for facet application:**
- `boolean` does NOT support `enumeration` (already has exactly two values)
- Derived types inherit all facets from their base primitive
- Facets marked `fixed="true"` cannot be overridden in further derivations
- Union types only support: `pattern`, `enumeration`
- List types only support: `length`, `minLength`, `maxLength`, `pattern`, `enumeration`, `whiteSpace`

### 0.4.1 Facet Cross-Validation (`xsd/facets.go`)

Related facets must be validated against each other. A schema that specifies
contradictory facets is invalid and must be rejected at parse time.

**Cross-validation rules:**
```go
// ValidateFacetSet checks that a set of facets is internally consistent.
func ValidateFacetSet(facets []Facet, baseType QName) []error

// Rules enforced:
// 1. minLength ≤ maxLength (if both present)
// 2. minInclusive ≤ maxInclusive (if both present)
// 3. minExclusive < maxExclusive (if both present)
// 4. minInclusive < maxExclusive (if both present) — cannot equal
// 5. minExclusive < maxInclusive (if both present) — cannot equal
// 6. If length is set, minLength and maxLength must not contradict it
//    (length ≥ minLength, length ≤ maxLength, or don't set them)
// 7. totalDigits ≥ fractionDigits (if both present)
// 8. enumeration values must be valid for the base type
//    (e.g., enum value "abc" on xs:integer is invalid)
// 9. pattern must be a valid regex (compile-check at parse time)
// 10. Fixed facets from base cannot be overridden
// 11. Facets can only narrow, never widen:
//     - New minLength ≥ inherited minLength
//     - New maxLength ≤ inherited maxLength
//     - New minInclusive ≥ inherited minInclusive
//     - New maxInclusive ≤ inherited maxInclusive
//     - etc.
```

**Example invalid schemas that MUST be rejected:**
```xml
<!-- minLength > maxLength -->
<xs:simpleType name="Bad1">
  <xs:restriction base="xs:string">
    <xs:minLength value="10"/>
    <xs:maxLength value="5"/>   <!-- ERROR -->
  </xs:restriction>
</xs:simpleType>

<!-- minInclusive > maxInclusive -->
<xs:simpleType name="Bad2">
  <xs:restriction base="xs:integer">
    <xs:minInclusive value="100"/>
    <xs:maxInclusive value="50"/>  <!-- ERROR -->
  </xs:restriction>
</xs:simpleType>

<!-- totalDigits < fractionDigits -->
<xs:simpleType name="Bad3">
  <xs:restriction base="xs:decimal">
    <xs:totalDigits value="3"/>
    <xs:fractionDigits value="5"/>  <!-- ERROR -->
  </xs:restriction>
</xs:simpleType>

<!-- enum value invalid for base type -->
<xs:simpleType name="Bad4">
  <xs:restriction base="xs:integer">
    <xs:enumeration value="abc"/>  <!-- ERROR: not an integer -->
  </xs:restriction>
</xs:simpleType>

<!-- widens inherited facet -->
<xs:simpleType name="Base">
  <xs:restriction base="xs:string">
    <xs:maxLength value="10"/>
  </xs:restriction>
</xs:simpleType>
<xs:simpleType name="Bad5">
  <xs:restriction base="Base">
    <xs:maxLength value="20"/>  <!-- ERROR: widens maxLength -->
  </xs:restriction>
</xs:simpleType>
```

### 0.4.2 Default & Fixed Value Validation (`xsd/validate.go`)

Element and attribute `default` and `fixed` values must be validated against
their declared type at parse time. This catches schema authoring errors early.

```go
// ValidateDefaultValue checks that a default/fixed value is valid for the given type.
func ValidateDefaultValue(value string, typeName QName, registry *BuiltinRegistry) error

// Rules:
// 1. Value must be in the type's value space (e.g., "abc" is not valid for xs:integer)
// 2. Value must satisfy all facets of the type (pattern, enumeration, min/max, etc.)
// 3. For derived types, walk the restriction chain and validate against each level
// 4. For list types, validate each whitespace-separated item against the itemType
// 5. For union types, value must be valid for at least one memberType
```

**Built-in type value parsers** (used for validation):
```go
// Each built-in type family needs a value parser for validation.
type ValueValidator interface {
    // Validate checks if the string is a valid lexical representation.
    Validate(value string) error
}

// Built-in validators:
// - StringValidator: always valid (any string)
// - BooleanValidator: "true", "false", "1", "0"
// - IntegerValidator: optional sign + digits (no decimal point)
// - DecimalValidator: optional sign + digits + optional fractional part
// - FloatValidator: decimal | "INF" | "-INF" | "NaN"
// - DateTimeValidator: ISO 8601 format validation
// - DateValidator, TimeValidator, etc.
// - Base64Validator: valid base64 characters
// - HexBinaryValidator: even number of hex characters
// - AnyURIValidator: IRI validation (RFC 3987)
// - QNameValidator: prefix:localName format
```

**Examples that must be caught:**
```xml
<!-- default value "abc" is not a valid integer -->
<xs:element name="count" type="xs:integer" default="abc"/>  <!-- ERROR -->

<!-- fixed value outside enumeration -->
<xs:simpleType name="Color">
  <xs:restriction base="xs:string">
    <xs:enumeration value="red"/>
    <xs:enumeration value="blue"/>
  </xs:restriction>
</xs:simpleType>
<xs:element name="c" type="Color" fixed="green"/>  <!-- ERROR -->

<!-- default value exceeds maxLength -->
<xs:simpleType name="ShortName">
  <xs:restriction base="xs:string">
    <xs:maxLength value="3"/>
  </xs:restriction>
</xs:simpleType>
<xs:element name="n" type="ShortName" default="Jonathan"/>  <!-- ERROR -->
```

### 0.5 User-Defined Type Derivation

Users create new types by **restricting** built-in types or other user types:

```xml
<!-- SimpleType: restrict string with facets -->
<xs:simpleType name="ZipCode">
  <xs:restriction base="xs:string">
    <xs:pattern value="\d{5}(-\d{4})?"/>
    <xs:maxLength value="10"/>
  </xs:restriction>
</xs:simpleType>

<!-- SimpleType: restrict integer with range -->
<xs:simpleType name="Percentage">
  <xs:restriction base="xs:integer">
    <xs:minInclusive value="0"/>
    <xs:maxInclusive value="100"/>
  </xs:restriction>
</xs:simpleType>

<!-- SimpleType: list of tokens -->
<xs:simpleType name="ColorList">
  <xs:list itemType="xs:token"/>
</xs:simpleType>

<!-- SimpleType: union of types -->
<xs:simpleType name="SizeOrName">
  <xs:union memberTypes="xs:integer xs:string"/>
</xs:simpleType>

<!-- ComplexType: extend a base with new elements -->
<xs:complexType name="USAddress">
  <xs:complexContent>
    <xs:extension base="AddressType">
      <xs:sequence>
        <xs:element name="state" type="xs:string"/>
        <xs:element name="zip" type="ZipCode"/>
      </xs:sequence>
    </xs:extension>
  </xs:complexContent>
</xs:complexType>

<!-- ComplexType: restrict a base (narrow constraints) -->
<xs:complexType name="SmallOrder">
  <xs:complexContent>
    <xs:restriction base="OrderType">
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="5"/>
      </xs:sequence>
    </xs:restriction>
  </xs:complexContent>
</xs:complexType>
```

**Inheritance chain**: The parser must track the full derivation chain so that:
1. Facet applicability is inherited from base types
2. Facets can only be made *more restrictive* (never relaxed)
3. ComplexType extension appends new particles to the base content model
4. ComplexType restriction narrows the base content model

### 0.6 Built-in Type Tests (`xsd/builtin_test.go`)

**Comprehensive test coverage for all 49 built-in types:**

```go
// Test categories:
// 1. Registry completeness — all 49 types present
// 2. Hierarchy — derivation chains are correct
// 3. Facet applicability — each type supports exactly the right facets
// 4. Go type mapping — each built-in maps to the correct Go type
// 5. Restriction validation — valid/invalid facet application
// 6. Property inheritance — ordered, bounded, numeric propagation

func TestAllBuiltinTypesPresent(t *testing.T)          // all 49 types registered
func TestBuiltinDerivationChain(t *testing.T)          // integer→decimal→anySimpleType→anyType
func TestStringFamilyFacets(t *testing.T)              // string, normalizedString, token, language, ...
func TestNumericFamilyFacets(t *testing.T)             // decimal, integer, long, int, short, byte, ...
func TestDateTimeFamilyFacets(t *testing.T)            // dateTime, date, time, gYear, ...
func TestBinaryFamilyFacets(t *testing.T)              // hexBinary, base64Binary
func TestBooleanFacets(t *testing.T)                   // only pattern, whiteSpace
func TestGoTypeMappings(t *testing.T)                  // xs:int→int32, xs:long→int64, ...
func TestValidRestriction(t *testing.T)                // pattern on string → OK
func TestInvalidRestriction(t *testing.T)              // totalDigits on string → error
func TestFacetInheritance(t *testing.T)                // token inherits string's facets
func TestFacetNarrowing(t *testing.T)                  // can tighten maxLength, can't loosen
func TestXSD11BuiltinTypes(t *testing.T)               // yearMonthDuration, dayTimeDuration, dateTimeStamp
func TestTypeProperties(t *testing.T)                  // ordered, bounded, numeric, cardinality
func TestUserDefinedSimpleTypeRestriction(t *testing.T) // ZipCode restricts string
func TestUserDefinedSimpleTypeList(t *testing.T)       // list of tokens
func TestUserDefinedSimpleTypeUnion(t *testing.T)      // union of integer|string
func TestComplexTypeExtension(t *testing.T)            // USAddress extends AddressType
func TestComplexTypeRestriction(t *testing.T)          // SmallOrder restricts OrderType
```

---

## Phase 1: Schema Model (the IR)

The foundation — all other components depend on this.

### 1.1 Core Model Types (`xsd/model.go`)

```go
package xsd

// Schema is the root of a parsed XSD.
// STABILITY: All slices preserve document order. Maps are for O(1) lookup only
// and are NEVER iterated for output. This ensures deterministic code generation.
type Schema struct {
    TargetNamespace string
    Namespaces      map[string]string // prefix → URI (lookup only)
    Elements        []*Element        // document order (stable iteration)
    Types           []Type            // document order (stable iteration)
    Groups          []*Group          // document order
    AttributeGroups []*AttributeGroup // document order
    Imports         []*Import         // document order
    Includes        []*Include        // document order
    Annotations     []*Annotation     // document order

    // Internal indexes — for O(1) lookup, NEVER iterated for output.
    elementIndex map[string]*Element  // name → element
    typeIndex    map[QName]Type       // qname → type
    groupIndex   map[string]*Group    // name → group
}

type Element struct {
    Name             string
    Type             TypeRef
    MinOccurs        int
    MaxOccurs        int  // -1 = unbounded
    Nillable         bool
    Abstract         bool
    Default          *string
    Fixed            *string
    SubstitutionGroup *QName
    InlineType       Type // anonymous type defined inline
    Annotations      []*Annotation
    Alternatives     []*Alternative // XSD 1.1
}

type Attribute struct {
    Name        string
    Type        TypeRef
    Use         AttributeUse // Optional, Required, Prohibited
    Default     *string
    Fixed       *string
    Inheritable bool // XSD 1.1
    Annotations []*Annotation
}
```

### 1.2 Type Hierarchy (`xsd/types.go`)

```go
// Type is the interface for all XSD types.
type Type interface {
    TypeName() QName
    isType()
}

type SimpleType struct {
    Name        QName
    Restriction *Restriction
    List        *List
    Union       *Union
    Annotations []*Annotation
}

type ComplexType struct {
    Name            QName
    Abstract        bool
    Mixed           bool
    Content         Content        // SimpleContent | ComplexContent | direct compositor
    Attributes      []*Attribute
    AttributeGroups []*AttributeGroupRef
    AnyAttribute    *AnyAttribute
    Assertions      []*Assertion   // XSD 1.1
    OpenContent     *OpenContent   // XSD 1.1
    Annotations     []*Annotation
}
```

### 1.3 Compositors & Nested Compositor Support (`xsd/compositor.go`)

Compositors can nest arbitrarily deep. This is a key area where existing tools fail.

```go
// Content is the interface for complex type content models.
type Content interface {
    isContent()
}

// Compositor is the shared interface for Sequence, Choice, and All.
// Compositors are themselves Particles, enabling arbitrary nesting.
type Compositor interface {
    Particle
    Content
    GetParticles() []Particle
    GetMinOccurs() int
    GetMaxOccurs() int
}

type Sequence struct {
    MinOccurs int
    MaxOccurs int
    Particles []Particle // Element | GroupRef | Choice | Sequence | All | Any
}
func (Sequence) isParticle() {}
func (Sequence) isContent()  {}

type Choice struct {
    MinOccurs int
    MaxOccurs int
    Particles []Particle
}
func (Choice) isParticle() {}
func (Choice) isContent()  {}

type All struct {
    MinOccurs int
    MaxOccurs int  // XSD 1.1 allows > 1
    Particles []Particle
}
func (All) isParticle() {}
func (All) isContent()  {}

// Particle is anything that can appear in a compositor.
type Particle interface {
    isParticle()
}

// Element, GroupRef, and Any are also Particles (implement isParticle).
```

**Nesting examples that MUST work:**
```xml
<!-- Choice inside Sequence -->
<xs:sequence>
  <xs:element name="header" type="xs:string"/>
  <xs:choice>
    <xs:element name="optionA" type="xs:string"/>
    <xs:element name="optionB" type="xs:int"/>
  </xs:choice>
  <xs:element name="footer" type="xs:string"/>
</xs:sequence>

<!-- Sequence inside Choice -->
<xs:choice>
  <xs:sequence>
    <xs:element name="first" type="xs:string"/>
    <xs:element name="last" type="xs:string"/>
  </xs:sequence>
  <xs:element name="fullName" type="xs:string"/>
</xs:choice>

<!-- Choice inside Choice -->
<xs:choice>
  <xs:choice>
    <xs:element name="a" type="xs:string"/>
    <xs:element name="b" type="xs:string"/>
  </xs:choice>
  <xs:element name="c" type="xs:string"/>
</xs:choice>

<!-- Deep nesting: sequence > choice > sequence > choice -->
<xs:sequence>
  <xs:choice>
    <xs:sequence>
      <xs:element name="x" type="xs:string"/>
      <xs:choice>
        <xs:element name="y1" type="xs:string"/>
        <xs:element name="y2" type="xs:int"/>
      </xs:choice>
    </xs:sequence>
    <xs:element name="z" type="xs:string"/>
  </xs:choice>
</xs:sequence>
```

**Codegen strategy for nested compositors:**
- Nested choices generate nested interfaces (e.g., `OuterChoice` with one variant being a struct containing an `InnerChoice` interface field)
- Sequence inside choice: the sequence becomes a struct variant of the choice
- Choice inside sequence: the choice becomes an interface field of the struct
- Recursive flattening is applied where possible (choice-in-choice can flatten to single interface)

### 1.4 Constraints & Facets (`xsd/constraint.go`)

```go
type Restriction struct {
    Base   TypeRef
    Facets []Facet
    // For complex type restriction
    Content Content
}

type Facet struct {
    Kind  FacetKind // Pattern, Enumeration, MinLength, MaxLength, etc.
    Value string
}

type Assertion struct { // XSD 1.1
    Test        string // XPath expression
    Annotations []*Annotation
}
```

### Tests for Phase 1
- Unit tests that construct model types programmatically
- Verify QName resolution helpers
- Verify model traversal/visitor works

---

## Phase 2: Parser (XSD XML → Model)

**Streaming-first architecture**: The streaming parser (`parser/streaming.go`) is the
foundation. The DOM parser (`parser/parser.go`) is a thin wrapper that feeds the stream
into a `CollectingHandler` which builds the full model.

### 2.0 Logging with `slog`

All parser components use `log/slog` (Go 1.21+) for structured logging:

```go
package parser

type Options struct {
    Logger           *slog.Logger       // If nil, slog.Default() is used
    Resolver         SchemaResolver     // How to fetch imported/included schemas
    SchemaStrictness ValidationConfig   // How strictly to validate the XSD itself
}

// Functional options:
func WithLogger(l *slog.Logger) Option
func WithResolver(r SchemaResolver) Option
func WithSchemaStrictness(c ValidationConfig) Option

// Usage throughout:
// p.opts.Logger.Debug("resolving type reference", "ref", ref, "namespace", ns)
// p.opts.Logger.Info("parsed schema", "namespace", schema.TargetNamespace, "types", len(schema.Types))
// p.opts.Logger.Warn("duplicate type definition", "name", name, "location", loc)
// p.opts.Logger.Error("circular import detected", "chain", chain)
```

Log groups for structured context:
- `parser.stream` — streaming parser events
- `parser.resolve` — type/ref resolution
- `parser.import` — import/include processing
- `parser.validate` — facet/default validation

### 2.1 Streaming Parser — THE FOUNDATION (`parser/streaming.go`)

The streaming parser reads XSD using `encoding/xml` token-by-token and emits
typed events. It is the single XML reading path — all other parsing builds on it.

```go
package parser

// Event types emitted by the streaming parser.
type EventKind int

const (
    EventSchemaStart EventKind = iota
    EventSchemaEnd
    EventElementStart
    EventElementEnd
    EventComplexTypeStart
    EventComplexTypeEnd
    EventSimpleTypeStart
    EventSimpleTypeEnd
    EventSequenceStart
    EventSequenceEnd
    EventChoiceStart
    EventChoiceEnd
    EventAllStart
    EventAllEnd
    EventAttribute
    EventImport
    EventInclude
    EventRedefine        // xs:redefine
    EventOverride        // xs:override (XSD 1.1)
    EventAnnotation
    EventFacet
    EventGroup
    EventGroupEnd
    EventAttributeGroup
    EventAttributeGroupEnd
    EventRestriction
    EventExtension
    EventList
    EventUnion
)

// Event is emitted during streaming parse.
type Event struct {
    Kind       EventKind
    Name       string
    Namespace  string
    Attributes map[string]string // raw XML attributes
    Depth      int               // nesting depth in XSD structure
    Location   Location          // file + line/col
}

type Location struct {
    SystemID string // file path or URI
    Line     int
    Col      int
}

// StreamHandler receives events during streaming parse.
type StreamHandler interface {
    OnEvent(event Event) error
    OnError(err error) error
}

// StreamParser parses XSD token-by-token and emits events.
type StreamParser struct {
    opts    Options
    handler StreamHandler
    logger  *slog.Logger
}

func NewStreamParser(handler StreamHandler, opts ...Option) *StreamParser

// Stream parses and emits events from an io.Reader.
func (sp *StreamParser) Stream(r io.Reader, systemID string) error

// StreamFile is a convenience for file-based streaming.
func (sp *StreamParser) StreamFile(path string) error
```

### 2.1.1 CollectingHandler — Stream → DOM (`parser/handler.go`)

The `CollectingHandler` builds the full schema model from stream events. It maintains
an incremental symbol table so that type references can be connected as definitions
are encountered during the stream.

```go
// CollectingHandler builds a full xsd.Schema from stream events.
// This is the bridge between streaming and DOM parsing.
type CollectingHandler struct {
    schema   *xsd.Schema
    registry *xsd.BuiltinRegistry
    symbols  *SymbolTable
    stack    []buildContext      // stack of in-progress constructs
    logger   *slog.Logger

    // Incremental reference resolution:
    // As type definitions are seen, they're added to the symbol table.
    // Forward references are collected and resolved in a final pass.
    pendingRefs []pendingRef
}

type pendingRef struct {
    ref      xsd.TypeRef
    location Location
    setter   func(xsd.Type) // callback to wire up the reference
}

// Schema returns the built model. Call after streaming completes.
// Performs final reference resolution and validation.
func (c *CollectingHandler) Schema() (*xsd.Schema, error)

// OnEvent processes each stream event, building the model incrementally.
func (c *CollectingHandler) OnEvent(event Event) error

// OnError logs and optionally collects non-fatal errors.
func (c *CollectingHandler) OnError(err error) error
```

**Incremental resolution strategy:**
1. As `EventComplexTypeStart`/`EventSimpleTypeStart` events arrive, definitions
   are added to the symbol table immediately
2. When a `type="..."` or `base="..."` reference is encountered, check the symbol
   table — if found, wire it up immediately; if not, add to `pendingRefs`
3. After streaming completes, resolve all `pendingRefs` (forward references)
4. Any remaining unresolved refs are errors (unless from imported namespaces
   that haven't been loaded yet)

### 2.1.2 Other Built-in Handlers

```go
// FilteringHandler wraps another handler and only forwards matching events.
type FilteringHandler struct {
    Inner      StreamHandler
    KindFilter map[EventKind]bool
}

// TypeListHandler collects just type names (fast schema introspection).
type TypeListHandler struct {
    Types []xsd.QName
}

// ElementListHandler collects just global element names.
type ElementListHandler struct {
    Elements []xsd.QName
}

// MultiHandler fans out events to multiple handlers.
type MultiHandler struct {
    Handlers []StreamHandler
}

// FollowImportsHandler automatically streams imported/included schemas.
// When it receives EventImport/EventInclude, it calls the SchemaResolver to
// get the bytes, creates a new StreamParser, and feeds the result to Inner.
// Everything is synchronous — no goroutines.
type FollowImportsHandler struct {
    Inner    StreamHandler
    Resolver SchemaResolver
    visited  map[string]bool // prevent circular follows
    logger   *slog.Logger
}
```

### 2.2 DOM Parser — Convenience Wrapper (`parser/parser.go`)

The DOM parser is a thin wrapper: StreamParser + CollectingHandler + resolution pass.

```go
package parser

type Parser struct {
    opts       Options
    hooks      []Hook
    schemas    map[string]*xsd.Schema // namespace → schema
    symbols    SymbolTable
    logger     *slog.Logger
}

func New(opts ...Option) *Parser

// Parse parses one or more XSD files and returns the resolved schema set.
// Internally uses StreamParser + CollectingHandler.
func (p *Parser) Parse(files ...string) (*xsd.SchemaSet, error)

// ParseReader parses from an io.Reader.
func (p *Parser) ParseReader(r io.Reader, systemID string) (*xsd.SchemaSet, error)

// Parse implementation (pseudocode):
// func (p *Parser) Parse(files ...string) (*xsd.SchemaSet, error) {
//     for _, file := range files {
//         handler := NewCollectingHandler(p.registry, p.symbols, p.logger)
//         followHandler := &FollowImportsHandler{
//             Inner: handler, Resolver: p.resolver, // SchemaResolver
//         }
//         sp := NewStreamParser(followHandler, p.opts)
//         // Everything is synchronous — no goroutines anywhere.
//         if err := sp.StreamFile(file); err != nil { return nil, err }
//         schema, err := handler.Schema()
//         // ... add to schema set
//     }
//     // Final cross-schema resolution pass (also synchronous)
//     return p.resolveAll()
// }
```

### 2.2 Import/Include Resolution (Deep Dive)

Schema composition is critical for real-world XSD usage. Three mechanisms exist:

#### `xs:import` — Cross-Namespace References (`parser/import.go`)

```xml
<!-- main.xsd -->
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:addr="http://example.com/address"
           targetNamespace="http://example.com/order">
  <xs:import namespace="http://example.com/address"
             schemaLocation="address.xsd"/>
  <xs:element name="shipTo" type="addr:AddressType"/>
</xs:schema>
```

**Requirements:**
- Import loads a schema from a different namespace
- `schemaLocation` is a *hint*, not a mandate — support catalog override
- Multiple imports of the same namespace are allowed (merged)
- If `schemaLocation` is absent, type must be resolved via catalog or pre-loaded schemas
- Circular imports across namespaces must be detected and handled

```go
// SchemaResolver resolves a schema location to its raw bytes.
// The parser calls this when it encounters xs:import, xs:include, xs:redefine,
// or xs:override. The user controls how schemas are fetched.
//
// Parameters:
//   - location: the schemaLocation attribute value (relative or absolute path/URI)
//   - baseURI:  the URI of the schema that contains the import/include directive
//   - namespace: the target namespace (for xs:import; empty for xs:include)
//
// Returns the raw schema bytes. The parser handles all XML parsing internally.
type SchemaResolver interface {
    Resolve(location, baseURI, namespace string) ([]byte, error)
}

// Pre-defined resolvers:

// FileResolver resolves schema locations relative to the importing schema's
// directory on the local filesystem.
type FileResolver struct{}

// HTTPResolver fetches schemas from HTTP/HTTPS URLs.
// Includes an in-memory cache keyed by resolved URL.
type HTTPResolver struct {
    cache map[string][]byte
}

// CatalogResolver looks up schemas via an OASIS XML Catalog file.
// Maps namespace URIs and system IDs to local file paths.
type CatalogResolver struct {
    catalogPath string
    entries     map[string]string // namespace/systemID → local path
}

// MultiResolver tries multiple resolvers in order, returning the first success.
type MultiResolver struct {
    Resolvers []SchemaResolver
}

func (m *MultiResolver) Resolve(location, baseURI, namespace string) ([]byte, error) {
    for _, r := range m.Resolvers {
        data, err := r.Resolve(location, baseURI, namespace)
        if err == nil {
            return data, nil
        }
    }
    return nil, fmt.Errorf("no resolver could resolve %q (base: %q, ns: %q)", location, baseURI, namespace)
}

// Example usage:
// resolver := &MultiResolver{
//     Resolvers: []SchemaResolver{
//         &CatalogResolver{catalogPath: "catalog.xml"},
//         &FileResolver{},
//         &HTTPResolver{},
//     },
// }
// p := parser.New(parser.WithResolver(resolver))
// schemas, err := p.Parse("main.xsd")
```

#### `xs:include` — Same-Namespace Composition (`parser/include.go`)

```xml
<!-- types.xsd has same targetNamespace or no targetNamespace (chameleon) -->
<xs:include schemaLocation="types.xsd"/>
```

**Requirements:**
- Include merges declarations into the including schema's namespace
- **Chameleon include**: if the included schema has no `targetNamespace`, it adopts the includer's namespace
- Duplicate definitions after include must be detected (same name = error unless identical)
- Circular includes detected via visited set (keyed by resolved URI)

#### `xs:redefine` — Include with Modifications

```xml
<xs:redefine schemaLocation="base-types.xsd">
  <xs:complexType name="AddressType">
    <xs:complexContent>
      <xs:extension base="AddressType">  <!-- self-reference = original -->
        <xs:sequence>
          <xs:element name="country" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:redefine>
```

**Requirements:**
- Like include, but allows redefining types/groups in the included schema
- Self-referencing base type in redefine refers to the *original* definition
- Deprecated in XSD 1.1 in favor of `xs:override`, but still must be supported

#### `xs:override` (XSD 1.1) — Replacement for Redefine

```xml
<xs:override schemaLocation="base-types.xsd">
  <xs:complexType name="AddressType">
    <!-- Completely replaces the original definition -->
  </xs:complexType>
</xs:override>
```

#### Schema Resolution Order

Resolution is handled entirely by the user-provided `SchemaResolver`. The
`MultiResolver` chains resolvers in user-specified order:

```
Default MultiResolver order:
1. CatalogResolver (if configured) — namespace/systemID → local file
2. FileResolver — schemaLocation relative to importing schema's directory
3. HTTPResolver — fetch from URL (if location is http/https)

Special cases handled by the parser (before calling resolver):
- xs namespace (http://www.w3.org/2001/XMLSchema) → built-in types (no I/O)
- Already-loaded schemas (visited set) → skip (no I/O)
```

#### Import/Include Test Fixtures

```
testdata/imports/
├── simple_import/
│   ├── main.xsd              # imports types.xsd
│   └── types.xsd             # separate namespace
├── chameleon_include/
│   ├── main.xsd              # includes no-ns.xsd
│   └── no_ns.xsd             # no targetNamespace
├── circular_import/
│   ├── a.xsd                 # imports b.xsd
│   └── b.xsd                 # imports a.xsd
├── diamond_import/
│   ├── main.xsd              # imports left.xsd, right.xsd
│   ├── left.xsd              # imports common.xsd
│   ├── right.xsd             # imports common.xsd
│   └── common.xsd            # shared types
├── redefine/
│   ├── main.xsd              # redefines base
│   └── base.xsd              # original types
├── override/                  # XSD 1.1
│   ├── main.xsd
│   └── base.xsd
├── multi_ns/
│   ├── orders.xsd            # ns: orders
│   ├── products.xsd          # ns: products
│   ├── customers.xsd         # ns: customers
│   └── main.xsd              # imports all three
└── catalog/
    ├── catalog.xml            # OASIS XML Catalog
    └── main.xsd              # uses catalog for resolution
```

### 2.3 Parser Hooks (`parser/hooks.go`)

```go
// Hook allows plugins to intercept and modify parsing.
type Hook interface {
    // Called after an element is parsed but before it's added to the schema.
    OnElement(ctx *HookContext, elem *xsd.Element) (*xsd.Element, error)
    // Called after a type is parsed.
    OnType(ctx *HookContext, typ xsd.Type) (xsd.Type, error)
    // Called for custom/unknown elements (extension points).
    OnCustomElement(ctx *HookContext, el xml.StartElement) error
    // Called for custom/unknown attributes.
    OnCustomAttribute(ctx *HookContext, attr xml.Attr) error
}
```

### Test Plan for Phase 2

**Incremental test fixtures** (each builds on the previous):

#### Basic Parsing
1. `testdata/basic/simple_element.xsd` — single element with built-in type
2. `testdata/basic/simple_type.xsd` — simpleType with restriction (enum, pattern)
3. `testdata/basic/complex_type.xsd` — complexType with sequence of elements
4. `testdata/basic/attributes.xsd` — elements with attributes

#### Built-in Types (Strong Coverage)
5. `testdata/builtin/all_primitives.xsd` — elements using every primitive type
6. `testdata/builtin/all_derived.xsd` — elements using every derived type
7. `testdata/builtin/restrict_string.xsd` — custom type restricting string with pattern + maxLength
8. `testdata/builtin/restrict_integer.xsd` — custom type restricting integer with min/max
9. `testdata/builtin/restrict_decimal.xsd` — custom type with totalDigits + fractionDigits
10. `testdata/builtin/invalid_facet.xsd` — applying totalDigits to string (must error)
11. `testdata/builtin/chained_restriction.xsd` — type A restricts string, type B restricts A
12. `testdata/builtin/list_type.xsd` — simpleType list
13. `testdata/builtin/union_type.xsd` — simpleType union
14. `testdata/builtin/xsd11_types.xsd` — yearMonthDuration, dayTimeDuration, dateTimeStamp

#### Choice & Nested Compositors
15. `testdata/choice/basic_choice.xsd` — complexType with choice
16. `testdata/choice/nested_choice.xsd` — choice inside sequence, sequence inside choice
17. `testdata/nested/choice_in_choice.xsd` — choice containing another choice
18. `testdata/nested/sequence_in_all.xsd` — sequence inside all (XSD 1.1)
19. `testdata/nested/deep_nesting.xsd` — 4+ levels: sequence > choice > sequence > choice
20. `testdata/nested/mixed_compositors.xsd` — all three compositors in one type

#### Type Derivation
21. `testdata/derivation/simple_restriction.xsd` — simpleType restricts built-in
22. `testdata/derivation/complex_extension.xsd` — complexType extends base type
23. `testdata/derivation/complex_restriction.xsd` — complexType restricts base type
24. `testdata/derivation/multi_level.xsd` — A extends B extends C
25. `testdata/derivation/abstract_base.xsd` — abstract type with concrete subtypes
26. `testdata/derivation/mixed_content_extension.xsd` — mixed content + extension

#### Advanced Features
27. `testdata/complex/group.xsd` — model groups and attribute groups
28. `testdata/complex/any.xsd` — xs:any and xs:anyAttribute
29. `testdata/complex/substitution.xsd` — substitution groups

#### Import/Include (see section 2.2 for fixture details)
30-38. All fixtures in `testdata/imports/` (simple, chameleon, circular, diamond, redefine, override, multi-ns, catalog)

#### XSD 1.1
39. `testdata/xsd11/assert.xsd` — assertions
40. `testdata/xsd11/alternative.xsd` — conditional type assignment
41. `testdata/xsd11/open_content.xsd` — open content model

#### Streaming Parser
42. `testdata/streaming/large_schema.xsd` — schema with 100+ types (event count test)
43. `testdata/streaming/filter_test.xsd` — verify filtering handler
44. `testdata/streaming/stream_vs_dom.xsd` — parse both ways, verify same types/elements found

Each test: parse XSD → assert model structure matches expectations.

---

## Phase 3: Code Generator (Model → Go Source)

### 3.0 Contextual Naming System (`codegen/naming.go`)

Anonymous (inline) types in XSD have no `name` attribute — they must be assigned
stable, human-readable Go names during code generation. This is a critical system
that must be **deterministic** (same input always produces same output) and
**conflict-free** (no two types get the same Go name).

#### 3.0.1 The Problem

```xml
<!-- Named type: easy, just use "AddressType" -->
<xs:complexType name="AddressType">...</xs:complexType>

<!-- Anonymous type: needs a generated name -->
<xs:element name="order">
  <xs:complexType>                    <!-- What Go name? -->
    <xs:sequence>
      <xs:element name="item">
        <xs:complexType>              <!-- What Go name? Nested anonymous! -->
          <xs:sequence>
            <xs:element name="details">
              <xs:simpleType>         <!-- What Go name? 3 levels deep! -->
                <xs:restriction base="xs:string">
                  <xs:maxLength value="100"/>
                </xs:restriction>
              </xs:simpleType>
            </xs:element>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:element>
```

#### 3.0.2 Naming Strategy

Names are derived from the **definition context** — the path of containing
elements/types from the schema root to the anonymous definition.

**Priority order for name derivation:**
1. **Named type**: use the `name` attribute directly → `AddressType`
2. **Anonymous type in element**: `{ElementName}Type` → `OrderType`
3. **Anonymous type in attribute**: `{AttributeName}Type` → `CurrencyType`
4. **Nested anonymous in element chain**: join parent names → `OrderItemType`, `OrderItemDetailsType`
5. **Anonymous in choice variant**: `{ParentType}{ElementName}Type` → `ShapeCircleType`
6. **Anonymous in group**: `{GroupName}{ElementName}Type` → `AddressFieldsStreetType`
7. **Anonymous in restriction/extension**: `{ParentType}BaseType` or use parent name

```go
package codegen

// Namer assigns stable Go names to all types in a schema.
type Namer struct {
    registry  map[string]namedEntry  // Go name → source (for conflict detection)
    usedNames map[string]bool        // all assigned names
    logger    *slog.Logger
}

type namedEntry struct {
    GoName   string
    Source   namingSource  // what produced this name
    XSDPath  []string      // element/type path from schema root
}

type namingSource int
const (
    sourceNamed      namingSource = iota // explicit XSD name attribute
    sourceElement                         // derived from parent element name
    sourceAttribute                       // derived from parent attribute name
    sourceCompositor                      // derived from compositor context
    sourceConflict                        // renamed due to conflict
)

// AssignNames walks the schema model and assigns Go names to all types.
// This is called once before code generation begins.
// Returns a map from model pointer → Go name.
func (n *Namer) AssignNames(schema *xsd.SchemaSet) (*NameMap, error)

// NameMap provides O(1) lookup from any Type/Element to its assigned Go name.
type NameMap struct {
    types    map[xsd.Type]string
    elements map[*xsd.Element]string
}

func (m *NameMap) TypeName(t xsd.Type) string
func (m *NameMap) ElementName(e *xsd.Element) string
```

#### 3.0.3 Conflict Resolution

When two anonymous types would produce the same Go name, conflicts are resolved
deterministically:

```go
// Conflict resolution strategy (in order):
// 1. Try the base name: "OrderType"
// 2. If conflict, qualify with parent: "MainOrderType" vs "LegacyOrderType"
// 3. If still conflict, qualify with grandparent: "SchemaMainOrderType"
// 4. If still conflict (extremely rare), append numeric suffix: "OrderType2"
//    Suffix is assigned by document order (first occurrence gets no suffix).

// Example conflicts:
// <xs:element name="order">           → OrderType
//   <xs:complexType>...</xs:complexType>
// </xs:element>
// <xs:element name="legacyOrder">
//   <xs:complexType>
//     <xs:sequence>
//       <xs:element name="order">     → LegacyOrderOrderType (qualified with parent)
//         <xs:complexType>...</xs:complexType>
//       </xs:element>
//     </xs:sequence>
//   </xs:complexType>
// </xs:element>
```

#### 3.0.4 Determinism & Stability Requirements

**The naming system MUST produce identical output for identical input, every time.**
This is critical for:
- Generated code that's checked into version control (no spurious diffs)
- Reproducible builds
- Stable API surfaces (renamed types break downstream consumers)

**Rules for deterministic processing:**

```go
// STABILITY RULES — enforced throughout parser, namer, and codegen:
//
// 1. NO CONCURRENCY. The entire library is single-threaded.
//    No goroutines are used anywhere in parsing, naming, or codegen.
//    This eliminates an entire class of non-determinism and makes the
//    code simpler to reason about, debug, and test.
//
// 2. NO map iteration for ordered output.
//    Maps are used for O(1) lookup only. When order matters, iterate
//    over the source slice (which preserves document order) and look up
//    in the map.
//
// 3. Document order is the canonical ordering.
//    Elements, types, attributes, and compositors are stored in slices
//    that preserve their order from the XSD source document. The streaming
//    parser naturally produces events in document order.
//
// 4. Name assignment happens in a single deterministic pass.
//    Walk the schema in document order (depth-first). Assign names as
//    encountered. First occurrence wins (no suffix). Later conflicts get
//    qualified names or suffixes.
//
// 5. Import/include order is deterministic.
//    Imported schemas are processed in the order their <xs:import> elements
//    appear in the importing schema. The FollowImportsHandler processes
//    them synchronously in this order.
//
// 6. Cross-namespace naming uses namespace-prefixed disambiguation.
//    If two namespaces both define "AddressType", they become
//    "AddressType" and "Ns2AddressType" (or user-configured prefix).
```

**Data structures for stable iteration:**

```go
// OrderedMap preserves insertion order while providing O(1) lookup.
// Used throughout the codebase where both ordering and lookup are needed.
type OrderedMap[K comparable, V any] struct {
    keys   []K          // insertion order
    values map[K]V      // O(1) lookup
}

func (m *OrderedMap[K, V]) Set(key K, value V)
func (m *OrderedMap[K, V]) Get(key K) (V, bool)
func (m *OrderedMap[K, V]) Keys() []K           // returns keys in insertion order
func (m *OrderedMap[K, V]) Range(fn func(K, V)) // iterates in insertion order

// Usage in Schema model:
type Schema struct {
    // ...
    // Types are stored as a slice (document order) + map (O(1) lookup by QName).
    Types     []Type                    // document order
    typeIndex map[QName]Type            // O(1) lookup (NOT iterated)
}
```

#### 3.0.5 Naming Edge Cases

| XSD Pattern | Generated Name | Notes |
|---|---|---|
| `<xs:element name="person"><xs:complexType>...` | `PersonType` | Element name + "Type" |
| `<xs:element name="address"><xs:complexType>` inside `PersonType` | `PersonAddressType` | Parent + element |
| `<xs:element name="line"><xs:simpleType>` inside `PersonAddressType` | `PersonAddressLineType` | Full path |
| Anonymous type in `<xs:choice>` variant | `{ChoiceParent}{VariantElement}Type` | Choice context |
| Anonymous type in `<xs:group name="Foo">` | `Foo{Element}Type` | Group context |
| Anonymous type in `<xs:restriction>` | Inherits parent's name context | No extra nesting |
| Anonymous type in `<xs:list itemType>` | `{ParentType}ItemType` | List item |
| Anonymous type in `<xs:union>` | `{ParentType}Member{N}Type` | Union member (N=1,2,...) |
| Two elements named "item" at different nesting | `ItemType`, `OrderItemType` | Conflict resolution |
| Cross-namespace same name | `AddressType`, `Ns2AddressType` | Namespace prefix |

#### 3.0.6 Naming Tests

```go
func TestSimpleAnonymousTypeName(t *testing.T)        // element → ElementType
func TestNestedAnonymousTypeName(t *testing.T)         // parent.child → ParentChildType
func TestDeeplyNestedAnonymousTypeName(t *testing.T)   // 4+ levels deep
func TestConflictResolutionQualify(t *testing.T)       // same base name, different parents
func TestConflictResolutionSuffix(t *testing.T)        // truly identical paths (rare)
func TestNamingDeterminism(t *testing.T)               // parse same schema 100x, verify same names
func TestNamingStabilityAcrossRuns(t *testing.T)       // serialize names, compare across runs
func TestAnonymousTypeInChoice(t *testing.T)           // choice variant naming
func TestAnonymousTypeInGroup(t *testing.T)            // group element naming
func TestCrossNamespaceNaming(t *testing.T)            // namespace-prefixed disambiguation
func TestNamedTypeUnchanged(t *testing.T)              // named types keep their XSD name
func TestAnonymousTypeInListUnion(t *testing.T)        // list itemType, union memberTypes
```

#### 3.0.7 Integration with Streaming Parser

The naming system works on the completed model (after `CollectingHandler.Schema()`
returns), not during streaming. However, the streaming parser's document-order
preservation is what makes naming deterministic:

```
Stream events (document order) → CollectingHandler (preserves order in slices)
    → Schema model (slices = document order) → Namer (walks in document order)
    → NameMap (stable assignments) → CodeGen (uses NameMap for all type names)
```

### 3.1 Type Mapping Strategy — Strict vs Freeform Types

Users can choose between two type modes (or mix them per-type):

**Freeform mode** (default): Uses plain Go types. Simple, easy to work with,
no validation overhead. Best for trusted data or when validation happens elsewhere.

**Strict mode**: Uses wrapper types with built-in validation. Each type enforces
its XSD facets at the Go level. Best for ensuring data conforms to the schema.

```go
// codegen Options
type Options struct {
    // ...
    TypeMode     TypeMode     // Freeform (default) | Strict | Mixed
    // Per-type overrides (when TypeMode=Mixed)
    StrictTypes  map[string]bool // type name → strict?
    // Per-rule validation strictness
    Validation   ValidationConfig
}

type TypeMode int
const (
    TypeModeFreeform TypeMode = iota // plain Go types (string, int64, etc.)
    TypeModeStrict                    // wrapper types with validation
    TypeModeMixed                     // per-type choice via StrictTypes map
)
```

#### Freeform Type Mapping

| XSD Construct | Go Output |
|---|---|
| `xs:string` | `string` |
| `xs:int` | `int32` |
| `xs:integer` | `int64` (or `*big.Int` if unbounded) |
| `xs:boolean` | `bool` |
| `xs:float`/`xs:double` | `float32`/`float64` |
| `xs:dateTime` | `time.Time` |
| `xs:date` | `time.Time` |
| `xs:duration` | `string` (no stdlib equivalent) |
| `xs:base64Binary` | `[]byte` |
| `xs:hexBinary` | `[]byte` |
| `xs:decimal` | `string` (or `*big.Rat` if configured) |
| `xs:anyURI` | `string` |
| `xs:complexType` (sequence) | Go struct |
| `xs:complexType` (choice) | **Interface + concrete types** |
| `xs:simpleType` (enum) | `type X string` + constants |
| `xs:simpleType` (restriction) | Named type alias (no validation) |
| `xs:element` maxOccurs > 1 | `[]T` |
| `xs:element` nillable | `*T` |
| `xs:any` | `[]xml.Token` or `any` |
| `xs:group` | Embedded struct (inlined) |
| `xs:extension` | Embedded base struct |
| `xs:substitutionGroup` | Interface (like choice) |

#### Strict Type Mapping — Validated Wrapper Types

Strict types wrap the underlying Go type and enforce XSD facets:

```go
// Generated strict types live in a "xsdtypes" sub-package.
// Example for xs:string restricted with maxLength=10, pattern=[A-Z]+:

// PartCode is a strict wrapper for the PartCode XSD type.
type PartCode struct {
    value string
}

// NewPartCode creates a PartCode, returning an error if validation fails.
func NewPartCode(v string) (PartCode, error) {
    if err := validatePartCode(v); err != nil {
        return PartCode{}, err
    }
    return PartCode{value: v}, nil
}

// MustPartCode panics if validation fails (for literals/tests).
func MustPartCode(v string) PartCode {
    pc, err := NewPartCode(v)
    if err != nil { panic(err) }
    return pc
}

func (p PartCode) String() string { return p.value }

func validatePartCode(v string) error {
    if len(v) > 10 {
        return &xsdtypes.FacetError{Type: "PartCode", Facet: "maxLength", Limit: "10", Got: v}
    }
    if !partCodePattern.MatchString(v) {
        return &xsdtypes.FacetError{Type: "PartCode", Facet: "pattern", Limit: "[A-Z]+", Got: v}
    }
    return nil
}

var partCodePattern = regexp.MustCompile(`^[A-Z]+$`)
```

**Strict types for numeric ranges:**
```go
// Percentage wraps int64, enforces minInclusive=0, maxInclusive=100.
type Percentage struct { value int64 }

func NewPercentage(v int64) (Percentage, error) {
    if v < 0 { return Percentage{}, &xsdtypes.FacetError{...} }
    if v > 100 { return Percentage{}, &xsdtypes.FacetError{...} }
    return Percentage{value: v}, nil
}
```

**Built-in strict types** (provided by `xsdtypes` package):
```go
package xsdtypes

// Strict wrappers for XSD built-in types that have no exact Go equivalent.
type Duration struct { ... }        // ISO 8601 duration with Parse/Format
type Date struct { ... }            // date without time (year, month, day + optional TZ)
type Time struct { ... }            // time without date
type GYear struct { ... }           // just a year
type GYearMonth struct { ... }      // year + month
type GMonthDay struct { ... }       // month + day
type GDay struct { ... }            // just a day
type GMonth struct { ... }          // just a month
type HexBinary struct { ... }       // []byte with hex encoding in XML/JSON
type AnyURI struct { ... }          // validated IRI
type Decimal struct { ... }         // arbitrary-precision decimal
type Integer struct { ... }         // arbitrary-precision integer
type QName struct { ... }           // namespace-qualified name

// FacetError reports a validation failure.
type FacetError struct {
    Type   string // XSD type name
    Facet  string // facet name (maxLength, pattern, etc.)
    Limit  string // facet value from the schema
    Got    string // the invalid value
}
func (e *FacetError) Error() string
```

### 3.1.1 Two-Level Validation: Schema-Parsing vs Data-Parsing

Validation strictness is configured at **two separate levels**, because the
concerns are different:

1. **Schema-parsing strictness** — how strictly to validate the XSD schema itself
   during parsing (e.g., reject contradictory facets? reject invalid defaults?)
2. **Data-parsing strictness** — how strictly the *generated code* validates
   runtime data during marshal/unmarshal (e.g., enforce patterns? check ranges?)

Both use the same `ValidationConfig` type, but are configured independently.

```go
// ValidationConfig controls which validation rules are enforced.
// Used for both schema-parsing and data-parsing strictness.
type ValidationConfig struct {
    // Default behavior for all rules not explicitly configured.
    Default ValidationLevel

    // Per-rule overrides.
    Rules map[ValidationRule]ValidationLevel
}

type ValidationLevel int
const (
    ValidationError ValidationLevel = iota // reject invalid values (default)
    ValidationWarn                          // log warning via slog, accept value
    ValidationOff                           // skip validation entirely
)

type ValidationRule int
const (
    // --- Facet rules (apply to both schema-parsing and data-parsing) ---
    RulePattern        ValidationRule = iota // regex pattern matching
    RuleEnumeration                          // value in enum set
    RuleMinLength                            // string/list minimum length
    RuleMaxLength                            // string/list maximum length
    RuleMinInclusive                         // numeric/date minimum (inclusive)
    RuleMaxInclusive                         // numeric/date maximum (inclusive)
    RuleMinExclusive                         // numeric/date minimum (exclusive)
    RuleMaxExclusive                         // numeric/date maximum (exclusive)
    RuleTotalDigits                          // decimal total digits
    RuleFractionDigits                       // decimal fraction digits
    RuleWhiteSpace                           // whitespace normalization
    RuleLength                               // exact length

    // --- Schema-parsing only rules ---
    RuleDefaultValue        // default value type-validity
    RuleFixedValue          // fixed value type-validity
    RuleFacetCrossValidation // contradictory facets (minLength > maxLength)
    RuleFacetNarrowing      // facet can only narrow, not widen
    RuleFacetApplicability  // facet not applicable to base type
    RuleDuplicateDefinition // duplicate type/element names after include
    RuleCircularDerivation  // circular type derivation chains

    // --- Data-parsing only rules ---
    RuleRequiredElement     // required element missing during unmarshal
    RuleRequiredAttribute   // required attribute missing during unmarshal
    RuleUnknownElement      // unexpected element during unmarshal
    RuleUnknownAttribute    // unexpected attribute during unmarshal
)
```

#### Schema-Parsing Strictness

Controls how the parser validates the XSD schema files themselves. Configured
on `parser.Options`:

```go
package parser

type Options struct {
    Logger          *slog.Logger
    Resolver        SchemaResolver
    SchemaStrictness ValidationConfig  // how strictly to validate the schema
}

// Example: lenient parsing — accept schemas with contradictory facets
p := parser.New(
    parser.WithSchemaStrictness(ValidationConfig{
        Default: ValidationError,
        Rules: map[ValidationRule]ValidationLevel{
            RuleFacetCrossValidation: ValidationWarn,  // warn but don't reject
            RuleDefaultValue:         ValidationOff,   // don't validate defaults
        },
    }),
)
```

**What schema-parsing strictness controls:**
- `RuleFacetCrossValidation`: reject `minLength="10" maxLength="5"` or just warn?
- `RuleFacetNarrowing`: reject derived type that widens a facet, or accept?
- `RuleFacetApplicability`: reject `totalDigits` on `xs:string`, or ignore?
- `RuleDefaultValue`: reject `default="abc"` on `xs:integer`, or accept?
- `RuleFixedValue`: reject `fixed="green"` when enum is `{red, blue}`, or accept?
- `RuleDuplicateDefinition`: reject duplicate names after include, or last-wins?
- `RuleCircularDerivation`: reject circular type chains, or break the cycle?

#### Data-Parsing Strictness

Controls how the *generated code* validates runtime data during marshal/unmarshal.
Configured on `codegen.Options`:

```go
package codegen

type Options struct {
    // ...
    TypeMode        TypeMode
    StrictTypes     map[string]bool
    DataStrictness  ValidationConfig  // how strictly generated code validates data
}

// Example: strict data validation except patterns are just warnings
g := codegen.New(codegen.Options{
    TypeMode: codegen.TypeModeStrict,
    DataStrictness: ValidationConfig{
        Default: ValidationError,
        Rules: map[ValidationRule]ValidationLevel{
            RulePattern:          ValidationWarn,  // warn on pattern mismatch
            RuleUnknownElement:   ValidationOff,   // ignore unknown elements
        },
    },
})
```

**What data-parsing strictness controls:**
- `RulePattern`: generated `New*()` checks regex, returns error or warns?
- `RuleMaxLength`: generated code enforces maxLength, or skips?
- `RuleEnumeration`: generated code checks enum membership, or accepts any value?
- `RuleRequiredElement`: generated unmarshal errors on missing required element?
- `RuleUnknownElement`: generated unmarshal errors on unexpected element?

**How validation levels affect generated code:**
- `ValidationError`: generated `New*()` returns error, `Unmarshal*` returns error
- `ValidationWarn`: generated code calls `slog.Warn()` but accepts the value
- `ValidationOff`: no validation code generated for that rule (smaller binary)

#### Config File Support (`goxsd3.yaml`)

Both schema-parsing and data-parsing strictness can be configured via a YAML
config file, allowing teams to share and version-control their validation rules.

```yaml
# goxsd3.yaml — validation configuration

# Schema-parsing strictness: how strictly to validate XSD schema files
schema_strictness:
  default: error           # error | warn | off
  rules:
    facet_cross_validation: error    # reject contradictory facets
    facet_narrowing: error           # reject widening restrictions
    facet_applicability: error       # reject inapplicable facets
    default_value: warn              # warn on invalid defaults, don't reject
    fixed_value: warn                # warn on invalid fixed values
    duplicate_definition: error      # reject duplicate names after include
    circular_derivation: error       # reject circular type derivation

# Data-parsing strictness: how strictly generated code validates runtime data
data_strictness:
  default: error
  rules:
    pattern: warn                    # warn on pattern mismatch, accept value
    enumeration: error               # reject values not in enum set
    min_length: error
    max_length: error
    min_inclusive: error
    max_inclusive: error
    min_exclusive: error
    max_exclusive: error
    total_digits: off                # don't generate totalDigits checks
    fraction_digits: off             # don't generate fractionDigits checks
    whitespace: off                  # don't generate whitespace normalization
    length: error
    required_element: error
    required_attribute: error
    unknown_element: warn            # warn on unknown elements, don't reject
    unknown_attribute: off           # ignore unknown attributes entirely

# Type generation mode
type_mode: strict                    # freeform | strict | mixed
strict_types:                        # only used when type_mode=mixed
  - ZipCode
  - Percentage
  - PartCode

# Code generation options
codegen:
  package: myschema
  output_dir: ./generated
  choice_style: typeswitch           # typeswitch | flat
  generate_xml: true
  generate_json: true
  generate_ber: false
```

```go
// Loading config file:
package config

// Load reads a goxsd3.yaml config file and returns parsed options.
func Load(path string) (*Config, error)

// Config holds all options from the config file.
type Config struct {
    SchemaStrictness ValidationConfig
    DataStrictness   ValidationConfig
    TypeMode         TypeMode
    StrictTypes      []string
    Codegen          CodegenConfig
}

// LoadValidationConfig reads just the validation section from a config file.
// Useful when you only need validation rules (e.g., in a CI pipeline).
func LoadValidationConfig(path string) (*ValidationConfig, *ValidationConfig, error)
```

**Config file resolution order:**
1. Explicit `--config` flag on CLI
2. `goxsd3.yaml` in current directory
3. `goxsd3.yml` in current directory
4. `$HOME/.config/goxsd3/config.yaml` (user-level defaults)
5. Built-in defaults (all rules at `error` level)

### 3.2 Choice → Type Switch (Key Design Decision)

This is the flagship feature. Given:

```xml
<xs:complexType name="Shape">
  <xs:choice>
    <xs:element name="Circle" type="CircleType"/>
    <xs:element name="Square" type="SquareType"/>
    <xs:element name="Triangle" type="TriangleType"/>
  </xs:choice>
</xs:complexType>
```

Generate:

```go
// ShapeChoice is the interface for Shape's choice content.
type ShapeChoice interface {
    isShapeChoice()
}

type ShapeCircle struct {
    Circle CircleType `xml:"Circle"`
}
func (ShapeCircle) isShapeChoice() {}

type ShapeSquare struct {
    Square SquareType `xml:"Square"`
}
func (ShapeSquare) isShapeChoice() {}

type ShapeTriangle struct {
    Triangle TriangleType `xml:"Triangle"`
}
func (ShapeTriangle) isShapeChoice() {}

type Shape struct {
    Choice ShapeChoice
}
```

Marshallers use type switches:

```go
func (s Shape) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
    switch v := s.Choice.(type) {
    case ShapeCircle:
        return e.EncodeElement(v.Circle, xmlStart("Circle"))
    case ShapeSquare:
        return e.EncodeElement(v.Square, xmlStart("Square"))
    case ShapeTriangle:
        return e.EncodeElement(v.Triangle, xmlStart("Triangle"))
    default:
        return fmt.Errorf("unknown ShapeChoice type: %T", v)
    }
}
```

### 3.3 Code Generator (`codegen/codegen.go`)

```go
package codegen

type Generator struct {
    opts   Options
    hooks  []Hook
    tmpl   *template.Template
    logger *slog.Logger
}

// Options includes all fields from 3.1 (TypeMode, Validation, etc.) plus:
//   PackageName, OutputDir, GenerateXML, GenerateJSON, GenerateBER,
//   ChoiceStyle, TypeMode, StrictTypes, Validation

func New(opts Options, hooks ...Hook) *Generator

// Generate produces Go source files from the schema set.
func (g *Generator) Generate(schemas *xsd.SchemaSet) ([]*GeneratedFile, error)
```

### 3.4 Codegen Hooks (`codegen/hooks.go`)

```go
type Hook interface {
    // Modify struct fields before template rendering.
    OnStructFields(ctx *HookContext, typeName string, fields []Field) ([]Field, error)
    // Modify the entire generated file before formatting.
    OnFileGenerated(ctx *HookContext, filename string, source []byte) ([]byte, error)
    // Add extra methods/functions for a type.
    OnTypeGenerated(ctx *HookContext, typeName string) ([]ExtraDecl, error)
}
```

### Test Plan for Phase 3

1. Parse each test fixture from Phase 2 → generate Go → compile (`go build`)
2. **Golden file tests**: compare generated output against checked-in `.go.golden` files
3. **Round-trip tests**: generate code → compile → create instance → marshal → unmarshal → compare
4. Specific tests:
   - Choice generates interface + concrete types
   - Extension generates embedded structs
   - Enums generate typed constants
   - Optional elements generate pointers
   - Repeated elements generate slices
   - Groups are properly inlined

---

## Phase 4: Marshallers/Unmarshallers

### 4.1 XML Marshal/Unmarshal (`marshal/xml.go`)

- Generate `MarshalXML`/`UnmarshalXML` methods for complex types
- Choice types use type switch in marshal; peek-ahead in unmarshal
- Handle namespaces, attributes, mixed content
- Handle `xs:any` (passthrough raw XML)

### 4.2 JSON Marshal/Unmarshal (`marshal/json.go`)

- Generate `MarshalJSON`/`UnmarshalJSON` methods
- Choice → discriminated union with `"type"` field:
  ```json
  {"type": "Circle", "value": {"radius": 5}}
  ```
- Or use Go's default JSON with wrapper objects (configurable)

### 4.3 BER Marshal/Unmarshal (`marshal/ber.go`)

- Generate ASN.1 BER encoding/decoding
- Map XSD types to ASN.1 types (string→UTF8String, int→INTEGER, etc.)
- Choice → CHOICE (context-tagged alternatives)
- Sequence → SEQUENCE
- Use struct tags compatible with `encoding/asn1` or `Logicalis/asn1`

### 4.4 Marshal Hooks (`marshal/hooks.go`)

```go
type Hook interface {
    // Transform a value before marshalling.
    OnMarshal(ctx *HookContext, typeName string, value interface{}) (interface{}, error)
    // Transform raw data before unmarshalling.
    OnUnmarshal(ctx *HookContext, typeName string, data []byte) ([]byte, error)
}
```

### Test Plan for Phase 4

For each marshaller format:
1. Simple struct round-trip (marshal → unmarshal → compare)
2. Choice type round-trip
3. Nested types round-trip
4. Attributes in XML
5. Namespace handling in XML
6. Enum validation
7. Optional/nillable field handling
8. Cross-format: parse XML → marshal to JSON → unmarshal JSON → marshal to XML → compare

---

## Phase 5: Plugin System

### 5.1 Plugin Interface (`plugin/plugin.go`)

```go
package plugin

// Plugin is the unified interface for extending goxsd3.
type Plugin interface {
    Name() string
    // Return hook implementations (nil for phases you don't care about).
    ParserHook() parser.Hook
    CodegenHook() codegen.Hook
    MarshalHook() marshal.Hook
}

// Registry holds registered plugins.
type Registry struct {
    plugins []Plugin
}

func (r *Registry) Register(p Plugin)
```

### 5.2 Custom Element/Attribute Handlers

XSD allows `<xs:appinfo>` and custom attributes in foreign namespaces. Plugins can:
- React to `<xs:appinfo>` content (e.g., `<myns:goName>FooBar</myns:goName>` to override Go names)
- React to custom attributes (e.g., `myns:jsonOmit="true"`)
- Add validation logic
- Modify generated output

### Test Plan for Phase 5

1. Write a sample plugin that renames types (e.g., PascalCase override)
2. Write a sample plugin that adds custom struct tags
3. Write a sample plugin that injects validation methods
4. Verify plugin execution order is deterministic
5. Verify plugins can abort generation with errors

---

## Phase 6: CLI (`cmd/goxsd3`)

```
goxsd3 generate \
  --input schema.xsd \
  --output ./generated \
  --package myschema \
  --config goxsd3.yaml \           # config file (optional, auto-detected)
  --xml \                          # generate XML marshallers (default)
  --json \                         # generate JSON marshallers
  --ber \                          # generate BER marshallers
  --choice=typeswitch \            # choice strategy (typeswitch|flat)
  --type-mode=freeform \           # freeform|strict|mixed
  --strict-types="ZipCode,Percentage" \  # types to make strict (mixed mode)
  --schema-strictness=error \      # default schema-parsing strictness
  --data-strictness=error \        # default data-parsing strictness
  --log-level=info \               # slog level (debug|info|warn|error)
  --log-format=text                # slog format (text|json)

# CLI flags override config file values. Config file provides defaults.
# Per-rule overrides are only available via config file (too verbose for CLI).
```

### Test Plan for Phase 6

1. CLI parses flags correctly
2. Config file loading: explicit `--config`, auto-detect `goxsd3.yaml`, user-level
3. CLI flags override config file values
4. CLI reads XSD files and produces Go files
5. `go build` succeeds on generated output
6. Strict mode generates validated types, freeform generates plain types
7. Mixed mode applies strict only to specified types
8. Schema-parsing strictness: lenient mode accepts invalid schemas with warnings
9. Data-parsing strictness: generated code respects per-rule levels

---

## Implementation Order (Streaming-First, Build Outward)

### Sprint 1: Foundation + Built-in Type Registry
- [ ] `go.mod` init (require Go 1.21+ for `log/slog`)
- [ ] `xsd/` — QName, TypeRef, Namespace helpers
- [ ] `xsd/builtin.go` — Built-in type registry with all 49 types, hfp: facet definitions
- [ ] `xsd/facets.go` — Facet kinds, applicability table, cross-validation rules
- [ ] `xsd/validate.go` — Default/fixed value validation, facet narrowing checks
- [ ] `xsd/builtin_test.go` — All 19 built-in type test functions (see Phase 0.6)
- [ ] Facet cross-validation tests (minLength > maxLength, etc.)
- [ ] Verify: every built-in type present, hierarchy correct, facet applicability matches spec

### Sprint 2: Core Model Types
- [ ] `xsd/model.go` — Schema, Element, Attribute, Import, Include, Annotation
- [ ] `xsd/types.go` — Type interface, SimpleType (restriction/list/union), ComplexType
- [ ] `xsd/compositor.go` — Compositor interface, Sequence, Choice, All, Particle interface
- [ ] `xsd/constraint.go` — Restriction, Facet, Assertion
- [ ] Unit tests for model construction and traversal

### Sprint 3: Streaming Parser (THE FOUNDATION)
- [ ] `parser/streaming.go` — Event types, StreamHandler interface, StreamParser
- [ ] StreamParser implementation using `encoding/xml` token reader
- [ ] `slog` integration — structured logging for all parser events
- [ ] Parse simple elements → emit EventElementStart/End → test: `simple_element.xsd`
- [ ] Parse simpleType → emit EventSimpleTypeStart/End + EventFacet → test: `simple_type.xsd`
- [ ] Parse complexType with sequence → emit compositor events → test: `complex_type.xsd`
- [ ] Parse attributes → emit EventAttribute → test: `attributes.xsd`
- [ ] Stream all 49 built-in types → test: `all_primitives.xsd`, `all_derived.xsd`
- [ ] Tests: event ordering, depth tracking, location tracking

### Sprint 4: Handlers (Stream → DOM)
- [ ] `parser/handler.go` — CollectingHandler, FilteringHandler, TypeListHandler, etc.
- [ ] CollectingHandler: builds xsd.Schema from stream events
- [ ] Incremental symbol table: connect type refs as definitions arrive
- [ ] Forward reference resolution (pendingRefs list, resolved at end)
- [ ] `parser/parser.go` — DOM parser as thin wrapper (StreamParser + CollectingHandler)
- [ ] FilteringHandler — selective event forwarding
- [ ] MultiHandler — fan-out to multiple handlers
- [ ] Tests: `stream_vs_dom.xsd` — parse both ways, verify identical models
- [ ] Tests: `large_schema.xsd` event counts, `filter_test.xsd`

### Sprint 5: Type Derivation, Facets & Validation
- [ ] `config/validation.go` — ValidationConfig, ValidationLevel, ValidationRule types
- [ ] Schema-parsing strictness: integrate ValidationConfig into parser Options
- [ ] SimpleType restriction with facet validation (check facet applicability via registry)
- [ ] Facet cross-validation (minLength ≤ maxLength, minInclusive ≤ maxInclusive, etc.)
- [ ] Default/fixed value validation against declared types
- [ ] All schema validation respects per-rule strictness (error/warn/off)
- [ ] Tests: `restrict_string.xsd`, `restrict_integer.xsd`, `restrict_decimal.xsd`
- [ ] Invalid facet application detection → test: `invalid_facet.xsd`
- [ ] Facet cross-validation tests → test: `invalid_facet_combo.xsd`
- [ ] Default value validation tests → test: `invalid_defaults.xsd`
- [ ] Chained restriction (A restricts B restricts built-in) → test: `chained_restriction.xsd`
- [ ] SimpleType list and union → tests: `list_type.xsd`, `union_type.xsd`
- [ ] ComplexType extension (complexContent + extension) → test: `complex_extension.xsd`
- [ ] ComplexType restriction (complexContent + restriction) → test: `complex_restriction.xsd`
- [ ] Multi-level inheritance (A extends B extends C) → test: `multi_level.xsd`
- [ ] Abstract types with concrete subtypes → test: `abstract_base.xsd`

### Sprint 6: Choice & Nested Compositors
- [ ] Parse `xs:choice` → test: `basic_choice.xsd`
- [ ] Parse nested compositors:
  - [ ] Choice inside sequence → test: `nested_choice.xsd`
  - [ ] Sequence inside choice
  - [ ] Choice inside choice → test: `choice_in_choice.xsd`
  - [ ] Deep nesting (4+ levels) → test: `deep_nesting.xsd`
  - [ ] Mixed compositors → test: `mixed_compositors.xsd`
- [ ] `xs:all` with maxOccurs > 1 (XSD 1.1) → test: `sequence_in_all.xsd`

### Sprint 7: Advanced Features
- [ ] Model groups (`xs:group`) and attribute groups
- [ ] `xs:any` and `xs:anyAttribute`
- [ ] Substitution groups
- [ ] Tests for each

### Sprint 8: Import, Include & Schema Composition
- [ ] `SchemaResolver` interface (location + baseURI + namespace → []byte)
- [ ] `FileResolver` — resolve relative to importing schema's directory
- [ ] `HTTPResolver` — fetch from HTTP/HTTPS with in-memory cache
- [ ] `CatalogResolver` — OASIS XML Catalog lookup
- [ ] `MultiResolver` — chain resolvers in order, first success wins
- [ ] `parser/import.go` — xs:import via FollowImportsHandler (synchronous)
- [ ] `parser/include.go` — xs:include handler with chameleon namespace support
- [ ] Circular import/include detection (visited set by resolved URI)
- [ ] Tests: `simple_import/`, `chameleon_include/`, `circular_import/`, `diamond_import/`
- [ ] `xs:redefine` with self-referencing base → test: `redefine/`
- [ ] `xs:override` (XSD 1.1) → test: `override/`
- [ ] Multi-namespace composition → test: `multi_ns/`
- [ ] `parser/catalog.go` — OASIS XML Catalog support → test: `catalog/`

### Sprint 9: Public Test Suite Integration
- [ ] Download W3C XSD Test Suite (XSTS) subset — focus on schema validation tests
- [ ] Categorize XSTS tests by feature (types, facets, compositors, derivation, etc.)
- [ ] Run XSTS positive tests (valid schemas) → parse succeeds
- [ ] Run XSTS negative tests (invalid schemas) → parse correctly rejects
- [ ] Track pass rate, document known failures
- [ ] Download real-world schemas: SOAP 1.1/1.2, GPX 1.1, KML 2.2, XBRL, UBL, HL7 CDA
- [ ] Parse each real-world schema without error
- [ ] Verify type counts and element counts match expectations

### Sprint 10: Contextual Naming System
- [ ] `codegen/naming.go` — Namer, NameMap, conflict detection
- [ ] Name derivation from element/attribute/group context path
- [ ] Conflict resolution: qualify with parent, then grandparent, then numeric suffix
- [ ] `OrderedMap[K, V]` generic type for stable iteration + O(1) lookup
- [ ] Audit all map usage in parser/codegen: maps used for lookup only, never iterated
- [ ] Determinism tests: parse same schema N times, verify identical name assignments
- [ ] Naming edge case tests (see Phase 3.0.6)
- [ ] Cross-namespace disambiguation tests

### Sprint 11: Strict Type Library (`xsdtypes/`)
- [ ] `xsdtypes/` package — strict wrapper types for built-in XSD types
- [ ] Duration, Date, Time, GYear, GYearMonth, GMonthDay, GDay, GMonth
- [ ] HexBinary, AnyURI, Decimal, Integer, QName
- [ ] FacetError type for validation failures
- [ ] Per-rule ValidationConfig (Error/Warn/Off per facet rule)
- [ ] `slog.Warn` integration for ValidationWarn level
- [ ] Tests for each strict type: valid values, boundary values, invalid values

### Sprint 12: Basic Code Generation (Freeform Mode)
- [ ] `codegen/naming.go` integrated — Namer runs before codegen
- [ ] Type mapping (XSD built-in → Go, using BuiltinRegistry.GoType)
- [ ] Struct generation from complexType + sequence
- [ ] Anonymous type naming via Namer + NameMap
- [ ] Pointer/slice for optional/repeated
- [ ] User-defined simpleType → named Go type alias (freeform, no validation)
- [ ] `go/format` integration
- [ ] Golden file tests

### Sprint 13: Strict Mode Code Generation
- [ ] Generate strict wrapper types with New*/Must* constructors
- [ ] Generate validation functions based on facets
- [ ] Data-parsing strictness: per-rule validation in generated code (error/warn/off)
- [ ] TypeModeMixed: per-type strict/freeform selection
- [ ] Golden file tests (strict mode variants)

### Sprint 14: Choice Code Generation
- [ ] Interface + concrete types for `xs:choice`
- [ ] Type switch marshal/unmarshal generation
- [ ] Nested choice → nested interfaces / flattened where possible
- [ ] Sequence-in-choice → struct variant
- [ ] Choice-in-sequence → interface field
- [ ] Tests with compilation check

### Sprint 15: Full Codegen
- [ ] Enum generation (typed string constants)
- [ ] Extension → embedded struct (ComplexType inheritance)
- [ ] Group inlining
- [ ] Any → `any` / `xml.Token`
- [ ] Golden file tests for all

### Sprint 16: XML Marshallers
- [ ] Generate `MarshalXML` / `UnmarshalXML`
- [ ] Choice type switch in marshaller
- [ ] Namespace handling
- [ ] Strict mode: validation during unmarshal (per-rule config)
- [ ] Round-trip tests

### Sprint 17: JSON Marshallers
- [ ] Generate `MarshalJSON` / `UnmarshalJSON`
- [ ] Discriminated union for choice types
- [ ] Round-trip tests

### Sprint 18: BER Marshallers
- [ ] ASN.1 type mapping
- [ ] Generate BER marshal/unmarshal
- [ ] Round-trip tests

### Sprint 19: XSD 1.1 Features
- [ ] Assertions (`xs:assert`) — store in model, optionally generate validation
- [ ] Conditional type assignment (`xs:alternative`)
- [ ] Open content (`xs:openContent`)
- [ ] Enhanced wildcards
- [ ] Tests for each

### Sprint 20: Plugin System
- [ ] Hook interfaces finalized
- [ ] Plugin registry
- [ ] Sample plugins (rename, custom tags, validation)
- [ ] Plugin tests

### Sprint 21: Config File & CLI
- [ ] `config/config.go` — Load/parse goxsd3.yaml
- [ ] Config file auto-detection (cwd, then user-level)
- [ ] CLI flags override config file values
- [ ] CLI with flag parsing (`--config`, `--type-mode`, `--schema-strictness`, `--data-strictness`)
- [ ] `slog` handler configuration via CLI (`--log-level`, `--log-format`)
- [ ] End-to-end integration tests
- [ ] Error messages and diagnostics
- [ ] Run full XSTS + real-world test suite as CI gate

---

## Key Design Decisions

### 1. Streaming-First Parser
The streaming parser is the foundation — the DOM parser is built on top via
`CollectingHandler`. This ensures a single XML reading path, makes the streaming
parser the most exercised code path, and enables use cases from fast introspection
to full schema loading with the same battle-tested tokenizer.

### 2. Choice = Type Switch (not flat optional fields)
Existing tools flatten choices into optional fields, losing type safety. We generate interfaces with a type switch, which is idiomatic Go and preserves the XSD semantics.

### 3. Incremental Reference Resolution
As stream events arrive, type definitions are added to the symbol table immediately.
References are resolved eagerly when possible, with forward references collected and
resolved in a final pass. This means the DOM parser needs only one streaming pass
plus a fast resolution sweep, not two full XML parses.

### 4. No Concurrency
The entire library is single-threaded. No goroutines anywhere in parsing, naming,
or codegen. This eliminates non-determinism and makes the code simpler to reason
about. The `SchemaResolver` interface is synchronous: the parser calls it, blocks
until bytes are returned, and continues.

### 5. Strict vs Freeform Types
Users choose between plain Go types (freeform, easy, no overhead) and validated
wrapper types (strict, enforces XSD facets at the Go level). Mixed mode allows
per-type selection.

### 6. Two-Level Validation Strictness
Schema-parsing strictness (how strictly to validate the XSD itself) and
data-parsing strictness (how strictly generated code validates runtime data)
are configured independently, both using the same `ValidationConfig` type.
Per-rule granularity (error/warn/off) for each. Both can be set via a
`goxsd3.yaml` config file or programmatic API.

### 7. Simple Schema Resolution
The `SchemaResolver` interface is minimal: `(location, baseURI, namespace) → []byte`.
Users control how schemas are fetched. Pre-defined resolvers (`FileResolver`,
`HTTPResolver`, `CatalogResolver`) are combined via `MultiResolver`. The parser
never does I/O directly — all external access goes through the resolver.

### 8. Template-Based Codegen
Use `text/template` with `go/format` rather than AST construction. Templates are easier to read, modify, and debug. Plugins can modify the model before template rendering or post-process the output.

### 9. Hooks at Every Phase
Four hook points: Parser → Model → Codegen → Marshal. Each hook can modify, add, or reject. This allows plugins to:
- Add custom struct tags during codegen
- Override type mappings
- Inject validation logic
- Handle proprietary XSD extensions

### 10. BER via Struct Tags
Generate ASN.1 struct tags on the same Go types, so a single struct can marshal to XML, JSON, and BER. Use `asn1:"..."` tags alongside `xml:"..."` and `json:"..."`.

### 11. Deterministic Contextual Naming
Anonymous types are named from their definition context (containing element/type path).
Conflicts are resolved by qualifying with parent names, then numeric suffixes. All
data structures preserve document order — maps are used for O(1) lookup only, never
iterated for output. The `OrderedMap[K,V]` generic type enforces this pattern.
Determinism is tested: same schema parsed N times must produce identical name
assignments.

### 12. Structured Logging with slog
All components use `log/slog` for structured, leveled logging. Log groups
(`parser.stream`, `parser.resolve`, `parser.import`, `parser.validate`)
provide fine-grained control. The logger is injected via Options, defaulting
to `slog.Default()`.

---

## Testing Strategy

### Unit Tests
- Model construction and traversal
- Facet cross-validation (contradictory facets rejected)
- Default/fixed value validation
- Parser: one test per XSD construct (via streaming + DOM)
- Codegen: golden file comparison (both freeform and strict mode)
- Marshallers: round-trip per format
- Strict types: boundary value testing, invalid value rejection

### Integration Tests
- End-to-end: XSD file → parse → generate → compile → instantiate → marshal → unmarshal → compare
- Cross-format: XML → JSON → XML round-trip
- Multi-file schemas with imports
- Stream vs DOM: parse same schema both ways, verify identical results

### Public Conformance Tests

#### W3C XSD Test Suite (XSTS)
The W3C publishes an official test suite for XSD processors:
- **Source**: https://www.w3.org/XML/2004/xml-schema-test-suite/
- **Structure**: categorized by feature (types, facets, compositors, identity constraints, etc.)
- **Positive tests**: valid schemas that must parse successfully
- **Negative tests**: invalid schemas that must be rejected with appropriate errors
- We download a curated subset and run it as part of CI
- Track pass rates per category, document known limitations

```
testdata/w3c/
├── README.md                    # which XSTS version, how to update
├── positive/                    # schemas that must parse
│   ├── datatypes/               # built-in type tests
│   ├── facets/                  # facet application tests
│   ├── compositors/             # sequence/choice/all tests
│   ├── derivation/              # restriction/extension tests
│   └── identity/                # key/keyref/unique tests
├── negative/                    # schemas that must be rejected
│   ├── invalid_facets/          # contradictory or inapplicable facets
│   ├── invalid_derivation/      # illegal restriction/extension
│   └── invalid_defaults/        # default values that violate types
└── results.json                 # expected pass/fail for each test
```

#### Real-World Schema Smoke Tests
Parse well-known public schemas to ensure compatibility:

```
testdata/realworld/
├── soap_1_1.xsd                 # SOAP 1.1 envelope
├── soap_1_2.xsd                 # SOAP 1.2 envelope
├── wsdl_1_1.xsd                 # WSDL 1.1
├── gpx_1_1.xsd                  # GPS Exchange Format
├── kml_2_2.xsd                  # Keyhole Markup Language
├── xbrl_2_1.xsd                 # Financial reporting
├── ubl_2_1/                     # Universal Business Language (multi-file)
│   ├── UBL-Invoice-2.1.xsd
│   └── common/                  # shared types
├── hl7_cda.xsd                  # Health Level 7 Clinical Document
├── svg_1_1.xsd                  # Scalable Vector Graphics
├── xhtml_1_0.xsd                # XHTML
└── expected.json                # expected type/element counts per schema
```

Each real-world schema test verifies:
1. Parsing completes without error
2. Number of types/elements matches expected counts
3. Key types are present and correctly structured
4. Import/include chains resolve correctly

### Benchmarks
- Streaming parser throughput (events/sec on large schemas)
- DOM parser performance (streaming + CollectingHandler)
- Codegen throughput
- Marshal/unmarshal performance vs hand-written code
- Memory usage: streaming vs DOM on large schemas
