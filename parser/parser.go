package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/kud360/goxsd3/xsd"
)

// Parser reads XSD documents using xml.Decoder and builds the schema model.
type Parser struct {
	opts    Options
	schemas []*xsd.Schema
	visited map[string]bool
	logger  *slog.Logger
	builtin *xsd.BuiltinRegistry
}

// New creates a new Parser with the given options.
func New(opts ...Option) *Parser {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}
	o.defaults()
	return &Parser{
		opts:    o,
		visited: make(map[string]bool),
		logger:  o.Logger,
		builtin: xsd.NewBuiltinRegistry(),
	}
}

// Parse parses one or more XSD files and returns the schema set.
// Each file is parsed as a separate document. xs:import and xs:include
// directives are recorded but not followed in this sprint.
func (p *Parser) Parse(files ...string) (*xsd.SchemaSet, error) {
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", file, err)
		}
		_, err = p.parseOne(f, file)
		f.Close()
		if err != nil {
			return nil, err
		}
	}

	ss := xsd.NewSchemaSet()
	for _, s := range p.schemas {
		ss.AddSchema(s)
		for _, t := range s.Types {
			ss.RegisterType(t)
		}
		for _, e := range s.Elements {
			ss.RegisterElement(e)
		}
	}
	p.logger.Info("parsed schemas", "count", len(p.schemas))
	return ss, nil
}

// ParseReader parses from an io.Reader as a single entry-point document.
func (p *Parser) ParseReader(r io.Reader, systemID string) (*xsd.SchemaSet, error) {
	if _, err := p.parseOne(r, systemID); err != nil {
		return nil, err
	}

	ss := xsd.NewSchemaSet()
	for _, s := range p.schemas {
		ss.AddSchema(s)
		for _, t := range s.Types {
			ss.RegisterType(t)
		}
		for _, e := range s.Elements {
			ss.RegisterElement(e)
		}
	}
	return ss, nil
}

// buildContext tracks the current parsing context stack.
type buildContext struct {
	kind       string // "schema", "element", "complexType", "simpleType", "sequence", "choice", "all", etc.
	schema     *xsd.Schema
	element    *xsd.Element
	complexTyp *xsd.ComplexType
	simpleTyp  *xsd.SimpleType
	compositor compositorBuilder
	content    contentBuilder
	restrict   *xsd.Restriction
	extension  *xsd.Extension
	annotation *xsd.Annotation
}

// compositorBuilder accumulates particles for a compositor being built.
type compositorBuilder struct {
	kind      string // "sequence", "choice", "all"
	minOccurs int
	maxOccurs int
	items     []xsd.Particle
}

// contentBuilder tracks simpleContent/complexContent being built.
type contentBuilder struct {
	kind string // "simpleContent" or "complexContent"
	sc   *xsd.SimpleContent
	cc   *xsd.ComplexContent
}

