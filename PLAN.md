# goxsd3 — XSD 1.1 Parser & Code Generator for Go

## Vision

A Go library and CLI tool that:
1. **Parses** XSD 1.1 schemas into a rich in-memory model (AST)
2. **Generates** idiomatic Go types with `xs:choice` mapped to type switches (interfaces + concrete types)
3. **Optionally generates** JSON, XML, and BER marshallers/unmarshallers
4. **Exposes** the model as a library with hook points for plugins/extensions at every phase

---

## Architecture Overview

```
                     ┌───────────────────┐
                     │  Built-in Types   │
                     │  (hfp: registry)  │
                     └────────┬──────────┘
                              │ bootstrap
┌──────────────┐     ┌───────▼──────┐     ┌──────────────────┐     ┌──────────────┐
│  XSD Files   │────▶│    Parser    │────▶│   Schema Model   │────▶│   CodeGen    │
│  (.xsd)      │     │  (Phase 1)   │     │   (AST / IR)     │     │  (Phase 2)   │
└──────────────┘     └──────────────┘     └──────────────────┘     └──────────────┘
       │                   │                      │                       │
  ┌────▼────┐        ┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼──────┐
  │ import/ │        │  Parser   │          │  Model    │          │  CodeGen   │
  │ include │        │  Hooks    │          │  Hooks    │          │  Hooks     │
  └─────────┘        └───────────┘          └───────────┘          └────────────┘
                                                                        │
                           ┌──────────────────────┐           ┌─────────┼─────────┐
                           │  Streaming Parser     │           ▼         ▼         ▼
                           │  (SAX-style events)   │        XML       JSON       BER
                           └──────────────────────┘       Marshal   Marshal   Marshal
```

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
│   ├── facets.go        # Facet definitions, applicability, inheritance
│   ├── compositor.go    # Sequence, Choice, All (nested support)
│   ├── constraint.go    # Facets, assertions, identity constraints
│   └── namespace.go     # Namespace & import resolution
├── parser/              # XSD parser (XML → model)
│   ├── parser.go        # Main parser (DOM-style, full schema load)
│   ├── streaming.go     # Streaming parser (SAX-style event callbacks)
│   ├── resolve.go       # Type/ref resolution, import/include/redefine
│   ├── import.go        # xs:import handler (cross-namespace)
│   ├── include.go       # xs:include handler (same-namespace)
│   ├── catalog.go       # XML Catalog support for schema resolution
│   ├── options.go       # Parser options
│   └── hooks.go         # Parser hook interfaces
├── codegen/             # Go code generation (model → Go source)
│   ├── codegen.go       # Main code generator
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
    └── xsd11/
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
type Schema struct {
    TargetNamespace string
    Namespaces      map[string]string // prefix → URI
    Elements        []*Element
    Types           []Type           // SimpleType | ComplexType
    Groups          []*Group
    AttributeGroups []*AttributeGroup
    Imports         []*Import
    Includes        []*Include
    Annotations     []*Annotation
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

### 2.1 Core Parser (`parser/parser.go`)

Uses `encoding/xml` to parse XSD files into the model from Phase 1.

**Strategy**: Parse in two passes:
1. **Pass 1**: Parse all schema documents, collect all named types/elements/groups (symbol table)
2. **Pass 2**: Resolve all references (`ref`, `type`, `base`, `substitutionGroup`)

```go
package parser

type Parser struct {
    opts       Options
    hooks      []Hook
    schemas    map[string]*xsd.Schema // namespace → schema
    symbols    SymbolTable
}

func New(opts ...Option) *Parser

// Parse parses one or more XSD files and returns the resolved schema set.
func (p *Parser) Parse(files ...string) (*xsd.SchemaSet, error)

// ParseReader parses from an io.Reader.
func (p *Parser) ParseReader(r io.Reader, systemID string) (*xsd.SchemaSet, error)
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
type ImportResolver interface {
    // Resolve returns the schema content for the given namespace + location hint.
    // Default implementation reads from filesystem relative to the importing schema.
    Resolve(namespace, schemaLocation, baseURI string) (io.ReadCloser, error)
}

// Built-in resolvers:
type FileResolver struct{}     // Filesystem (relative/absolute paths)
type HTTPResolver struct{}     // HTTP/HTTPS fetch (with caching)
type CatalogResolver struct{}  // XML Catalog (OASIS) lookup
type CompositeResolver struct{} // Chain of resolvers (try each in order)
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

```
1. Check pre-loaded schemas (programmatic API)
2. Check XML Catalog (if configured)
3. Try schemaLocation hint (relative to importing schema's base URI)
4. Try well-known locations (e.g., xs namespace → built-in)
5. Call custom ImportResolver hook
6. Error: cannot resolve
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

### 2.3 Streaming Parser (`parser/streaming.go`)

For large schemas or when you only need partial information, the streaming parser
provides SAX-style event callbacks without building the full model in memory.

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
    EventAnnotation
    EventFacet
    EventGroup
    EventAttributeGroup
)

// Event is emitted during streaming parse.
type Event struct {
    Kind       EventKind
    Name       string
    Namespace  string
    Attributes map[string]string
    Depth      int    // Nesting depth
    Location   Location // File + line/col
}

// StreamHandler receives events during streaming parse.
type StreamHandler interface {
    OnEvent(event Event) error
    // OnError is called for non-fatal parse issues (e.g., unresolved ref in streaming mode).
    OnError(err error) error
}

// StreamParser parses XSD without building full model.
type StreamParser struct {
    opts    Options
    handler StreamHandler
}

func NewStreamParser(handler StreamHandler, opts ...Option) *StreamParser

// Stream parses and emits events. Does NOT resolve references (no two-pass).
func (sp *StreamParser) Stream(r io.Reader, systemID string) error

// StreamFile is a convenience for file-based streaming.
func (sp *StreamParser) StreamFile(path string) error
```

**Use cases:**
- Schema introspection (list all types/elements without full parse)
- IDE tooling (autocomplete, hover info)
- Schema validation pre-flight (check syntax without full resolution)
- Custom schema analysis tools via plugins
- Processing schemas too large to fit in memory

**Streaming + DOM interop:**
```go
// CollectingHandler builds a full model from stream events (bridges stream → DOM).
// This is the bridge: you can start streaming, then "promote" to full DOM when needed.
type CollectingHandler struct {
    schema *xsd.Schema
}
func (c *CollectingHandler) Schema() *xsd.Schema // returns built model after streaming

// FilteringHandler wraps another handler and only forwards matching events.
type FilteringHandler struct {
    Inner     StreamHandler
    KindFilter map[EventKind]bool
}

// TypeListHandler collects just type names (fast schema introspection).
type TypeListHandler struct {
    Types []QName
}

// ElementListHandler collects just global element names.
type ElementListHandler struct {
    Elements []QName
}
```

**Streaming parser for import/include:**

When streaming encounters `xs:import` or `xs:include`, it emits `EventImport`/`EventInclude`
with the namespace and schemaLocation. The handler can decide whether to:
1. Ignore (pure streaming, no I/O)
2. Follow (create new StreamParser for the referenced schema)
3. Defer (collect locations, resolve later)

```go
// FollowImportsHandler automatically streams imported/included schemas.
type FollowImportsHandler struct {
    Inner    StreamHandler
    Resolver ImportResolver
    visited  map[string]bool // prevent circular follows
}
```

### 2.4 Parser Hooks (`parser/hooks.go`)

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

### 3.1 Type Mapping Strategy

| XSD Construct | Go Output |
|---|---|
| `xs:string` | `string` |
| `xs:int`, `xs:integer` | `int`, `int64` |
| `xs:boolean` | `bool` |
| `xs:float`/`xs:double` | `float32`/`float64` |
| `xs:dateTime` | `time.Time` |
| `xs:base64Binary` | `[]byte` |
| `xs:hexBinary` | `[]byte` (with custom type) |
| `xs:complexType` (sequence) | Go struct |
| `xs:complexType` (choice) | **Interface + concrete types** |
| `xs:simpleType` (enum) | `type X string` + constants |
| `xs:simpleType` (restriction) | Named type with validation |
| `xs:element` maxOccurs > 1 | `[]T` |
| `xs:element` nillable | `*T` |
| `xs:any` | `[]xml.Token` or `interface{}` |
| `xs:group` | Embedded struct (inlined) |
| `xs:extension` | Embedded base struct |
| `xs:substitutionGroup` | Interface (like choice) |

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
    opts  Options
    hooks []Hook
    tmpl  *template.Template
}

type Options struct {
    PackageName    string
    OutputDir      string
    GenerateXML    bool // XML marshal/unmarshal (default true)
    GenerateJSON   bool // JSON marshal/unmarshal
    GenerateBER    bool // BER/ASN.1 marshal/unmarshal
    ChoiceStyle    ChoiceStyle // TypeSwitch (default) | FlatOptional
}

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

```go
goxsd3 generate \
  --input schema.xsd \
  --output ./generated \
  --package myschema \
  --xml \              # generate XML marshallers (default)
  --json \             # generate JSON marshallers
  --ber \              # generate BER marshallers
  --choice=typeswitch  # choice strategy (typeswitch|flat)
```

### Test Plan for Phase 6

1. CLI parses flags correctly
2. CLI reads XSD files and produces Go files
3. `go build` succeeds on generated output
4. Integration test: end-to-end from XSD to compiled Go with round-trip marshal

---

## Implementation Order (Build Small → Outward)

### Sprint 1: Foundation + Built-in Type Registry
- [ ] `go.mod` init
- [ ] `xsd/` — QName, TypeRef, Namespace helpers
- [ ] `xsd/builtin.go` — Built-in type registry with all 49 types, hfp: facet definitions
- [ ] `xsd/facets.go` — Facet kinds, applicability table, inheritance rules
- [ ] `xsd/builtin_test.go` — All 19 built-in type test functions (see Phase 0.6)
- [ ] Verify: every built-in type present, hierarchy correct, facet applicability matches spec

### Sprint 2: Core Model Types
- [ ] `xsd/model.go` — Schema, Element, Attribute, Import, Include, Annotation
- [ ] `xsd/types.go` — Type interface, SimpleType (restriction/list/union), ComplexType
- [ ] `xsd/compositor.go` — Compositor interface, Sequence, Choice, All, Particle interface
- [ ] `xsd/constraint.go` — Restriction, Facet, Assertion
- [ ] Unit tests for model construction and traversal

### Sprint 3: Basic Parser
- [ ] `parser/parser.go` — Parse single XSD file, two-pass architecture
- [ ] Parse simple elements with built-in types → test: `simple_element.xsd`
- [ ] Parse simpleType (restriction with enum, pattern) → test: `simple_type.xsd`
- [ ] Parse complexType with sequence → test: `complex_type.xsd`
- [ ] Parse attributes → test: `attributes.xsd`
- [ ] Built-in type resolution: all 49 types parseable → tests: `all_primitives.xsd`, `all_derived.xsd`

### Sprint 4: Type Derivation & Facets
- [ ] SimpleType restriction with facet validation (check facet applicability via registry)
- [ ] Tests: `restrict_string.xsd`, `restrict_integer.xsd`, `restrict_decimal.xsd`
- [ ] Invalid facet application detection → test: `invalid_facet.xsd`
- [ ] Chained restriction (A restricts B restricts built-in) → test: `chained_restriction.xsd`
- [ ] SimpleType list and union → tests: `list_type.xsd`, `union_type.xsd`
- [ ] ComplexType extension (complexContent + extension) → test: `complex_extension.xsd`
- [ ] ComplexType restriction (complexContent + restriction) → test: `complex_restriction.xsd`
- [ ] Multi-level inheritance (A extends B extends C) → test: `multi_level.xsd`
- [ ] Abstract types with concrete subtypes → test: `abstract_base.xsd`

### Sprint 5: Choice & Nested Compositors
- [ ] Parse `xs:choice` → test: `basic_choice.xsd`
- [ ] Parse nested compositors:
  - [ ] Choice inside sequence → test: `nested_choice.xsd`
  - [ ] Sequence inside choice
  - [ ] Choice inside choice → test: `choice_in_choice.xsd`
  - [ ] Deep nesting (4+ levels) → test: `deep_nesting.xsd`
  - [ ] Mixed compositors → test: `mixed_compositors.xsd`
- [ ] `xs:all` with maxOccurs > 1 (XSD 1.1) → test: `sequence_in_all.xsd`

### Sprint 6: Advanced Features
- [ ] Model groups (`xs:group`) and attribute groups
- [ ] `xs:any` and `xs:anyAttribute`
- [ ] Substitution groups
- [ ] Tests for each

### Sprint 7: Import, Include & Schema Composition
- [ ] `parser/import.go` — xs:import handler with ImportResolver interface
- [ ] `parser/include.go` — xs:include handler with chameleon namespace support
- [ ] FileResolver (relative paths from importing schema)
- [ ] Circular import/include detection (visited set by resolved URI)
- [ ] Tests: `simple_import/`, `chameleon_include/`, `circular_import/`, `diamond_import/`
- [ ] `xs:redefine` with self-referencing base → test: `redefine/`
- [ ] `xs:override` (XSD 1.1) → test: `override/`
- [ ] Multi-namespace composition → test: `multi_ns/`
- [ ] `parser/catalog.go` — OASIS XML Catalog support → test: `catalog/`
- [ ] HTTPResolver (fetch remote schemas with caching)
- [ ] CompositeResolver (chain of resolvers)

### Sprint 8: Streaming Parser
- [ ] `parser/streaming.go` — Event types, StreamHandler interface
- [ ] StreamParser implementation using `encoding/xml` token reader
- [ ] FilteringHandler (only forward matching event kinds)
- [ ] CollectingHandler (bridge stream → DOM model)
- [ ] Tests: `large_schema.xsd` event counts, `filter_test.xsd`, `stream_vs_dom.xsd`

### Sprint 9: Basic Code Generation
- [ ] Type mapping (XSD built-in → Go, using BuiltinRegistry.GoType)
- [ ] Struct generation from complexType + sequence
- [ ] Pointer/slice for optional/repeated
- [ ] User-defined simpleType → named Go type with validation
- [ ] `go/format` integration
- [ ] Golden file tests

### Sprint 10: Choice Code Generation
- [ ] Interface + concrete types for `xs:choice`
- [ ] Type switch marshal/unmarshal generation
- [ ] Nested choice → nested interfaces / flattened where possible
- [ ] Sequence-in-choice → struct variant
- [ ] Choice-in-sequence → interface field
- [ ] Tests with compilation check

### Sprint 11: Full Codegen
- [ ] Enum generation (typed string constants)
- [ ] Extension → embedded struct (ComplexType inheritance)
- [ ] Group inlining
- [ ] Any → `interface{}` / `xml.Token`
- [ ] Golden file tests for all

### Sprint 12: XML Marshallers
- [ ] Generate `MarshalXML` / `UnmarshalXML`
- [ ] Choice type switch in marshaller
- [ ] Namespace handling
- [ ] Round-trip tests

### Sprint 13: JSON Marshallers
- [ ] Generate `MarshalJSON` / `UnmarshalJSON`
- [ ] Discriminated union for choice types
- [ ] Round-trip tests

### Sprint 14: BER Marshallers
- [ ] ASN.1 type mapping
- [ ] Generate BER marshal/unmarshal
- [ ] Round-trip tests

### Sprint 15: XSD 1.1 Features
- [ ] Assertions (`xs:assert`) — store in model, optionally generate validation
- [ ] Conditional type assignment (`xs:alternative`)
- [ ] Open content (`xs:openContent`)
- [ ] Enhanced wildcards
- [ ] Tests for each

### Sprint 16: Plugin System
- [ ] Hook interfaces finalized
- [ ] Plugin registry
- [ ] Sample plugins (rename, custom tags, validation)
- [ ] Plugin tests

### Sprint 17: CLI & Polish
- [ ] CLI with flag parsing
- [ ] End-to-end integration tests
- [ ] Error messages and diagnostics
- [ ] Documentation

---

## Key Design Decisions

### 1. Choice = Type Switch (not flat optional fields)
Existing tools flatten choices into optional fields, losing type safety. We generate interfaces with a type switch, which is idiomatic Go and preserves the XSD semantics.

### 2. Two-Pass Parser
Pass 1 collects symbols, Pass 2 resolves references. This handles forward references and circular types cleanly.

### 3. Template-Based Codegen
Use `text/template` with `go/format` rather than AST construction. Templates are easier to read, modify, and debug. Plugins can modify the model before template rendering or post-process the output.

### 4. Hooks at Every Phase
Four hook points: Parser → Model → Codegen → Marshal. Each hook can modify, add, or reject. This allows plugins to:
- Add custom struct tags during codegen
- Override type mappings
- Inject validation logic
- Handle proprietary XSD extensions

### 5. BER via Struct Tags
Generate ASN.1 struct tags on the same Go types, so a single struct can marshal to XML, JSON, and BER. Use `asn1:"..."` tags alongside `xml:"..."` and `json:"..."`.

---

## Testing Strategy

### Unit Tests
- Model construction and traversal
- Parser: one test per XSD construct
- Codegen: golden file comparison
- Marshallers: round-trip per format

### Integration Tests
- End-to-end: XSD file → parse → generate → compile → instantiate → marshal → unmarshal → compare
- Cross-format: XML → JSON → XML round-trip
- Multi-file schemas with imports

### Conformance Tests
- W3C XSD test suite (subset) for parser validation
- Real-world XSD files (SOAP, WSDL, GPX, KML, XBRL) as smoke tests

### Benchmarks
- Parser performance on large schemas
- Codegen throughput
- Marshal/unmarshal performance vs hand-written code
