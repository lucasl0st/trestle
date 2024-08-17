package util

import "sync"

// SafeMap is a thread-safe map
type SafeMap[KeyT comparable, ValueT any] struct {
	sync.RWMutex
	m map[KeyT]ValueT
}

// NewSafeMap creates a new SafeMap
func NewSafeMap[KeyT comparable, ValueT any]() *SafeMap[KeyT, ValueT] {
	return &SafeMap[KeyT, ValueT]{
		m: map[KeyT]ValueT{},
	}
}

// Get returns the value for the given key
func (m *SafeMap[KeyT, ValueT]) Get(key KeyT) (ValueT, bool) {
	m.RLock()
	defer m.RUnlock()

	value, ok := m.m[key]
	return value, ok
}

// Set sets the value for the given key
func (m *SafeMap[KeyT, ValueT]) Set(key KeyT, value ValueT) {
	m.Lock()
	defer m.Unlock()

	m.m[key] = value
}

// Delete deletes the value for the given key
func (m *SafeMap[KeyT, ValueT]) Delete(key KeyT) {
	m.Lock()
	defer m.Unlock()

	delete(m.m, key)
}

// Keys returns all keys
func (m *SafeMap[KeyT, ValueT]) Keys() []KeyT {
	m.RLock()
	defer m.RUnlock()

	var keys []KeyT

	for key := range m.m {
		keys = append(keys, key)
	}

	return keys
}

// Range iterates over all key-value pairs
func (m *SafeMap[KeyT, ValueT]) Range(worker func(key KeyT, value ValueT) bool) {
	keys := m.Keys()

	for _, key := range keys {
		value, ok := m.Get(key)
		if !ok {
			continue
		}

		if !worker(key, value) {
			break
		}
	}
}
