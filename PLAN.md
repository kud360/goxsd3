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
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│  XSD Files   │────▶│    Parser    │────▶│   Schema Model   │────▶│   CodeGen    │
│  (.xsd)      │     │  (Phase 1)   │     │   (AST / IR)     │     │  (Phase 2)   │
└──────────────┘     └──────────────┘     └──────────────────┘     └──────────────┘
                           │                      │                       │
                     ┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼──────┐
                     │  Parser   │          │  Model    │          │  CodeGen   │
                     │  Hooks    │          │  Hooks    │          │  Hooks     │
                     └───────────┘          └───────────┘          └────────────┘
                                                                        │
                                                              ┌─────────┼─────────┐
                                                              ▼         ▼         ▼
                                                           XML       JSON       BER
                                                         Marshal   Marshal   Marshal
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
│   ├── compositor.go    # Sequence, Choice, All
│   ├── constraint.go    # Facets, assertions, identity constraints
│   └── namespace.go     # Namespace & import resolution
├── parser/              # XSD parser (XML → model)
│   ├── parser.go        # Main parser
│   ├── resolve.go       # Type/ref resolution, import/include
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
    ├── choice/
    ├── complex/
    ├── imports/
    └── xsd11/
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

### 1.3 Compositors (`xsd/compositor.go`)

```go
// Content is the interface for complex type content models.
type Content interface {
    isContent()
}

type Sequence struct {
    MinOccurs int
    MaxOccurs int
    Particles []Particle // Element | Group | Choice | Sequence | All | Any
}

type Choice struct {
    MinOccurs int
    MaxOccurs int
    Particles []Particle
}

type All struct {
    MinOccurs int
    MaxOccurs int  // XSD 1.1 allows > 1
    Particles []Particle
}

// Particle is anything that can appear in a compositor.
type Particle interface {
    isParticle()
}
```

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

### 2.2 Import/Include Resolution (`parser/resolve.go`)

- Resolve `xs:import` (different namespace, requires schemaLocation or catalog)
- Resolve `xs:include` (same namespace, textual inclusion)
- Resolve `xs:redefine` (include with modifications)
- Handle circular imports via visited set
- Resolve all `ref` and `type` attributes to model pointers

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

1. `testdata/basic/simple_element.xsd` — single element with built-in type
2. `testdata/basic/simple_type.xsd` — simpleType with restriction (enum, pattern)
3. `testdata/basic/complex_type.xsd` — complexType with sequence of elements
4. `testdata/basic/attributes.xsd` — elements with attributes
5. `testdata/choice/basic_choice.xsd` — complexType with choice
6. `testdata/choice/nested_choice.xsd` — choice inside sequence, sequence inside choice
7. `testdata/complex/extension.xsd` — type extension (inheritance)
8. `testdata/complex/restriction.xsd` — type restriction
9. `testdata/complex/group.xsd` — model groups and attribute groups
10. `testdata/complex/any.xsd` — xs:any and xs:anyAttribute
11. `testdata/complex/substitution.xsd` — substitution groups
12. `testdata/imports/main.xsd` + `testdata/imports/types.xsd` — import/include
13. `testdata/xsd11/assert.xsd` — assertions
14. `testdata/xsd11/alternative.xsd` — conditional type assignment
15. `testdata/xsd11/open_content.xsd` — open content model

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

### Sprint 1: Foundation
- [ ] `go.mod` init
- [ ] `xsd/` — Core model types (Element, Attribute, SimpleType, ComplexType, Sequence, Choice, All)
- [ ] `xsd/` — QName, TypeRef helpers
- [ ] Unit tests for model construction

### Sprint 2: Basic Parser
- [ ] `parser/` — Parse single XSD file with simple elements and built-in types
- [ ] Test: `simple_element.xsd`
- [ ] Add simpleType parsing (restriction with enum, pattern)
- [ ] Test: `simple_type.xsd`
- [ ] Add complexType with sequence
- [ ] Test: `complex_type.xsd`
- [ ] Add attribute parsing
- [ ] Test: `attributes.xsd`

### Sprint 3: Choice & Compositors
- [ ] Parse `xs:choice`
- [ ] Parse nested compositors (choice in sequence, sequence in choice)
- [ ] Tests: `basic_choice.xsd`, `nested_choice.xsd`

### Sprint 4: Advanced Type Features
- [ ] Type extension/restriction
- [ ] Model groups (`xs:group`) and attribute groups
- [ ] `xs:any` and `xs:anyAttribute`
- [ ] Substitution groups
- [ ] Tests for each

### Sprint 5: Schema Composition
- [ ] `xs:import` and `xs:include` resolution
- [ ] Circular import handling
- [ ] `xs:redefine`
- [ ] Multi-file tests

### Sprint 6: Basic Code Generation
- [ ] Type mapping (XSD built-in → Go)
- [ ] Struct generation from complexType + sequence
- [ ] Pointer/slice for optional/repeated
- [ ] `go/format` integration
- [ ] Golden file tests

### Sprint 7: Choice Code Generation
- [ ] Interface + concrete types for `xs:choice`
- [ ] Type switch marshal/unmarshal generation
- [ ] Nested choice handling
- [ ] Tests with compilation check

### Sprint 8: Full Codegen
- [ ] Enum generation (typed string constants)
- [ ] Extension → embedded struct
- [ ] Group inlining
- [ ] Any → `interface{}` / `xml.Token`
- [ ] Golden file tests for all

### Sprint 9: XML Marshallers
- [ ] Generate `MarshalXML` / `UnmarshalXML`
- [ ] Choice type switch in marshaller
- [ ] Namespace handling
- [ ] Round-trip tests

### Sprint 10: JSON Marshallers
- [ ] Generate `MarshalJSON` / `UnmarshalJSON`
- [ ] Discriminated union for choice types
- [ ] Round-trip tests

### Sprint 11: BER Marshallers
- [ ] ASN.1 type mapping
- [ ] Generate BER marshal/unmarshal
- [ ] Round-trip tests

### Sprint 12: XSD 1.1 Features
- [ ] Assertions (`xs:assert`) — store in model, optionally generate validation
- [ ] Conditional type assignment (`xs:alternative`)
- [ ] Open content (`xs:openContent`)
- [ ] Enhanced wildcards
- [ ] Tests for each

### Sprint 13: Plugin System
- [ ] Hook interfaces finalized
- [ ] Plugin registry
- [ ] Sample plugins (rename, custom tags, validation)
- [ ] Plugin tests

### Sprint 14: CLI & Polish
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
