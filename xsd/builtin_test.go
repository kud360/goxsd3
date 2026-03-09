package xsd_test

import (
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

// allBuiltinNames lists all 50 built-in XSD type local names.
// (2 ur-types + 1 XSD 1.1 intermediate + 19 primitives + 25 derived + 3 list types)
var allBuiltinNames = []string{
	"anyType", "anySimpleType", "anyAtomicType",
	"string", "normalizedString", "token", "language", "NMTOKEN",
	"Name", "NCName", "ID", "IDREF", "ENTITY",
	"boolean",
	"decimal", "integer",
	"nonPositiveInteger", "negativeInteger",
	"long", "int", "short", "byte",
	"nonNegativeInteger", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte",
	"positiveInteger",
	"float", "double",
	"duration", "yearMonthDuration", "dayTimeDuration",
	"dateTime", "dateTimeStamp", "time", "date",
	"gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth",
	"hexBinary", "base64Binary",
	"anyURI",
	"QName", "NOTATION",
	"NMTOKENS", "IDREFS", "ENTITIES",
}

func TestAllBuiltinTypesPresent(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	for _, name := range allBuiltinNames {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Errorf("built-in type %q not found in registry", name)
		}
	}
	if len(allBuiltinNames) != 50 {
		t.Errorf("expected 50 built-in types, have %d in list", len(allBuiltinNames))
	}
}

func TestBuiltinDerivationChain(t *testing.T) {
	r := xsd.NewBuiltinRegistry()

	chain := func(local string) []string {
		var result []string
		info := r.Lookup(xsd.XSDName(local))
		for info != nil && info.Base != nil {
			result = append(result, info.Base.Local)
			info = r.Lookup(*info.Base)
		}
		return result
	}

	tests := []struct {
		start string
		want  []string
	}{
		{"integer", []string{"decimal", "anyAtomicType", "anySimpleType", "anyType"}},
		{"token", []string{"normalizedString", "string", "anyAtomicType", "anySimpleType", "anyType"}},
		{"NCName", []string{"Name", "token", "normalizedString", "string", "anyAtomicType", "anySimpleType", "anyType"}},
		{"short", []string{"int", "long", "integer", "decimal", "anyAtomicType", "anySimpleType", "anyType"}},
		{"unsignedByte", []string{"unsignedShort", "unsignedInt", "unsignedLong", "nonNegativeInteger", "integer", "decimal", "anyAtomicType", "anySimpleType", "anyType"}},
	}

	for _, tt := range tests {
		t.Run(tt.start, func(t *testing.T) {
			got := chain(tt.start)
			if len(got) != len(tt.want) {
				t.Fatalf("chain(%s): got %v, want %v", tt.start, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("chain(%s)[%d] = %s, want %s", tt.start, i, got[i], tt.want[i])
				}
			}
		})
	}

	anyType := r.Lookup(xsd.XSDName("anyType"))
	if anyType.Base != nil {
		t.Error("anyType should have nil base")
	}
}

func TestStringFamilyFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	stringTypes := []string{"string", "normalizedString", "token", "language", "Name", "NCName"}

	for _, name := range stringTypes {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Fatalf("type %q not found", name)
		}
		if info.Family != "string" {
			t.Errorf("%s: family = %q, want %q", name, info.Family, "string")
		}
	}
	for _, fk := range []xsd.FacetKind{xsd.FacetLength, xsd.FacetMinLength, xsd.FacetMaxLength, xsd.FacetPattern, xsd.FacetEnumeration, xsd.FacetWhiteSpace} {
		if !xsd.IsFacetApplicable("string", fk) {
			t.Errorf("string family should support %s", fk)
		}
	}
	for _, fk := range []xsd.FacetKind{xsd.FacetMaxInclusive, xsd.FacetTotalDigits} {
		if xsd.IsFacetApplicable("string", fk) {
			t.Errorf("string family should not support %s", fk)
		}
	}
}

func TestNumericFamilyFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	numericTypes := []string{"decimal", "integer", "long", "int", "short", "byte"}

	for _, name := range numericTypes {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Fatalf("type %q not found", name)
		}
		if info.Family != "decimal" {
			t.Errorf("%s: family = %q, want %q", name, info.Family, "decimal")
		}
	}
	for _, fk := range []xsd.FacetKind{xsd.FacetPattern, xsd.FacetEnumeration, xsd.FacetMaxInclusive, xsd.FacetMaxExclusive, xsd.FacetMinInclusive, xsd.FacetMinExclusive, xsd.FacetTotalDigits, xsd.FacetFractionDigits} {
		if !xsd.IsFacetApplicable("decimal", fk) {
			t.Errorf("decimal family should support %s", fk)
		}
	}
	for _, fk := range []xsd.FacetKind{xsd.FacetLength, xsd.FacetMinLength, xsd.FacetMaxLength} {
		if xsd.IsFacetApplicable("decimal", fk) {
			t.Errorf("decimal family should not support %s", fk)
		}
	}
}

func TestDateTimeFamilyFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	dtTypes := map[string]string{
		"dateTime": "dateTime", "date": "date", "time": "time", "gYear": "gYear",
	}
	for name, family := range dtTypes {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Fatalf("type %q not found", name)
		}
		if info.Family != family {
			t.Errorf("%s: family = %q, want %q", name, info.Family, family)
		}
		if !xsd.IsFacetApplicable(family, xsd.FacetMinInclusive) {
			t.Errorf("%s family should support minInclusive", family)
		}
		if xsd.IsFacetApplicable(family, xsd.FacetLength) {
			t.Errorf("%s family should not support length", family)
		}
		if xsd.IsFacetApplicable(family, xsd.FacetTotalDigits) {
			t.Errorf("%s family should not support totalDigits", family)
		}
	}
}

func TestBinaryFamilyFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	for _, name := range []string{"hexBinary", "base64Binary"} {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Fatalf("type %q not found", name)
		}
		family := info.Family
		for _, fk := range []xsd.FacetKind{xsd.FacetLength, xsd.FacetMinLength, xsd.FacetMaxLength, xsd.FacetPattern, xsd.FacetEnumeration, xsd.FacetWhiteSpace} {
			if !xsd.IsFacetApplicable(family, fk) {
				t.Errorf("%s family should support %s", family, fk)
			}
		}
		if xsd.IsFacetApplicable(family, xsd.FacetMaxInclusive) {
			t.Errorf("%s family should not support maxInclusive", family)
		}
	}
}

func TestBooleanFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	info := r.Lookup(xsd.XSDName("boolean"))
	if info == nil {
		t.Fatal("boolean not found")
	}
	if info.Family != "boolean" {
		t.Errorf("boolean family = %q, want %q", info.Family, "boolean")
	}
	if !xsd.IsFacetApplicable("boolean", xsd.FacetPattern) {
		t.Error("boolean should support pattern")
	}
	if !xsd.IsFacetApplicable("boolean", xsd.FacetWhiteSpace) {
		t.Error("boolean should support whiteSpace")
	}
	if xsd.IsFacetApplicable("boolean", xsd.FacetEnumeration) {
		t.Error("boolean should NOT support enumeration")
	}
	for _, fk := range []xsd.FacetKind{xsd.FacetLength, xsd.FacetMinLength, xsd.FacetMaxInclusive} {
		if xsd.IsFacetApplicable("boolean", fk) {
			t.Errorf("boolean should not support %s", fk)
		}
	}
}

func TestGoTypeMappings(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	tests := []struct {
		xsdType, goType string
	}{
		{"string", "string"}, {"boolean", "bool"}, {"decimal", "float64"}, {"integer", "int64"},
		{"long", "int64"}, {"int", "int32"}, {"short", "int16"}, {"byte", "int8"},
		{"unsignedLong", "uint64"}, {"unsignedInt", "uint32"}, {"unsignedShort", "uint16"}, {"unsignedByte", "uint8"},
		{"float", "float32"}, {"double", "float64"},
		{"hexBinary", "[]byte"}, {"base64Binary", "[]byte"},
		{"NMTOKENS", "[]string"}, {"IDREFS", "[]string"}, {"ENTITIES", "[]string"},
		{"anyType", "any"}, {"duration", "string"}, {"dateTime", "string"}, {"time", "string"},
		{"date", "string"}, {"anyURI", "string"},
		{"positiveInteger", "uint64"}, {"nonNegativeInteger", "uint64"},
		{"nonPositiveInteger", "int64"}, {"negativeInteger", "int64"},
	}
	for _, tt := range tests {
		got := r.GoType(xsd.XSDName(tt.xsdType))
		if got != tt.goType {
			t.Errorf("GoType(%s) = %q, want %q", tt.xsdType, got, tt.goType)
		}
	}
}

