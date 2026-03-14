package parser

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileResolver resolves schema locations relative to the importing schema's
// directory on the local filesystem.
type FileResolver struct{}

// NewFileResolver creates a FileResolver.
func NewFileResolver() *FileResolver {
	return &FileResolver{}
}

// Resolve reads a schema file relative to the baseURI directory.
func (r *FileResolver) Resolve(location, baseURI, namespace string) ([]byte, error) {
	if location == "" {
		return nil, fmt.Errorf("empty schema location")
	}

	// If location is absolute, use it directly.
	resolved := location
	if !filepath.IsAbs(location) && baseURI != "" {
		resolved = filepath.Join(filepath.Dir(baseURI), location)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("resolving %q from %q: %w", location, baseURI, err)
	}
	return data, nil
}

// MultiResolver chains multiple resolvers, trying each in order until
// one succeeds.
type MultiResolver struct {
	resolvers []SchemaResolver
}

// NewMultiResolver creates a MultiResolver from the given resolvers.
func NewMultiResolver(resolvers ...SchemaResolver) *MultiResolver {
	return &MultiResolver{resolvers: resolvers}
}

// Resolve tries each resolver in order, returning the first success.
func (r *MultiResolver) Resolve(location, baseURI, namespace string) ([]byte, error) {
	var lastErr error
	for _, resolver := range r.resolvers {
		data, err := resolver.Resolve(location, baseURI, namespace)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no resolvers configured")
}
