// Package config provides configuration types for controlling validation
// strictness and behavior throughout the goxsd3 pipeline.
package config

// ValidationLevel controls how a validation rule behaves when violated.
type ValidationLevel int

const (
	// ValidationError rejects the input when the rule is violated.
	ValidationError ValidationLevel = iota
	// ValidationWarn logs a warning but accepts the input.
	ValidationWarn
	// ValidationOff skips the check entirely.
	ValidationOff
)

// ValidationRule identifies a specific validation check that can be
// configured independently.
type ValidationRule int

const (
	// Facet rules (both schema and data parsing).
	RulePattern ValidationRule = iota
	RuleEnumeration
	RuleMinLength
	RuleMaxLength
	RuleLength
	RuleMinInclusive
	RuleMaxInclusive
	RuleMinExclusive
	RuleMaxExclusive
	RuleTotalDigits
	RuleFractionDigits
	RuleWhiteSpace

	// Schema-parsing rules.
	RuleDefaultValue
	RuleFixedValue
	RuleFacetCrossValidation
	RuleFacetNarrowing
	RuleFacetApplicability
	RuleDuplicateDefinition
	RuleCircularDerivation

	// Data-parsing rules.
	RuleRequiredElement
	RuleRequiredAttribute
	RuleUnknownElement
	RuleUnknownAttribute
)

// ValidationConfig controls per-rule validation strictness. Rules not
// explicitly configured use the Default level.
type ValidationConfig struct {
	Default ValidationLevel
	Rules   map[ValidationRule]ValidationLevel
}

// NewValidationConfig creates a config with all rules at ValidationError.
func NewValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		Default: ValidationError,
		Rules:   make(map[ValidationRule]ValidationLevel),
	}
}

// Level returns the effective validation level for the given rule.
func (c *ValidationConfig) Level(rule ValidationRule) ValidationLevel {
	if lvl, ok := c.Rules[rule]; ok {
		return lvl
	}
	return c.Default
}

// SetRule configures the validation level for a specific rule.
func (c *ValidationConfig) SetRule(rule ValidationRule, level ValidationLevel) {
	c.Rules[rule] = level
}

// IsError reports whether the given rule is configured to produce errors.
func (c *ValidationConfig) IsError(rule ValidationRule) bool {
	return c.Level(rule) == ValidationError
}