func TestValidRestriction(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	if err := r.IsValidRestriction(xsd.XSDName("string"), []xsd.Facet{{Kind: xsd.FacetPattern, Value: `\d+`}}); err != nil {
		t.Errorf("pattern on string should be valid: %v", err)
	}
	if err := r.IsValidRestriction(xsd.XSDName("integer"), []xsd.Facet{{Kind: xsd.FacetEnumeration, Value: "1"}}); err != nil {
		t.Errorf("enumeration on integer should be valid: %v", err)
	}
	if err := r.IsValidRestriction(xsd.XSDName("decimal"), []xsd.Facet{{Kind: xsd.FacetMinInclusive, Value: "0"}}); err != nil {
		t.Errorf("minInclusive on decimal should be valid: %v", err)
	}
}

func TestInvalidRestriction(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	if err := r.IsValidRestriction(xsd.XSDName("string"), []xsd.Facet{{Kind: xsd.FacetTotalDigits, Value: "5"}}); err == nil {
		t.Error("totalDigits on string should be invalid")
	}
	if err := r.IsValidRestriction(xsd.XSDName("boolean"), []xsd.Facet{{Kind: xsd.FacetLength, Value: "1"}}); err == nil {
		t.Error("length on boolean should be invalid")
	}
	if err := r.IsValidRestriction(xsd.XSDName("dateTime"), []xsd.Facet{{Kind: xsd.FacetFractionDigits, Value: "2"}}); err == nil {
		t.Error("fractionDigits on dateTime should be invalid")
	}
}

func TestFacetInheritance(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	stringFacets := r.ApplicableFacets(xsd.XSDName("string"))
	tokenFacets := r.ApplicableFacets(xsd.XSDName("token"))

	if len(stringFacets) != len(tokenFacets) {
		t.Errorf("token should have same number of applicable facets as string: got %d, want %d", len(tokenFacets), len(stringFacets))
	}
	stringSet := make(map[xsd.FacetKind]bool)
	for _, fk := range stringFacets {
		stringSet[fk] = true
	}
	for _, fk := range tokenFacets {
		if !stringSet[fk] {
			t.Errorf("token has facet %s not in string", fk)
		}
	}
}

func TestFacetNarrowing(t *testing.T) {
	// Narrowing maxLength 10→5: OK
	errs := xsd.ValidateFacetNarrowing(
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "10"}},
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "5"}},
	)
	if len(errs) != 0 {
		t.Errorf("narrowing maxLength 10→5 should be OK: %v", errs)
	}

	// Widening maxLength 10→20: error
	errs = xsd.ValidateFacetNarrowing(
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "10"}},
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "20"}},
	)
	if len(errs) == 0 {
		t.Error("widening maxLength 10→20 should be an error")
	}

	// Widening minLength 5→3: error
	errs = xsd.ValidateFacetNarrowing(
		[]xsd.Facet{{Kind: xsd.FacetMinLength, Value: "5"}},
		[]xsd.Facet{{Kind: xsd.FacetMinLength, Value: "3"}},
	)
	if len(errs) == 0 {
		t.Error("widening minLength 5→3 should be an error")
	}

	// Narrowing minInclusive 0→5: OK
	errs = xsd.ValidateFacetNarrowing(
		[]xsd.Facet{{Kind: xsd.FacetMinInclusive, Value: "0"}},
		[]xsd.Facet{{Kind: xsd.FacetMinInclusive, Value: "5"}},
	)
	if len(errs) != 0 {
		t.Errorf("narrowing minInclusive 0→5 should be OK: %v", errs)
	}

	// Fixed base facet changed: error
	errs = xsd.ValidateFacetNarrowing(
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "10", Fixed: true}},
		[]xsd.Facet{{Kind: xsd.FacetMaxLength, Value: "5"}},
	)
	if len(errs) == 0 {
		t.Error("changing a fixed facet should be an error")
	}
}

func TestXSD11BuiltinTypes(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	tests := []struct{ name, wantBase string }{
		{"yearMonthDuration", "duration"},
		{"dayTimeDuration", "duration"},
		{"dateTimeStamp", "dateTime"},
		{"anyAtomicType", "anySimpleType"},
	}
	for _, tt := range tests {
		info := r.Lookup(xsd.XSDName(tt.name))
		if info == nil {
			t.Errorf("XSD 1.1 type %q not found", tt.name)
			continue
		}
		if info.Base == nil || info.Base.Local != tt.wantBase {
			got := "<nil>"
			if info.Base != nil {
				got = info.Base.Local
			}
			t.Errorf("%s: base = %s, want %s", tt.name, got, tt.wantBase)
		}
	}
}