// parseOne reads a single XSD document and builds one xsd.Schema.
func (p *Parser) parseOne(r io.Reader, systemID string) (*xsd.Schema, error) {
	if p.visited[systemID] {
		return nil, nil
	}
	p.visited[systemID] = true

	lr := NewLocatingReader(r)
	dec := xml.NewDecoder(lr)

	schema := xsd.NewSchema("")
	schema.Location = systemID

	var stack []*buildContext

	push := func(ctx *buildContext) {
		stack = append(stack, ctx)
	}
	peek := func() *buildContext {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}
	pop := func() *buildContext {
		if len(stack) == 0 {
			return nil
		}
		ctx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return ctx
	}

	loc := func() xsd.Location {
		return lr.Location(dec.InputOffset(), systemID)
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			l := loc()
			return nil, fmt.Errorf("%s:%d:%d: %w", l.SystemID, l.Line, l.Col, err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			// Only handle elements in the XSD namespace.
			if t.Name.Space != xsd.XSDNS {
				continue
			}

			switch t.Name.Local {
			case "schema":
				p.parseSchemaAttrs(schema, t.Attr)
				push(&buildContext{kind: "schema", schema: schema})

			case "element":
				elem := p.buildElement(t.Attr, loc(), schema)
				push(&buildContext{kind: "element", element: elem})

			case "complexType":
				ct := p.buildComplexType(t.Attr, loc(), schema)
				push(&buildContext{kind: "complexType", complexTyp: ct})

			case "simpleType":
				st := p.buildSimpleType(t.Attr, loc(), schema)
				push(&buildContext{kind: "simpleType", simpleTyp: st})

			case "sequence":
				cb := p.buildCompositorStart("sequence", t.Attr)
				push(&buildContext{kind: "sequence", compositor: cb})

			case "choice":
				cb := p.buildCompositorStart("choice", t.Attr)
				push(&buildContext{kind: "choice", compositor: cb})

			case "all":
				cb := p.buildCompositorStart("all", t.Attr)
				push(&buildContext{kind: "all", compositor: cb})

			case "attribute":
				attr := p.buildAttribute(t.Attr, loc())
				// Add to nearest complexType or extension.
				p.addAttributeToContext(attr, stack)

			case "restriction":
				r := p.buildRestriction(t.Attr, schema)
				push(&buildContext{kind: "restriction", restrict: r})

			case "extension":
				ext := p.buildExtension(t.Attr, schema)
				push(&buildContext{kind: "extension", extension: ext})

			case "simpleContent":
				sc := &xsd.SimpleContent{}
				push(&buildContext{kind: "simpleContent", content: contentBuilder{kind: "simpleContent", sc: sc}})

			case "complexContent":
				mixed := attrVal(t.Attr, "mixed") == "true"
				cc := &xsd.ComplexContent{Mixed: mixed}
				push(&buildContext{kind: "complexContent", content: contentBuilder{kind: "complexContent", cc: cc}})

			case "annotation":
				ann := &xsd.Annotation{}
				push(&buildContext{kind: "annotation", annotation: ann})

			case "documentation":
				// Content will be collected in CharData handler. For now, we
				// use xml.Decoder to skip to the end and grab inner text.
				var text string
				if inner, err := collectInnerText(dec); err == nil {
					text = inner
				}
				if ctx := peek(); ctx != nil && ctx.kind == "annotation" && ctx.annotation != nil {
					ctx.annotation.Documentation = append(ctx.annotation.Documentation, text)
				}
				continue // already consumed the end element

			case "appinfo":
				var text string
				if inner, err := collectInnerText(dec); err == nil {
					text = inner
				}
				if ctx := peek(); ctx != nil && ctx.kind == "annotation" && ctx.annotation != nil {
					ctx.annotation.AppInfo = append(ctx.annotation.AppInfo, text)
				}
				continue

			case "import":
				imp := p.buildImport(t.Attr, loc())
				schema.Imports = append(schema.Imports, imp)
				p.logger.Debug("recorded import",
					slog.String("namespace", imp.Namespace),
					slog.String("location", imp.SchemaLocation))

			case "include":
				inc := p.buildInclude(t.Attr, loc())
				schema.Includes = append(schema.Includes, inc)
				p.logger.Debug("recorded include",
					slog.String("location", inc.SchemaLocation))

			// Facet elements inside restriction.
			case "enumeration", "pattern", "minLength", "maxLength", "length",
				"minInclusive", "maxInclusive", "minExclusive", "maxExclusive",
				"totalDigits", "fractionDigits", "whiteSpace":
				facet := p.buildFacet(t.Name.Local, t.Attr)
				p.addFacetToContext(facet, stack)

			case "list":
				if ctx := peek(); ctx != nil && ctx.kind == "simpleType" && ctx.simpleTyp != nil {
					itemType := p.resolveQName(attrVal(t.Attr, "itemType"), schema)
					ctx.simpleTyp.List = &xsd.ListType{
						ItemType: xsd.TypeRef{Name: itemType},
					}
				}

			case "union":
				if ctx := peek(); ctx != nil && ctx.kind == "simpleType" && ctx.simpleTyp != nil {
					memberTypesStr := attrVal(t.Attr, "memberTypes")
					var memberTypes []xsd.TypeRef
					if memberTypesStr != "" {
						for _, mt := range strings.Fields(memberTypesStr) {
							qn := p.resolveQName(mt, schema)
							memberTypes = append(memberTypes, xsd.TypeRef{Name: qn})
						}
					}
					ctx.simpleTyp.Union = &xsd.UnionType{MemberTypes: memberTypes}
				}

			case "group":
				name := attrVal(t.Attr, "name")
				ref := attrVal(t.Attr, "ref")
				if name != "" {
					g := &xsd.Group{Name: name, Location: loc()}
					push(&buildContext{kind: "group"})
					// We'll add the group to the schema on EndElement.
					_ = g
				} else if ref != "" {
					qn := p.resolveQName(ref, schema)
					gr := &xsd.GroupRef{
						Ref:       qn,
						MinOccurs: parseOccurs(attrVal(t.Attr, "minOccurs"), 1),
						MaxOccurs: parseOccurs(attrVal(t.Attr, "maxOccurs"), 1),
					}
					p.addParticleToContext(gr, stack)
				}

			case "any":
				any := &xsd.Any{
					Namespace:       attrVal(t.Attr, "namespace"),
					ProcessContents: attrValDefault(t.Attr, "processContents", "strict"),
					MinOccurs:       parseOccurs(attrVal(t.Attr, "minOccurs"), 1),
					MaxOccurs:       parseOccurs(attrVal(t.Attr, "maxOccurs"), 1),
				}
				p.addParticleToContext(any, stack)
			}

		case xml.EndElement:
			if t.Name.Space != xsd.XSDNS {
				continue
			}

			switch t.Name.Local {
			case "schema":
				pop()

			case "element":
				ctx := pop()
				if ctx == nil || ctx.element == nil {
					continue
				}
				elem := ctx.element
				p.addElementToContext(elem, stack, schema)

			case "complexType":
				ctx := pop()
				if ctx == nil || ctx.complexTyp == nil {
					continue
				}
				ct := ctx.complexTyp
				p.addTypeToContext(ct, stack, schema)
				p.logger.Debug("parsed complexType",
					slog.String("name", ct.Name.Local),
					slog.String("namespace", ct.Name.Namespace))

			case "simpleType":
				ctx := pop()
				if ctx == nil || ctx.simpleTyp == nil {
					continue
				}
				st := ctx.simpleTyp
				p.addTypeToContext(st, stack, schema)
				p.logger.Debug("parsed simpleType",
					slog.String("name", st.Name.Local))

			case "sequence", "choice", "all":
				ctx := pop()
				if ctx == nil {
					continue
				}
				comp := p.buildCompositorEnd(ctx.compositor)
				p.addCompositorToContext(comp, stack)

			case "restriction":
				ctx := pop()
				if ctx == nil || ctx.restrict == nil {
					continue
				}
				p.addRestrictionToContext(ctx.restrict, stack)

			case "extension":
				ctx := pop()
				if ctx == nil || ctx.extension == nil {
					continue
				}
				p.addExtensionToContext(ctx.extension, stack)

			case "simpleContent":
				ctx := pop()
				if ctx == nil {
					continue
				}
				p.addSimpleContentToContext(ctx.content.sc, stack)

			case "complexContent":
				ctx := pop()
				if ctx == nil {
					continue
				}
				p.addComplexContentToContext(ctx.content.cc, stack)

			case "annotation":
				ctx := pop()
				if ctx == nil || ctx.annotation == nil {
					continue
				}
				p.addAnnotationToContext(ctx.annotation, stack, schema)
			}
		}
	}

	p.schemas = append(p.schemas, schema)
	p.logger.Info("parsed document",
		slog.String("systemID", systemID),
		slog.String("namespace", schema.TargetNamespace),
		slog.Int("elements", len(schema.Elements)),
		slog.Int("types", len(schema.Types)))
	return schema, nil
}

