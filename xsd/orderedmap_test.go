package xsd_test

import (
	"testing"

	"github.com/kud360/goxsd3/xsd"
)

func TestOrderedMapBasic(t *testing.T) {
	m := xsd.NewOrderedMap[string, int]()
	m.Set("b", 2)
	m.Set("a", 1)
	m.Set("c", 3)

	if m.Len() != 3 {
		t.Fatalf("expected len 3, got %d", m.Len())
	}

	v, ok := m.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get('a') = %d, %v; want 1, true", v, ok)
	}

	_, ok = m.Get("missing")
	if ok {
		t.Fatal("Get('missing') should return false")
	}
}

func TestOrderedMapInsertionOrder(t *testing.T) {
	m := xsd.NewOrderedMap[string, int]()
	m.Set("c", 3)
	m.Set("a", 1)
	m.Set("b", 2)

	keys := m.Keys()
	expected := []string{"c", "a", "b"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Fatalf("key[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestOrderedMapUpdatePreservesOrder(t *testing.T) {
	m := xsd.NewOrderedMap[string, int]()
	m.Set("x", 1)
	m.Set("y", 2)
	m.Set("z", 3)
	m.Set("x", 10) // update, should not change position

	keys := m.Keys()
	if keys[0] != "x" {
		t.Fatalf("updated key moved; got keys %v", keys)
	}
	v, _ := m.Get("x")
	if v != 10 {
		t.Fatalf("updated value not stored; got %d", v)
	}
}

func TestOrderedMapRange(t *testing.T) {
	m := xsd.NewOrderedMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	var keys []string
	var values []int
	m.Range(func(k string, v int) {
		keys = append(keys, k)
		values = append(values, v)
	})

	if len(keys) != 3 || keys[0] != "a" || keys[2] != "c" {
		t.Fatalf("Range iterated out of order: %v", keys)
	}
	if values[0] != 1 || values[2] != 3 {
		t.Fatalf("Range returned wrong values: %v", values)
	}
}

func TestOrderedMapEmpty(t *testing.T) {
	m := xsd.NewOrderedMap[int, string]()
	if m.Len() != 0 {
		t.Fatal("new map should be empty")
	}
	if keys := m.Keys(); len(keys) != 0 {
		t.Fatal("new map should have no keys")
	}
	_, ok := m.Get(42)
	if ok {
		t.Fatal("Get on empty map should return false")
	}
}

func TestOrderedMapQNameKeys(t *testing.T) {
	m := xsd.NewOrderedMap[xsd.QName, *xsd.SimpleType]()
	name1 := xsd.NewQName("urn:a", "TypeA")
	name2 := xsd.NewQName("urn:b", "TypeB")
	t1 := &xsd.SimpleType{Name: name1}
	t2 := &xsd.SimpleType{Name: name2}

	m.Set(name1, t1)
	m.Set(name2, t2)

	got, ok := m.Get(name1)
	if !ok || got != t1 {
		t.Fatal("QName lookup failed")
	}
	if m.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", m.Len())
	}
}
