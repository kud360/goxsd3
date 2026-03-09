package xsd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ValidateDefaultValue validates that a default or fixed value is valid for
// the given built-in type. If the type is not a built-in, it returns nil
// since user-defined types cannot be validated at this level.
func ValidateDefaultValue(value string, typeName QName, registry *BuiltinRegistry) error {
	if typeName.Namespace != XSDNS {
		return nil // not a built-in type; nothing to validate here
	}

	info := registry.Lookup(typeName)
	if info == nil {
		return nil // unknown type; skip validation
	}

	local := typeName.Local

	// String family and list types — any value is valid.
	if info.Family == "string" || info.Family == "list" || local == "NOTATION" {
		return nil
	}

	switch {
	// Boolean
	case local == "boolean":
		if !isValidBoolean(value) {
			return fmt.Errorf("invalid boolean value %q: must be true, false, 1, or 0", value)
		}
		return nil

	// Integer and derived integer types (no decimal point allowed).
	case info.Family == "decimal" && local != "decimal":
		if !isValidInteger(value) {
			return fmt.Errorf("invalid integer value %q for type %s", value, local)
		}
		return validateIntegerRange(value, local)

	// Decimal family.
	case local == "decimal":
		if !isValidDecimal(value) {
			return fmt.Errorf("invalid decimal value %q", value)
		}
		return nil

	// Float / double.
	case info.Family == "float" || info.Family == "double":
		if !isValidFloat(value) {
			return fmt.Errorf("invalid %s value %q", local, value)
		}
		return nil

	// dateTime family — basic format check.
	case info.Family == "duration" ||
		info.Family == "dateTime" || info.Family == "time" || info.Family == "date" ||
		info.Family == "gYear" || info.Family == "gYearMonth" || info.Family == "gMonth" ||
		info.Family == "gMonthDay" || info.Family == "gDay":
		return validateDateTimeLike(value, local)

	// hexBinary
	case local == "hexBinary":
		if !isValidHexBinary(value) {
			return fmt.Errorf("invalid hexBinary value %q: must be an even number of hexadecimal characters", value)
		}
		return nil

	// base64Binary
	case local == "base64Binary":
		if !isValidBase64(value) {
			return fmt.Errorf("invalid base64Binary value %q", value)
		}
		return nil

	// anyURI
	case local == "anyURI":
		if strings.ContainsRune(value, ' ') {
			return fmt.Errorf("invalid anyURI value %q: must not contain spaces", value)
		}
		return nil

	// QName
	case local == "QName":
		if !isValidQNameValue(value) {
			return fmt.Errorf("invalid QName value %q: must be NCName or NCName:NCName", value)
		}
		return nil
	}

	return nil
}

// validateDateTimeLike performs basic sanity checking on date/time/duration values.
func validateDateTimeLike(value, local string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("invalid %s value: empty string", local)
	}
	for _, r := range value {
		if unicode.IsDigit(r) {
			return nil
		}
	}
	return fmt.Errorf("invalid %s value %q: expected ISO 8601 format", local, value)
}

// integerRange defines the valid range for an integer subtype.
type integerRange struct {
	min      int64
	max      int64
	unsigned bool
	umax     uint64
}

// integerRanges maps XSD integer subtypes to their valid ranges.
var integerRanges = map[string]integerRange{
	"long":            {min: math.MinInt64, max: math.MaxInt64},
	"int":             {min: math.MinInt32, max: math.MaxInt32},
	"short":           {min: math.MinInt16, max: math.MaxInt16},
	"byte":            {min: math.MinInt8, max: math.MaxInt8},
	"unsignedLong":    {unsigned: true, umax: math.MaxUint64},
	"unsignedInt":     {unsigned: true, umax: math.MaxUint32},
	"unsignedShort":   {unsigned: true, umax: math.MaxUint16},
	"unsignedByte":    {unsigned: true, umax: math.MaxUint8},
	"positiveInteger": {min: 1, max: math.MaxInt64},
}

// validateIntegerRange checks that value fits within the range of the named
// integer subtype. The caller has already verified that value is a valid integer.
func validateIntegerRange(value, local string) error {
	r, ok := integerRanges[local]
	if !ok {
		// Handle special cases not expressible as simple ranges.
		switch local {
		case "nonNegativeInteger":
			if strings.HasPrefix(value, "-") && value != "-0" {
				return fmt.Errorf("value %q out of range for nonNegativeInteger (must be >= 0)", value)
			}
		case "negativeInteger":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("value %q is not a valid negativeInteger", value)
			}
			if v >= 0 {
				return fmt.Errorf("value %q out of range for negativeInteger (must be < 0)", value)
			}
		case "nonPositiveInteger":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("value %q is not a valid nonPositiveInteger", value)
			}
			if v > 0 {
				return fmt.Errorf("value %q out of range for nonPositiveInteger (must be <= 0)", value)
			}
		}
		return nil
	}

	if r.unsigned {
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("value %q out of range for %s", value, local)
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil || v > r.umax {
			return fmt.Errorf("value %q out of range for %s", value, local)
		}
		return nil
	}

	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil || v < r.min || v > r.max {
		return fmt.Errorf("value %q out of range for %s", value, local)
	}
	return nil
}