// ---------------------------------------------------------------------------
// Schema attribute parsing
// ---------------------------------------------------------------------------

func (p *Parser) parseSchemaAttrs(s *xsd.Schema, attrs []xml.Attr) {
	for _, a := range attrs {
		switch {
		case a.Name.Local == "targetNamespace":
			s.TargetNamespace = a.Value
		case a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns"):
			prefix := a.Name.Local
			if a.Name.Space == "" {
				prefix = "" // default namespace
			}
			s.Namespaces[prefix] = a.Value
		}
	}
}

// ---------------------------------------------------------------------------
// Element builders
// ---------------------------------------------------------------------------

func (p *Parser) buildElement(attrs []xml.Attr, loc xsd.Location, schema *xsd.Schema) *xsd.Element {
	elem := &xsd.Element{
		Namespace: schema.TargetNamespace,
		MinOccurs: 1,
		MaxOccurs: 1,
		Location:  loc,
	}
	for _, a := range attrs {
		switch a.Name.Local {
		case "name":
			elem.Name = a.Value
		case "type":
			elem.Type = xsd.TypeRef{Name: p.resolveQName(a.Value, schema)}
		case "minOccurs":
			elem.MinOccurs = parseOccurs(a.Value, 1)
		case "maxOccurs":
			elem.MaxOccurs = parseOccurs(a.Value, 1)
		case "nillable":
			elem.Nillable = a.Value == "true"
		case "abstract":
			elem.Abstract = a.Value == "true"
		case "default":
			v := a.Value
			elem.Default = &v
		case "fixed":
			v := a.Value
			elem.Fixed = &v
		case "substitutionGroup":
			qn := p.resolveQName(a.Value, schema)
			elem.SubstitutionGroup = &qn
		}
	}
	return elem
}

