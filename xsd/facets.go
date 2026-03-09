package xsd

import (
	"fmt"
	"strconv"
)

// FacetKind identifies a constraining facet defined by the XSD specification.
type FacetKind string

// XSD 1.0 constraining facets (14 total). Each corresponds to an
// xs:<facetName> element that may appear in a restriction derivation.
const (
	FacetLength         FacetKind = "length"         // exact required length
	FacetMinLength      FacetKind = "minLength"      // minimum required length
	FacetMaxLength      FacetKind = "maxLength"      // maximum allowed length
	FacetPattern        FacetKind = "pattern"         // regex pattern constraint
	FacetEnumeration    FacetKind = "enumeration"     // set of allowed values
	FacetWhiteSpace     FacetKind = "whiteSpace"      // whitespace normalization rule
	FacetMaxInclusive   FacetKind = "maxInclusive"    // upper bound (inclusive)
	FacetMaxExclusive   FacetKind = "maxExclusive"    // upper bound (exclusive)
	FacetMinInclusive   FacetKind = "minInclusive"    // lower bound (inclusive)
	FacetMinExclusive   FacetKind = "minExclusive"    // lower bound (exclusive)
	FacetTotalDigits    FacetKind = "totalDigits"     // maximum number of digits
	FacetFractionDigits FacetKind = "fractionDigits"  // maximum fractional digits
)

// XSD 1.1 constraining facets (3 additional).
const (
	FacetExplicitTimezone FacetKind = "explicitTimezone" // timezone requirement for date/time types
	FacetMinScale         FacetKind = "minScale"         // minimum scale for decimal
	FacetMaxScale         FacetKind = "maxScale"         // maximum scale for decimal
)

// Facet represents a single constraining facet applied to a simple type.
type Facet struct {
	Kind  FacetKind
	Value string
	Fixed bool
}

// Shared facet lists used by multiple type families.
var (
	// lengthFacets are applicable to string-like and binary types.
	lengthFacets = []FacetKind{
		FacetLength, FacetMinLength, FacetMaxLength,
		FacetPattern, FacetEnumeration, FacetWhiteSpace,
	}
	// orderedFacets are applicable to ordered types (date/time, float, duration).
	orderedFacets = []FacetKind{
		FacetPattern, FacetEnumeration, FacetWhiteSpace,
		FacetMaxInclusive, FacetMaxExclusive,
		FacetMinInclusive, FacetMinExclusive,
	}
)

// facetApplicability maps each type family name to the set of facets that may
// be applied to types belonging to that family.
var facetApplicability = map[string][]FacetKind{
	"string":       lengthFacets,
	"hexBinary":    lengthFacets,
	"base64Binary": lengthFacets,
	"anyURI":       lengthFacets,
	"QName":        lengthFacets,
	"NOTATION":     lengthFacets,
	"list":         lengthFacets,

	"boolean": {FacetPattern, FacetWhiteSpace},

	"decimal": {
		FacetPattern, FacetEnumeration, FacetWhiteSpace,
		FacetMaxInclusive, FacetMaxExclusive,
		FacetMinInclusive, FacetMinExclusive,
		FacetTotalDigits, FacetFractionDigits,
	},

	"float":      orderedFacets,
	"double":     orderedFacets,
	"duration":   orderedFacets,
	"dateTime":   orderedFacets,
	"time":       orderedFacets,
	"date":       orderedFacets,
	"gYearMonth": orderedFacets,
	"gYear":      orderedFacets,
	"gMonthDay":  orderedFacets,
	"gDay":       orderedFacets,
	"gMonth":     orderedFacets,
}

// FacetApplicability returns the facets that are applicable to the given type
// family. It returns nil if the family is not recognised.
func FacetApplicability(family string) []FacetKind {
	kinds, ok := facetApplicability[family]
	if !ok {
		return nil
	}
	out := make([]FacetKind, len(kinds))
	copy(out, kinds)
	return out
}

// IsFacetApplicable reports whether the given facet kind is applicable to the
// specified type family.
func IsFacetApplicable(family string, facet FacetKind) bool {
	kinds, ok := facetApplicability[family]
	if !ok {
		return false
	}
	for _, k := range kinds {
		if k == facet {
			return true
		}
	}
	return false
}