func TestTypeProperties(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	tests := []struct {
		name        string
		ordered     xsd.Ordered
		bounded     bool
		cardinality xsd.Cardinality
		numeric     bool
	}{
		{"string", xsd.OrderedFalse, false, xsd.CardinalityCountablyInfinite, false},
		{"boolean", xsd.OrderedFalse, false, xsd.CardinalityFinite, false},
		{"decimal", xsd.OrderedTotal, false, xsd.CardinalityCountablyInfinite, true},
		{"float", xsd.OrderedPartial, true, xsd.CardinalityFinite, true},
		{"integer", xsd.OrderedTotal, false, xsd.CardinalityCountablyInfinite, true},
		{"long", xsd.OrderedTotal, true, xsd.CardinalityCountablyInfinite, true},
		{"duration", xsd.OrderedPartial, false, xsd.CardinalityCountablyInfinite, false},
		{"dateTime", xsd.OrderedPartial, false, xsd.CardinalityCountablyInfinite, false},
	}
	for _, tt := range tests {
		info := r.Lookup(xsd.XSDName(tt.name))
		if info == nil {
			t.Fatalf("type %q not found", tt.name)
		}
		if info.Properties.Ordered != tt.ordered {
			t.Errorf("%s: ordered = %q, want %q", tt.name, info.Properties.Ordered, tt.ordered)
		}
		if info.Properties.Bounded != tt.bounded {
			t.Errorf("%s: bounded = %v, want %v", tt.name, info.Properties.Bounded, tt.bounded)
		}
		if info.Properties.Cardinality != tt.cardinality {
			t.Errorf("%s: cardinality = %q, want %q", tt.name, info.Properties.Cardinality, tt.cardinality)
		}
		if info.Properties.Numeric != tt.numeric {
			t.Errorf("%s: numeric = %v, want %v", tt.name, info.Properties.Numeric, tt.numeric)
		}
	}
}

func TestFacetCrossValidation(t *testing.T) {
	// minLength > maxLength → error
	errs := xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetMinLength, Value: "10"},
		{Kind: xsd.FacetMaxLength, Value: "5"},
	}, "string")
	if len(errs) == 0 {
		t.Error("minLength > maxLength should produce an error")
	}

	// minInclusive > maxInclusive → error
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetMinInclusive, Value: "100"},
		{Kind: xsd.FacetMaxInclusive, Value: "50"},
	}, "decimal")
	if len(errs) == 0 {
		t.Error("minInclusive > maxInclusive should produce an error")
	}

	// totalDigits < fractionDigits → error
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetTotalDigits, Value: "3"},
		{Kind: xsd.FacetFractionDigits, Value: "5"},
	}, "decimal")
	if len(errs) == 0 {
		t.Error("totalDigits < fractionDigits should produce an error")
	}

	// Valid: minLength=5, maxLength=10
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetMinLength, Value: "5"},
		{Kind: xsd.FacetMaxLength, Value: "10"},
	}, "string")
	if len(errs) != 0 {
		t.Errorf("minLength=5, maxLength=10 should be valid: %v", errs)
	}

	// Valid: length=5, minLength=3, maxLength=10
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetLength, Value: "5"},
		{Kind: xsd.FacetMinLength, Value: "3"},
		{Kind: xsd.FacetMaxLength, Value: "10"},
	}, "string")
	if len(errs) != 0 {
		t.Errorf("length=5 with minLength=3 maxLength=10 should be valid: %v", errs)
	}

	// length < minLength → error
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetLength, Value: "5"},
		{Kind: xsd.FacetMinLength, Value: "8"},
	}, "string")
	if len(errs) == 0 {
		t.Error("length=5 with minLength=8 should produce an error")
	}

	// Inapplicable facet → error
	errs = xsd.ValidateFacetSet([]xsd.Facet{
		{Kind: xsd.FacetTotalDigits, Value: "5"},
	}, "string")
	if len(errs) == 0 {
		t.Error("totalDigits on string family should produce an error")
	}
}