func (p *Parser) buildAttribute(attrs []xml.Attr, loc xsd.Location) *xsd.Attribute {
	attr := &xsd.Attribute{
		Use:      xsd.AttributeOptional,
		Location: loc,
	}
	for _, a := range attrs {
		switch a.Name.Local {
		case "name":
			attr.Name = a.Value
		case "type":
			// Attribute types are always in XSD namespace context; simplified for now.
			attr.Type = xsd.TypeRef{Name: xsd.XSDName(a.Value)}
		case "use":
			attr.Use = xsd.AttributeUse(a.Value)
		case "default":
			v := a.Value
			attr.Default = &v
		case "fixed":
			v := a.Value
			attr.Fixed = &v
		}
	}
	return attr
}

// ---------------------------------------------------------------------------
// Type builders
// ---------------------------------------------------------------------------

func (p *Parser) buildComplexType(attrs []xml.Attr, loc xsd.Location, schema *xsd.Schema) *xsd.ComplexType {
	ct := &xsd.ComplexType{Location: loc}
	for _, a := range attrs {
		switch a.Name.Local {
		case "name":
			ct.Name = xsd.NewQName(schema.TargetNamespace, a.Value)
		case "abstract":
			ct.Abstract = a.Value == "true"
		case "mixed":
			ct.Mixed = a.Value == "true"
		}
	}
	return ct
}

func (p *Parser) buildSimpleType(attrs []xml.Attr, loc xsd.Location, schema *xsd.Schema) *xsd.SimpleType {
	st := &xsd.SimpleType{Location: loc}
	for _, a := range attrs {
		if a.Name.Local == "name" {
			st.Name = xsd.NewQName(schema.TargetNamespace, a.Value)
		}
	}
	return st
}

// ---------------------------------------------------------------------------
// Compositor builders
// ---------------------------------------------------------------------------

func (p *Parser) buildCompositorStart(kind string, attrs []xml.Attr) compositorBuilder {
	cb := compositorBuilder{
		kind:      kind,
		minOccurs: 1,
		maxOccurs: 1,
	}
	for _, a := range attrs {
		switch a.Name.Local {
		case "minOccurs":
			cb.minOccurs = parseOccurs(a.Value, 1)
		case "maxOccurs":
			cb.maxOccurs = parseOccurs(a.Value, 1)
		}
	}
	return cb
}

