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

	switch {
	// String family — always valid.
	case info.Family == "string" ||
		local == "string" || local == "normalizedString" || local == "token" ||
		local == "language" || local == "Name" || local == "NCName" ||
		local == "NMTOKEN" || local == "NMTOKENS" ||
		local == "ID" || local == "IDREF" || local == "IDREFS" ||
		local == "ENTITY" || local == "ENTITIES" ||
		local == "NOTATION":
		return nil

	// Boolean
	case local == "boolean":
		if !isValidBoolean(value) {
			return fmt.Errorf("invalid boolean value %q: must be true, false, 1, or 0", value)
		}
		return nil

	// Integer and derived integer types (no decimal point allowed).
	case local == "integer" || local == "nonPositiveInteger" || local == "negativeInteger" ||
		local == "nonNegativeInteger" || local == "positiveInteger" ||
		local == "long" || local == "int" || local == "short" || local == "byte" ||
		local == "unsignedLong" || local == "unsignedInt" || local == "unsignedShort" || local == "unsignedByte":
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
	case local == "float" || local == "double":
		if !isValidFloat(value) {
			return fmt.Errorf("invalid %s value %q", local, value)
		}
		return nil

	// dateTime family — basic format check.
	case local == "dateTime" || local == "date" || local == "time" ||
		local == "gYear" || local == "gYearMonth" || local == "gMonth" ||
		local == "gMonthDay" || local == "gDay" || local == "duration":
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("invalid %s value: empty string", local)
		}
		// Basic sanity: must contain at least a digit.
		hasDigit := false
		for _, r := range value {
			if unicode.IsDigit(r) {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return fmt.Errorf("invalid %s value %q: expected ISO 8601 format", local, value)
		}
		return nil

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

// validateIntegerRange checks that value fits within the range of the named
// integer subtype. The caller has already verified that value is a valid integer.
func validateIntegerRange(value, local string) error {
	switch local {
	case "long":
		_, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value %q out of range for long", value)
		}
	case "int":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("value %q out of range for int (-2147483648 to 2147483647)", value)
		}
	case "short":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < math.MinInt16 || v > math.MaxInt16 {
			return fmt.Errorf("value %q out of range for short (-32768 to 32767)", value)
		}
	case "byte":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < math.MinInt8 || v > math.MaxInt8 {
			return fmt.Errorf("value %q out of range for byte (-128 to 127)", value)
		}
	case "unsignedLong":
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("value %q out of range for unsignedLong", value)
		}
		_, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value %q out of range for unsignedLong", value)
		}
	case "unsignedInt":
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("value %q out of range for unsignedInt", value)
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil || v > math.MaxUint32 {
			return fmt.Errorf("value %q out of range for unsignedInt (0 to 4294967295)", value)
		}
	case "unsignedShort":
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("value %q out of range for unsignedShort", value)
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil || v > math.MaxUint16 {
			return fmt.Errorf("value %q out of range for unsignedShort (0 to 65535)", value)
		}
	case "unsignedByte":
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("value %q out of range for unsignedByte", value)
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil || v > math.MaxUint8 {
			return fmt.Errorf("value %q out of range for unsignedByte (0 to 255)", value)
		}
	case "positiveInteger":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value %q is not a valid positiveInteger", value)
		}
		if v < 1 {
			return fmt.Errorf("value %q out of range for positiveInteger (must be >= 1)", value)
		}
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
		return requireDerivedGEBase(kind, baseVal, derivedVal)
	case FacetMaxLength:
		return requireDerivedLEBase(kind, baseVal, derivedVal)
	case FacetMinInclusive:
		return requireDerivedGEBaseFloat(kind, baseVal, derivedVal)
	case FacetMaxInclusive:
		return requireDerivedLEBaseFloat(kind, baseVal, derivedVal)
	case FacetMinExclusive:
		return requireDerivedGEBaseFloat(kind, baseVal, derivedVal)
	case FacetMaxExclusive:
		return requireDerivedLEBaseFloat(kind, baseVal, derivedVal)
	case FacetTotalDigits:
		return requireDerivedLEBase(kind, baseVal, derivedVal)
	case FacetFractionDigits:
		return requireDerivedLEBase(kind, baseVal, derivedVal)
	}
	return nil
}

// requireDerivedGEBase checks derived >= base using integer comparison.
func requireDerivedGEBase(kind FacetKind, baseVal, derivedVal string) error {
	bv, err := strconv.ParseInt(baseVal, 10, 64)
	if err != nil {
		return nil // cannot compare; skip
	}
	dv, err := strconv.ParseInt(derivedVal, 10, 64)
	if err != nil {
		return nil
	}
	if dv < bv {
		return fmt.Errorf("facet %v value %s in derived type is less than base value %s (must be >=)", kind, derivedVal, baseVal)
	}
	return nil
}

// requireDerivedLEBase checks derived <= base using integer comparison.
func requireDerivedLEBase(kind FacetKind, baseVal, derivedVal string) error {
	bv, err := strconv.ParseInt(baseVal, 10, 64)
	if err != nil {
		return nil
	}
	dv, err := strconv.ParseInt(derivedVal, 10, 64)
	if err != nil {
		return nil
	}
	if dv > bv {
		return fmt.Errorf("facet %v value %s in derived type is greater than base value %s (must be <=)", kind, derivedVal, baseVal)
	}
	return nil
}

// requireDerivedGEBaseFloat checks derived >= base using float comparison.
func requireDerivedGEBaseFloat(kind FacetKind, baseVal, derivedVal string) error {
	bv, err := strconv.ParseFloat(baseVal, 64)
	if err != nil {
		return nil
	}
	dv, err := strconv.ParseFloat(derivedVal, 64)
	if err != nil {
		return nil
	}
	if dv < bv {
		return fmt.Errorf("facet %v value %s in derived type is less than base value %s (must be >=)", kind, derivedVal, baseVal)
	}
	return nil
}

// requireDerivedLEBaseFloat checks derived <= base using float comparison.
func requireDerivedLEBaseFloat(kind FacetKind, baseVal, derivedVal string) error {
	bv, err := strconv.ParseFloat(baseVal, 64)
	if err != nil {
		return nil
	}
	dv, err := strconv.ParseFloat(derivedVal, 64)
	if err != nil {
		return nil
	}
	if dv > bv {
		return fmt.Errorf("facet %v value %s in derived type is greater than base value %s (must be <=)", kind, derivedVal, baseVal)
	}
	return nil
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

