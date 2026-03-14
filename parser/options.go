package parser

import (
	"log/slog"

	"github.com/kud360/goxsd3/config"
)

// Options holds configuration for the parser.
type Options struct {
	Logger           *slog.Logger
	Resolver         SchemaResolver
	SchemaStrictness *config.ValidationConfig
}

// defaults fills in zero-value options with sensible defaults.
func (o *Options) defaults() {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.SchemaStrictness == nil {
		o.SchemaStrictness = config.NewValidationConfig()
	}
}

// Option is a functional option for configuring the parser.
type Option func(*Options)

// WithLogger sets the structured logger for the parser.
func WithLogger(l *slog.Logger) Option {
	return func(o *Options) { o.Logger = l }
}

// WithResolver sets the schema resolver for import/include handling.
func WithResolver(r SchemaResolver) Option {
	return func(o *Options) { o.Resolver = r }
}

// WithSchemaStrictness sets the validation strictness configuration.
func WithSchemaStrictness(c *config.ValidationConfig) Option {
	return func(o *Options) { o.SchemaStrictness = c }
}