func (p *Parser) buildCompositorEnd(cb compositorBuilder) xsd.Compositor {
	switch cb.kind {
	case "sequence":
		return &xsd.Sequence{MinOccurs: cb.minOccurs, MaxOccurs: cb.maxOccurs, Items: cb.items}
	case "choice":
		return &xsd.Choice{MinOccurs: cb.minOccurs, MaxOccurs: cb.maxOccurs, Items: cb.items}
	case "all":
		return &xsd.All{MinOccurs: cb.minOccurs, MaxOccurs: cb.maxOccurs, Items: cb.items}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Restriction / Extension / Content builders
// ---------------------------------------------------------------------------

func (p *Parser) buildRestriction(attrs []xml.Attr, schema *xsd.Schema) *xsd.Restriction {
	r := &xsd.Restriction{}
	for _, a := range attrs {
		if a.Name.Local == "base" {
			r.Base = xsd.TypeRef{Name: p.resolveQName(a.Value, schema)}
		}
	}
	return r
}

func (p *Parser) buildExtension(attrs []xml.Attr, schema *xsd.Schema) *xsd.Extension {
	ext := &xsd.Extension{}
	for _, a := range attrs {
		if a.Name.Local == "base" {
			ext.Base = xsd.TypeRef{Name: p.resolveQName(a.Value, schema)}
		}
	}
	return ext
}

func (p *Parser) buildFacet(local string, attrs []xml.Attr) xsd.Facet {
	f := xsd.Facet{Kind: xsd.FacetKind(local)}
	for _, a := range attrs {
		switch a.Name.Local {
		case "value":
			f.Value = a.Value
		case "fixed":
			f.Fixed = a.Value == "true"
		}
	}
	return f
}

func (p *Parser) buildImport(attrs []xml.Attr, loc xsd.Location) *xsd.Import {
	imp := &xsd.Import{Location: loc}
	for _, a := range attrs {
		switch a.Name.Local {
		case "namespace":
			imp.Namespace = a.Value
		case "schemaLocation":
			imp.SchemaLocation = a.Value
		}
	}
	return imp
}

func (p *Parser) buildInclude(attrs []xml.Attr, loc xsd.Location) *xsd.Include {
	inc := &xsd.Include{Location: loc}
	for _, a := range attrs {
		if a.Name.Local == "schemaLocation" {
			inc.SchemaLocation = a.Value
		}
	}
	return inc
}

// ---------------------------------------------------------------------------
// Context wiring — connecting parsed components to their parents
// ---------------------------------------------------------------------------

func (p *Parser) addElementToContext(elem *xsd.Element, stack []*buildContext, schema *xsd.Schema) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "sequence" || ctx.kind == "choice" || ctx.kind == "all":
			ctx.compositor.items = append(ctx.compositor.items, elem)
			return
		case ctx.kind == "schema":
			schema.AddElement(elem)
			return
		}
	}
	// Top-level element with no context.
	schema.AddElement(elem)
}

func (p *Parser) addTypeToContext(t xsd.Type, stack []*buildContext, schema *xsd.Schema) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		if ctx.kind == "element" && ctx.element != nil {
			ctx.element.InlineType = t
			return
		}
	}
	// Top-level named type.
	schema.AddType(t)
}

func (p *Parser) addCompositorToContext(comp xsd.Compositor, stack []*buildContext) {
	if comp == nil {
		return
	}
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "sequence" || ctx.kind == "choice" || ctx.kind == "all":
			// Nested compositor.
			ctx.compositor.items = append(ctx.compositor.items, comp)
			return
		case ctx.kind == "complexType" && ctx.complexTyp != nil:
			ctx.complexTyp.Content = comp
			return
		case ctx.kind == "extension" && ctx.extension != nil:
			ctx.extension.Compositor = comp
			return
		case ctx.kind == "restriction" && ctx.restrict != nil:
			ctx.restrict.Content = comp
			return
		}
	}
}

func (p *Parser) addAttributeToContext(attr *xsd.Attribute, stack []*buildContext) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "complexType" && ctx.complexTyp != nil:
			ctx.complexTyp.Attributes = append(ctx.complexTyp.Attributes, attr)
			return
		case ctx.kind == "extension" && ctx.extension != nil:
			ctx.extension.Attributes = append(ctx.extension.Attributes, attr)
			return
		}
	}
}

func (p *Parser) addRestrictionToContext(r *xsd.Restriction, stack []*buildContext) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "simpleType" && ctx.simpleTyp != nil:
			ctx.simpleTyp.Restriction = r
			return
		case ctx.kind == "simpleContent" && ctx.content.sc != nil:
			ctx.content.sc.Restriction = r
			return
		case ctx.kind == "complexContent" && ctx.content.cc != nil:
			ctx.content.cc.Restriction = r
			return
		}
	}
}

