// Package parser provides XSD schema parsing and resolution infrastructure.
package parser

// SchemaResolver resolves schema locations to raw bytes. Implementations
// control how schemas are fetched (filesystem, HTTP, catalog, etc.).
type SchemaResolver interface {
	// Resolve fetches the schema identified by the given location.
	//
	// Parameters:
	//   - location: the schemaLocation attribute value (relative or absolute)
	//   - baseURI: the URI of the schema containing the import/include
	//   - namespace: the target namespace (for xs:import; empty for xs:include)
	//
	// Returns the raw schema bytes or an error if the schema cannot be found.
	Resolve(location, baseURI, namespace string) ([]byte, error)
}

// ResolverFunc adapts a plain function into a SchemaResolver.
type ResolverFunc func(location, baseURI, namespace string) ([]byte, error)

// Resolve calls the underlying function.
func (f ResolverFunc) Resolve(location, baseURI, namespace string) ([]byte, error) {
	return f(location, baseURI, namespace)
}
