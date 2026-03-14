package parser

import (
	"strings"
	"testing"

	"github.com/kud360/goxsd3/config"
	"github.com/kud360/goxsd3/xsd"
)

// ---------------------------------------------------------------------------
// Valid restrictions — should parse without error
// ---------------------------------------------------------------------------

func TestValidate_RestrictString(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/builtin/restrict_string.xsd")
	if err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
	st := ss.Schemas[0].Types[0].(*xsd.SimpleType)
	if st.Name.Local != "ShortName" {
		t.Errorf("expected ShortName, got %s", st.Name.Local)
	}
	if len(st.Restriction.Facets) != 3 {
		t.Errorf("expected 3 facets, got %d", len(st.Restriction.Facets))
	}
}

func TestValidate_RestrictInteger(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/builtin/restrict_integer.xsd")
	if err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
	st := ss.Schemas[0].Types[0].(*xsd.SimpleType)
	if st.Name.Local != "Percentage" {
		t.Errorf("expected Percentage, got %s", st.Name.Local)
	}
}

func TestValidate_RestrictDecimal(t *testing.T) {
	p := newTestParser()
	ss, err := p.Parse("../testdata/builtin/restrict_decimal.xsd")
	if err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
	st := ss.Schemas[0].Types[0].(*xsd.SimpleType)
	if st.Name.Local != "Price" {
		t.Errorf("expected Price, got %s", st.Name.Local)
	}
}

// ---------------------------------------------------------------------------
// Invalid facet applicability — totalDigits on xs:string
// ---------------------------------------------------------------------------

