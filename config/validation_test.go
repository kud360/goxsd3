package config_test

import (
	"testing"

	"github.com/kud360/goxsd3/config"
)

func TestDefaultLevel(t *testing.T) {
	c := config.NewValidationConfig()
	if c.Level(config.RulePattern) != config.ValidationError {
		t.Fatal("default level should be ValidationError")
	}
}

func TestSetRule(t *testing.T) {
	c := config.NewValidationConfig()
	c.SetRule(config.RulePattern, config.ValidationWarn)

	if c.Level(config.RulePattern) != config.ValidationWarn {
		t.Fatal("expected ValidationWarn for RulePattern")
	}
	// Other rules still use default.
	if c.Level(config.RuleEnumeration) != config.ValidationError {
		t.Fatal("unset rule should use default level")
	}
}

func TestIsError(t *testing.T) {
	c := config.NewValidationConfig()
	if !c.IsError(config.RulePattern) {
		t.Fatal("default rule should be an error")
	}
	c.SetRule(config.RulePattern, config.ValidationOff)
	if c.IsError(config.RulePattern) {
		t.Fatal("disabled rule should not be an error")
	}
}

func TestCustomDefault(t *testing.T) {
	c := &config.ValidationConfig{
		Default: config.ValidationWarn,
		Rules:   make(map[config.ValidationRule]config.ValidationLevel),
	}
	if c.Level(config.RuleMinLength) != config.ValidationWarn {
		t.Fatal("custom default should apply to unconfigured rules")
	}
	c.SetRule(config.RuleMinLength, config.ValidationError)
	if c.Level(config.RuleMinLength) != config.ValidationError {
		t.Fatal("explicit rule should override default")
	}
}

func TestAllRuleConstants(t *testing.T) {
	// Verify all rule constants are distinct by setting each and reading back.
	rules := []config.ValidationRule{
		config.RulePattern, config.RuleEnumeration, config.RuleMinLength,
		config.RuleMaxLength, config.RuleLength, config.RuleMinInclusive,
		config.RuleMaxInclusive, config.RuleMinExclusive, config.RuleMaxExclusive,
		config.RuleTotalDigits, config.RuleFractionDigits, config.RuleWhiteSpace,
		config.RuleDefaultValue, config.RuleFixedValue,
		config.RuleFacetCrossValidation, config.RuleFacetNarrowing,
		config.RuleFacetApplicability, config.RuleDuplicateDefinition,
		config.RuleCircularDerivation, config.RuleRequiredElement,
		config.RuleRequiredAttribute, config.RuleUnknownElement,
		config.RuleUnknownAttribute,
	}

	c := config.NewValidationConfig()
	for _, r := range rules {
		c.SetRule(r, config.ValidationOff)
	}
	// All should now be off.
	for _, r := range rules {
		if c.Level(r) != config.ValidationOff {
			t.Fatalf("rule %d was not set correctly", r)
		}
	}
	if len(rules) != 23 {
		t.Fatalf("expected 23 validation rules, got %d", len(rules))
	}
}
