package parser_test

import (
	"fmt"
	"testing"

	"github.com/kud360/goxsd3/parser"
)

func TestResolverFunc(t *testing.T) {
	called := false
	fn := parser.ResolverFunc(func(location, baseURI, namespace string) ([]byte, error) {
		called = true
		if location != "types.xsd" {
			t.Fatalf("unexpected location %q", location)
		}
		if baseURI != "/schemas/" {
			t.Fatalf("unexpected baseURI %q", baseURI)
		}
		if namespace != "urn:test" {
			t.Fatalf("unexpected namespace %q", namespace)
		}
		return []byte("<xs:schema/>"), nil
	})

	var resolver parser.SchemaResolver = fn
	data, err := resolver.Resolve("types.xsd", "/schemas/", "urn:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("resolver function was not called")
	}
	if string(data) != "<xs:schema/>" {
		t.Fatalf("unexpected data: %q", string(data))
	}
}

func TestResolverFuncError(t *testing.T) {
	fn := parser.ResolverFunc(func(_, _, _ string) ([]byte, error) {
		return nil, fmt.Errorf("not found")
	})

	_, err := fn.Resolve("missing.xsd", "", "")
	if err == nil || err.Error() != "not found" {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}