// ValidateFacetNarrowing checks that derived facets only narrow (never widen)
// the base facets. It returns a slice of errors, one per violation found.
func ValidateFacetNarrowing(baseFacets, derivedFacets []Facet) []error {
	var errs []error

	baseByKind := make(map[FacetKind]Facet)
	for _, f := range baseFacets {
		baseByKind[f.Kind] = f
	}

	for _, df := range derivedFacets {
		bf, ok := baseByKind[df.Kind]
		if !ok {
			continue // no base facet of this kind to compare against
		}

		// Fixed base facets cannot be changed.
		if bf.Fixed && bf.Value != df.Value {
			errs = append(errs, fmt.Errorf("facet %v is fixed in base type as %q and cannot be changed to %q",
				df.Kind, bf.Value, df.Value))
			continue
		}

		if err := checkNarrowing(df.Kind, bf.Value, df.Value); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// checkNarrowing verifies that the derived facet value properly narrows the
// base facet value according to the facet kind's rules.
func checkNarrowing(kind FacetKind, baseVal, derivedVal string) error {
	switch kind {
	case FacetMinLength:
		return compareDerived(kind, baseVal, derivedVal, parseInt64, ">=")
	case FacetMaxLength, FacetTotalDigits, FacetFractionDigits:
		return compareDerived(kind, baseVal, derivedVal, parseInt64, "<=")
	case FacetMinInclusive, FacetMinExclusive:
		return compareDerived(kind, baseVal, derivedVal, parseFloat64, ">=")
	case FacetMaxInclusive, FacetMaxExclusive:
		return compareDerived(kind, baseVal, derivedVal, parseFloat64, "<=")
	}
	return nil
}

// compareDerived is a generic narrowing check. It parses both values using the
// given parse function and verifies the derived value satisfies the given
// relation (">=" or "<=") relative to the base value.
func compareDerived[T int64 | float64](kind FacetKind, baseVal, derivedVal string, parse func(string) (T, error), op string) error {
	bv, err := parse(baseVal)
	if err != nil {
		return nil // cannot compare; skip
	}
	dv, err := parse(derivedVal)
	if err != nil {
		return nil
	}
	switch op {
	case ">=":
		if dv < bv {
			return fmt.Errorf("facet %v value %s in derived type is less than base value %s (must be >=)", kind, derivedVal, baseVal)
		}
	case "<=":
		if dv > bv {
			return fmt.Errorf("facet %v value %s in derived type is greater than base value %s (must be <=)", kind, derivedVal, baseVal)
		}
	}
	return nil
}

// parseInt64 parses s as a base-10 int64.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// parseFloat64 parses s as a float64.
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ---------------------------------------------------------------------------
// Helper validation functions
// ---------------------------------------------------------------------------

// isValidBoolean reports whether s is a valid XSD boolean literal.
func isValidBoolean(s string) bool {
	return s == "true" || s == "false" || s == "1" || s == "0"
}

// isValidDecimal reports whether s is a valid XSD decimal literal:
// optional sign, digits, optional fractional part.
func isValidDecimal(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	// Must have at least one digit before or after the decimal point.
	hasDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		hasDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			hasDigit = true
			i++
		}
	}
	return i == len(s) && hasDigit
}

// isValidInteger reports whether s is a valid XSD integer literal:
// optional sign followed by one or more digits, no decimal point.
func isValidInteger(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	for i < len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		i++
	}
	return true
}

// isValidFloat reports whether s is a valid XSD float/double literal.
// Accepts standard floating-point notation as well as the special values
// "INF", "-INF", "+INF", and "NaN".
func isValidFloat(s string) bool {
	if s == "INF" || s == "-INF" || s == "+INF" || s == "NaN" {
		return true
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// isValidHexBinary reports whether s is a valid hexBinary value:
// an even number of hexadecimal characters.
func isValidHexBinary(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// isValidBase64 reports whether s contains only valid base64 characters
// (A-Z, a-z, 0-9, +, /, =, and whitespace).
func isValidBase64(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' ||
			r == ' ' || r == '\t' || r == '\n' || r == '\r') {
			return false
		}
	}
	return true
}

// isValidNCName reports whether s is a valid XML NCName (non-colonized name).
// An NCName starts with a letter or underscore, followed by letters, digits,
// hyphens, underscores, or periods.
func isValidNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_' {
				return false
			}
		}
	}
	return true
}

// isValidQNameValue reports whether s is a valid QName value: either a bare
// NCName ("local") or a prefixed form ("prefix:local"), where both parts are
// valid NCNames.
func isValidQNameValue(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 1 {
		return isValidNCName(parts[0])
	}
	return isValidNCName(parts[0]) && isValidNCName(parts[1])
}