func TestValidateDefaultValue(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	tests := []struct {
		value, xsdType string
		wantErr        bool
	}{
		{"true", "boolean", false},
		{"false", "boolean", false},
		{"1", "boolean", false},
		{"0", "boolean", false},
		{"yes", "boolean", true},
		{"abc", "integer", true},
		{"42", "integer", false},
		{"3.14", "decimal", false},
		{"abc", "decimal", true},
		{"INF", "float", false},
		{"-INF", "double", false},
		{"NaN", "float", false},
		{"3.14", "float", false},
		{"0A1B", "hexBinary", false},
		{"0A1", "hexBinary", true},
		{"hello", "string", false},
		{"200", "byte", true},
		{"-1", "unsignedInt", true},
		{"0", "positiveInteger", true},
		{"0", "nonNegativeInteger", false},
		{"-1", "negativeInteger", false},
		{"1", "negativeInteger", true},
		{"-1", "nonPositiveInteger", false},
		{"1", "nonPositiveInteger", true},
		{"2024-01-15", "date", false},
		{"", "date", true},
		{"http://example.com", "anyURI", false},
		{"has space", "anyURI", true},
		{"foo:bar", "QName", false},
		{"local", "QName", false},
		{"", "QName", true},
	}
	for _, tt := range tests {
		err := xsd.ValidateDefaultValue(tt.value, xsd.XSDName(tt.xsdType), r)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateDefaultValue(%q, %s): err=%v, wantErr=%v", tt.value, tt.xsdType, err, tt.wantErr)
		}
	}
}

func TestWhiteSpaceRules(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	tests := []struct {
		name string
		want xsd.WhiteSpaceRule
	}{
		{"string", xsd.WhiteSpacePreserve},
		{"normalizedString", xsd.WhiteSpaceReplace},
		{"token", xsd.WhiteSpaceCollapse},
		{"boolean", xsd.WhiteSpaceCollapse},
		{"decimal", xsd.WhiteSpaceCollapse},
		{"integer", xsd.WhiteSpaceCollapse},
		{"float", xsd.WhiteSpaceCollapse},
		{"double", xsd.WhiteSpaceCollapse},
		{"dateTime", xsd.WhiteSpaceCollapse},
		{"hexBinary", xsd.WhiteSpaceCollapse},
		{"anyURI", xsd.WhiteSpaceCollapse},
	}
	for _, tt := range tests {
		info := r.Lookup(xsd.XSDName(tt.name))
		if info == nil {
			t.Fatalf("type %q not found", tt.name)
		}
		if info.WhiteSpace != tt.want {
			t.Errorf("%s: whiteSpace = %q, want %q", tt.name, info.WhiteSpace, tt.want)
		}
	}
}

func TestQName(t *testing.T) {
	q := xsd.NewQName("http://example.com", "foo")
	if q.String() != "{http://example.com}foo" {
		t.Errorf("QName.String() = %q, want %q", q.String(), "{http://example.com}foo")
	}
	q2 := xsd.NewQName("", "bar")
	if q2.String() != "bar" {
		t.Errorf("QName.String() with empty ns = %q, want %q", q2.String(), "bar")
	}
	q3 := xsd.XSDName("string")
	if q3.Namespace != xsd.XSDNS {
		t.Errorf("XSDName namespace = %q, want %q", q3.Namespace, xsd.XSDNS)
	}
	if q3.Local != "string" {
		t.Errorf("XSDName local = %q, want %q", q3.Local, "string")
	}
}

func TestListTypeFacets(t *testing.T) {
	r := xsd.NewBuiltinRegistry()
	for _, name := range []string{"NMTOKENS", "IDREFS", "ENTITIES"} {
		info := r.Lookup(xsd.XSDName(name))
		if info == nil {
			t.Fatalf("type %q not found", name)
		}
		if info.Family != "list" {
			t.Errorf("%s: family = %q, want %q", name, info.Family, "list")
		}
		for _, fk := range []xsd.FacetKind{xsd.FacetLength, xsd.FacetMinLength, xsd.FacetMaxLength, xsd.FacetPattern, xsd.FacetEnumeration, xsd.FacetWhiteSpace} {
			if !xsd.IsFacetApplicable("list", fk) {
				t.Errorf("list family should support %s", fk)
			}
		}
		if xsd.IsFacetApplicable("list", xsd.FacetMaxInclusive) {
			t.Error("list family should not support maxInclusive")
		}
	}
}