func TestValidate_InvalidFacet(t *testing.T) {
	p := newTestParser()
	_, err := p.Parse("../testdata/builtin/invalid_facet.xsd")
	if err == nil {
		t.Fatal("expected error for invalid facet (totalDigits on string)")
	}
	if !strings.Contains(err.Error(), "totalDigits") {
		t.Errorf("error should mention totalDigits, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Invalid facet combination — minLength > maxLength
// ---------------------------------------------------------------------------

func TestValidate_InvalidFacetCombo(t *testing.T) {
	p := newTestParser()
	_, err := p.Parse("../testdata/builtin/invalid_facet_combo.xsd")
	if err == nil {
		t.Fatal("expected error for minLength > maxLength")
	}
	if !strings.Contains(err.Error(), "minLength") {
		t.Errorf("error should mention minLength, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Invalid default value — "abc" for xs:integer
// ---------------------------------------------------------------------------

func TestValidate_InvalidDefaults(t *testing.T) {
	p := newTestParser()
	_, err := p.Parse("../testdata/builtin/invalid_defaults.xsd")
	if err == nil {
		t.Fatal("expected error for invalid default value")
	}
	if !strings.Contains(err.Error(), "abc") || !strings.Contains(err.Error(), "badInt") {
		t.Errorf("error should mention 'abc' and 'badInt', got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Invalid fixed value
// ---------------------------------------------------------------------------

func TestValidate_InvalidFixed(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="flag" type="xs:boolean" fixed="maybe"/>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "bad_fixed.xsd")
	if err == nil {
		t.Fatal("expected error for invalid fixed value")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Errorf("error should mention 'maybe', got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Facet narrowing violation — derived widens base
// ---------------------------------------------------------------------------

func TestValidate_FacetNarrowingViolation(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:maxLength value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:maxLength value="20"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "narrow.xsd")
	if err == nil {
		t.Fatal("expected error for facet narrowing violation (maxLength 20 > 10)")
	}
	if !strings.Contains(err.Error(), "maxLength") {
		t.Errorf("error should mention maxLength, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Valid narrowing — derived narrows base
// ---------------------------------------------------------------------------

func TestValidate_FacetNarrowingValid(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:maxLength value="100"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:maxLength value="50"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "narrow_ok.xsd")
	if err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidationConfig: Warn level — parses successfully, doesn't error
// ---------------------------------------------------------------------------

func TestValidate_WarnLevel(t *testing.T) {
	vc := config.NewValidationConfig()
	vc.SetRule(config.RuleFacetApplicability, config.ValidationWarn)
	vc.SetRule(config.RuleFacetCrossValidation, config.ValidationWarn)

	p := New(WithSchemaStrictness(vc))

	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="BadString">
    <xs:restriction base="xs:string">
      <xs:totalDigits value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	_, err := p.ParseReader(strings.NewReader(input), "warn.xsd")
	if err != nil {
		t.Fatalf("warn level should not produce error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidationConfig: Off level — no validation at all
// ---------------------------------------------------------------------------

func TestValidate_OffLevel(t *testing.T) {
	vc := config.NewValidationConfig()
	vc.SetRule(config.RuleFacetApplicability, config.ValidationOff)
	vc.SetRule(config.RuleFacetCrossValidation, config.ValidationOff)

	p := New(WithSchemaStrictness(vc))

	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="BadString">
    <xs:restriction base="xs:string">
      <xs:totalDigits value="5"/>
      <xs:minLength value="10"/>
      <xs:maxLength value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	_, err := p.ParseReader(strings.NewReader(input), "off.xsd")
	if err != nil {
		t.Fatalf("off level should not produce error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidationConfig: Default value check can be turned off
// ---------------------------------------------------------------------------

func TestValidate_DefaultValueOff(t *testing.T) {
	vc := config.NewValidationConfig()
	vc.SetRule(config.RuleDefaultValue, config.ValidationOff)

	p := New(WithSchemaStrictness(vc))

	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="badInt" type="xs:integer" default="abc"/>
</xs:schema>`

	_, err := p.ParseReader(strings.NewReader(input), "def_off.xsd")
	if err != nil {
		t.Fatalf("default validation off should not error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Multiple validation errors combined
// ---------------------------------------------------------------------------

func TestValidate_MultipleErrors(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="Bad1">
    <xs:restriction base="xs:string">
      <xs:totalDigits value="5"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Bad2">
    <xs:restriction base="xs:string">
      <xs:fractionDigits value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "multi.xsd")
	if err == nil {
		t.Fatal("expected error for multiple invalid facets")
	}
	// Should contain both errors.
	errStr := err.Error()
	if !strings.Contains(errStr, "totalDigits") {
		t.Errorf("error should mention totalDigits, got: %s", errStr)
	}
	if !strings.Contains(errStr, "fractionDigits") {
		t.Errorf("error should mention fractionDigits, got: %s", errStr)
	}
}

// ---------------------------------------------------------------------------
// Cross-validation: minInclusive > maxInclusive
// ---------------------------------------------------------------------------

func TestValidate_MinMaxInclusiveCross(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="BadRange">
    <xs:restriction base="xs:integer">
      <xs:minInclusive value="100"/>
      <xs:maxInclusive value="10"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "cross.xsd")
	if err == nil {
		t.Fatal("expected error for minInclusive > maxInclusive")
	}
	if !strings.Contains(err.Error(), "minInclusive") {
		t.Errorf("error should mention minInclusive, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Valid default values pass validation
// ---------------------------------------------------------------------------

func TestValidate_ValidDefaults(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="name" type="xs:string" default="John"/>
  <xs:element name="age" type="xs:integer" default="25"/>
  <xs:element name="flag" type="xs:boolean" default="true"/>
  <xs:element name="price" type="xs:decimal" default="9.99"/>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "valid_defaults.xsd")
	if err != nil {
		t.Fatalf("expected valid defaults to pass, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Invalid facet on derived user type (totalDigits on token, which is string)
// ---------------------------------------------------------------------------

func TestValidate_InvalidFacetOnDerived(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:simpleType name="BadToken">
    <xs:restriction base="xs:token">
      <xs:totalDigits value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	p := newTestParser()
	_, err := p.ParseReader(strings.NewReader(input), "bad_token.xsd")
	if err == nil {
		t.Fatal("expected error: totalDigits not applicable to token (string family)")
	}
}
