package xsd

// OrderedMap is a generic map that preserves insertion order for deterministic
// iteration while providing O(1) key lookup.
type OrderedMap[K comparable, V any] struct {
	keys   []K
	values map[K]V
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		values: make(map[K]V),
	}
}

// Set inserts or updates a key-value pair. New keys are appended to the
// insertion-order list; existing keys retain their original position.
func (m *OrderedMap[K, V]) Set(key K, value V) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get returns the value for key and whether it was found.
func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.values[key]
	return v, ok
}

// Keys returns all keys in insertion order.
func (m *OrderedMap[K, V]) Keys() []K {
	out := make([]K, len(m.keys))
	copy(out, m.keys)
	return out
}

// Len returns the number of entries.
func (m *OrderedMap[K, V]) Len() int {
	return len(m.keys)
}

// Range calls fn for each entry in insertion order.
func (m *OrderedMap[K, V]) Range(fn func(K, V)) {
	for _, k := range m.keys {
		fn(k, m.values[k])
	}
}