func (p *Parser) addExtensionToContext(ext *xsd.Extension, stack []*buildContext) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "simpleContent" && ctx.content.sc != nil:
			ctx.content.sc.Extension = ext
			return
		case ctx.kind == "complexContent" && ctx.content.cc != nil:
			ctx.content.cc.Extension = ext
			return
		}
	}
}

func (p *Parser) addSimpleContentToContext(sc *xsd.SimpleContent, stack []*buildContext) {
	if sc == nil {
		return
	}
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		if ctx.kind == "complexType" && ctx.complexTyp != nil {
			ctx.complexTyp.Content = sc
			return
		}
	}
}

func (p *Parser) addComplexContentToContext(cc *xsd.ComplexContent, stack []*buildContext) {
	if cc == nil {
		return
	}
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		if ctx.kind == "complexType" && ctx.complexTyp != nil {
			ctx.complexTyp.Content = cc
			return
		}
	}
}

func (p *Parser) addFacetToContext(f xsd.Facet, stack []*buildContext) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		if ctx.kind == "restriction" && ctx.restrict != nil {
			ctx.restrict.Facets = append(ctx.restrict.Facets, f)
			return
		}
	}
}

func (p *Parser) addParticleToContext(particle xsd.Particle, stack []*buildContext) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		if ctx.kind == "sequence" || ctx.kind == "choice" || ctx.kind == "all" {
			ctx.compositor.items = append(ctx.compositor.items, particle)
			return
		}
	}
}

func (p *Parser) addAnnotationToContext(ann *xsd.Annotation, stack []*buildContext, schema *xsd.Schema) {
	for i := len(stack) - 1; i >= 0; i-- {
		ctx := stack[i]
		switch {
		case ctx.kind == "element" && ctx.element != nil:
			ctx.element.Annotations = append(ctx.element.Annotations, ann)
			return
		case ctx.kind == "complexType" && ctx.complexTyp != nil:
			ctx.complexTyp.Annotations = append(ctx.complexTyp.Annotations, ann)
			return
		case ctx.kind == "simpleType" && ctx.simpleTyp != nil:
			ctx.simpleTyp.Annotations = append(ctx.simpleTyp.Annotations, ann)
			return
		case ctx.kind == "schema":
			schema.Annotations = append(schema.Annotations, ann)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// QName resolution
// ---------------------------------------------------------------------------

// resolveQName resolves a prefixed name (like "xs:string") into a QName
// using the schema's namespace map.
func (p *Parser) resolveQName(prefixed string, schema *xsd.Schema) xsd.QName {
	if prefixed == "" {
		return xsd.QName{}
	}
	parts := strings.SplitN(prefixed, ":", 2)
	if len(parts) == 1 {
		// Unprefixed — use target namespace.
		return xsd.NewQName(schema.TargetNamespace, parts[0])
	}
	prefix, local := parts[0], parts[1]
	if ns, ok := schema.Namespaces[prefix]; ok {
		return xsd.NewQName(ns, local)
	}
	// Unknown prefix — use as-is with empty namespace.
	p.logger.Warn("unknown namespace prefix", slog.String("prefix", prefix))
	return xsd.NewQName("", local)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// attrVal returns the value of the named attribute, or "".
func attrVal(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// attrValDefault returns the value of the named attribute, or the default.
func attrValDefault(attrs []xml.Attr, name, dflt string) string {
	v := attrVal(attrs, name)
	if v == "" {
		return dflt
	}
	return v
}

// parseOccurs parses a minOccurs/maxOccurs attribute value.
// "unbounded" returns -1. Empty string returns dflt.
func parseOccurs(s string, dflt int) int {
	if s == "" {
		return dflt
	}
	if s == "unbounded" {
		return -1
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return dflt
	}
	return v
}

// collectInnerText reads all content between the current StartElement and its
// matching EndElement, returning the concatenated text content.
func collectInnerText(dec *xml.Decoder) (string, error) {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return sb.String(), err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			sb.Write(tok.(xml.CharData))
		}
	}
	return sb.String(), nil
}