// ValidateFacetSet cross-validates a set of facets against each other and
// against the base type family. It returns a slice of errors describing every
// violation found; an empty slice means the facet set is valid.
func ValidateFacetSet(facets []Facet, baseFamily string) []error {
	var errs []error

	// Index facet values by kind for easy lookup.
	byKind := make(map[FacetKind]string, len(facets))
	for _, f := range facets {
		byKind[f.Kind] = f.Value
	}

	// Check that every facet is applicable to the base family.
	for _, f := range facets {
		if !IsFacetApplicable(baseFamily, f.Kind) {
			errs = append(errs, fmt.Errorf("facet %s is not applicable to type family %q", f.Kind, baseFamily))
		}
	}

	// Helper to parse an int64 from the facet map; returns (value, true) on
	// success and appends a parse error otherwise.
	parseInt := func(kind FacetKind) (int64, bool) {
		raw, ok := byKind[kind]
		if !ok {
			return 0, false
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("facet %s: invalid integer value %q: %v", kind, raw, err))
			return 0, false
		}
		return v, true
	}

	// Helper to parse a float64 from the facet map.
	parseFloat := func(kind FacetKind) (float64, bool) {
		raw, ok := byKind[kind]
		if !ok {
			return 0, false
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("facet %s: invalid numeric value %q: %v", kind, raw, err))
			return 0, false
		}
		return v, true
	}

	// minLength <= maxLength
	if minLen, ok1 := parseInt(FacetMinLength); ok1 {
		if maxLen, ok2 := parseInt(FacetMaxLength); ok2 {
			if minLen > maxLen {
				errs = append(errs, fmt.Errorf("minLength (%d) must be <= maxLength (%d)", minLen, maxLen))
			}
		}
	}

	// If length is set, it must satisfy: length >= minLength AND length <= maxLength
	if length, ok1 := parseInt(FacetLength); ok1 {
		if minLen, ok2 := parseInt(FacetMinLength); ok2 {
			if length < minLen {
				errs = append(errs, fmt.Errorf("length (%d) must be >= minLength (%d)", length, minLen))
			}
		}
		if maxLen, ok2 := parseInt(FacetMaxLength); ok2 {
			if length > maxLen {
				errs = append(errs, fmt.Errorf("length (%d) must be <= maxLength (%d)", length, maxLen))
			}
		}
	}

	// minInclusive <= maxInclusive
	if minInc, ok1 := parseFloat(FacetMinInclusive); ok1 {
		if maxInc, ok2 := parseFloat(FacetMaxInclusive); ok2 {
			if minInc > maxInc {
				errs = append(errs, fmt.Errorf("minInclusive (%v) must be <= maxInclusive (%v)", minInc, maxInc))
			}
		}
	}

	// minExclusive < maxExclusive
	if minExc, ok1 := parseFloat(FacetMinExclusive); ok1 {
		if maxExc, ok2 := parseFloat(FacetMaxExclusive); ok2 {
			if minExc >= maxExc {
				errs = append(errs, fmt.Errorf("minExclusive (%v) must be < maxExclusive (%v)", minExc, maxExc))
			}
		}
	}

	// minInclusive < maxExclusive (cannot be equal)
	if minInc, ok1 := parseFloat(FacetMinInclusive); ok1 {
		if maxExc, ok2 := parseFloat(FacetMaxExclusive); ok2 {
			if minInc >= maxExc {
				errs = append(errs, fmt.Errorf("minInclusive (%v) must be < maxExclusive (%v)", minInc, maxExc))
			}
		}
	}

	// minExclusive < maxInclusive (cannot be equal)
	if minExc, ok1 := parseFloat(FacetMinExclusive); ok1 {
		if maxInc, ok2 := parseFloat(FacetMaxInclusive); ok2 {
			if minExc >= maxInc {
				errs = append(errs, fmt.Errorf("minExclusive (%v) must be < maxInclusive (%v)", minExc, maxInc))
			}
		}
	}

	// totalDigits >= fractionDigits
	if total, ok1 := parseInt(FacetTotalDigits); ok1 {
		if frac, ok2 := parseInt(FacetFractionDigits); ok2 {
			if total < frac {
				errs = append(errs, fmt.Errorf("totalDigits (%d) must be >= fractionDigits (%d)", total, frac))
			}
		}
	}

	return errs
}
