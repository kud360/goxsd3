package parser

import (
	"fmt"
	"log/slog"

	"github.com/kud360/goxsd3/config"
	"github.com/kud360/goxsd3/xsd"
)

// validateSimpleTypeRestriction checks that a simpleType's restriction is valid:
// - All facets are applicable to the base type family
// - Facets are internally consistent (minLength <= maxLength, etc.)
// - Facets narrow (don't widen) the base type's facets
func (p *Parser) validateSimpleTypeRestriction(st *xsd.SimpleType) []error {
	if st.Restriction == nil {
		return nil
	}

	r := st.Restriction
	baseName := r.Base.Name

	var errs []error

	// 1. Facet applicability — check each facet is valid for the base type.
	if p.opts.SchemaStrictness.Level(config.RuleFacetApplicability) != config.ValidationOff {
		if appErrs := p.validateFacetApplicability(baseName, r.Facets); len(appErrs) > 0 {
			errs = append(errs, p.handleValidationErrors(
				config.RuleFacetApplicability, appErrs, st.Name.Local)...)
		}
	}

	// 2. Facet cross-validation — minLength <= maxLength, etc.
	if p.opts.SchemaStrictness.Level(config.RuleFacetCrossValidation) != config.ValidationOff {
		baseFamily := p.baseFamily(baseName)
		if crossErrs := xsd.ValidateFacetSet(r.Facets, baseFamily); len(crossErrs) > 0 {
			errs = append(errs, p.handleValidationErrors(
				config.RuleFacetCrossValidation, crossErrs, st.Name.Local)...)
		}
	}

	// 3. Facet narrowing — derived type must not widen base facets.
	if p.opts.SchemaStrictness.Level(config.RuleFacetNarrowing) != config.ValidationOff {
		baseFacets := p.resolveBaseFacets(baseName)
		if narrowErrs := xsd.ValidateFacetNarrowing(baseFacets, r.Facets); len(narrowErrs) > 0 {
			errs = append(errs, p.handleValidationErrors(
				config.RuleFacetNarrowing, narrowErrs, st.Name.Local)...)
		}
	}

	return errs
}

// validateElementDefaults checks that default/fixed values are valid for
// the element's declared type.
func (p *Parser) validateElementDefaults(elem *xsd.Element) []error {
	var errs []error

	if elem.Default != nil {
		if p.opts.SchemaStrictness.Level(config.RuleDefaultValue) != config.ValidationOff {
			if err := xsd.ValidateDefaultValue(*elem.Default, elem.Type.Name, p.builtin); err != nil {
				errs = append(errs, p.handleValidationErrors(
					config.RuleDefaultValue,
					[]error{fmt.Errorf("element %q default: %w", elem.Name, err)},
					elem.Name)...)
			}
		}
	}

	if elem.Fixed != nil {
		if p.opts.SchemaStrictness.Level(config.RuleFixedValue) != config.ValidationOff {
			if err := xsd.ValidateDefaultValue(*elem.Fixed, elem.Type.Name, p.builtin); err != nil {
				errs = append(errs, p.handleValidationErrors(
					config.RuleFixedValue,
					[]error{fmt.Errorf("element %q fixed: %w", elem.Name, err)},
					elem.Name)...)
			}
		}
	}

	return errs
}

// validateFacetApplicability checks that each facet is applicable to the
// given base type. Returns errors for inapplicable facets.
func (p *Parser) validateFacetApplicability(baseName xsd.QName, facets []xsd.Facet) []error {
	if len(facets) == 0 {
		return nil
	}

	family := p.baseFamily(baseName)
	if family == "" {
		// Cannot determine the type family; skip validation.
		return nil
	}

	// Check each facet against the family's applicability.
	var bad []xsd.FacetKind
	for _, f := range facets {
		if !xsd.IsFacetApplicable(family, f.Kind) {
			bad = append(bad, f.Kind)
		}
	}
	if len(bad) > 0 {
		return []error{fmt.Errorf("facets not applicable to type family %q: %v", family, bad)}
	}
	return nil
}

// resolveUltimateBase walks the restriction chain from a user-defined type
// to its ultimate built-in base type.
func (p *Parser) resolveUltimateBase(name xsd.QName) xsd.QName {
	visited := make(map[xsd.QName]bool)
	current := name
	for {
		if visited[current] {
			break // circular derivation
		}
		visited[current] = true

		// If it's a built-in type, we're done.
		if p.builtin.Lookup(current) != nil {
			return current
		}

		// Look up the user-defined type and follow its restriction base.
		t := p.symbols.LookupType(current)
		if t == nil {
			break
		}
		st, ok := t.(*xsd.SimpleType)
		if !ok || st.Restriction == nil {
			break
		}
		current = st.Restriction.Base.Name
	}
	return current
}

// baseFamily returns the type family string for a given type name, walking
// the derivation chain if necessary.
func (p *Parser) baseFamily(name xsd.QName) string {
	// Check if the type itself (or any ancestor) is a list type.
	if p.isListType(name) {
		return "list"
	}
	ultimate := p.resolveUltimateBase(name)
	info := p.builtin.Lookup(ultimate)
	if info == nil {
		return ""
	}
	return info.Family
}

// isListType checks if the given type name refers to a list simple type
// (directly or via restriction chain).
func (p *Parser) isListType(name xsd.QName) bool {
	visited := make(map[xsd.QName]bool)
	current := name
	for {
		if visited[current] {
			return false
		}
		visited[current] = true

		// Check built-in list types.
		if info := p.builtin.Lookup(current); info != nil {
			return info.Family == "list"
		}

		t := p.symbols.LookupType(current)
		if t == nil {
			return false
		}
		st, ok := t.(*xsd.SimpleType)
		if !ok {
			return false
		}
		if st.List != nil {
			return true
		}
		if st.Restriction != nil {
			current = st.Restriction.Base.Name
			continue
		}
		return false
	}
}

// resolveBaseFacets returns the facets defined on the base type. For
// user-defined types, it collects facets from the immediate base.
func (p *Parser) resolveBaseFacets(baseName xsd.QName) []xsd.Facet {
	t := p.symbols.LookupType(baseName)
	if t == nil {
		return nil
	}
	st, ok := t.(*xsd.SimpleType)
	if !ok || st.Restriction == nil {
		return nil
	}
	return st.Restriction.Facets
}

// handleValidationErrors processes validation errors according to the
// configured strictness level. Returns errors for Error level, logs
// warnings for Warn level.
func (p *Parser) handleValidationErrors(rule config.ValidationRule, errs []error, context string) []error {
	level := p.opts.SchemaStrictness.Level(rule)
	switch level {
	case config.ValidationError:
		return errs
	case config.ValidationWarn:
		for _, err := range errs {
			p.logger.Warn("schema validation warning",
				slog.String("rule", fmt.Sprintf("%d", rule)),
				slog.String("context", context),
				slog.String("error", err.Error()))
		}
		return nil
	default: // ValidationOff
		return nil
	}
}
