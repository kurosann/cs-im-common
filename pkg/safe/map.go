package safe

import "sync"

// Map 是基于 sync.Map 封装的泛型并发 Map
type Map[K comparable, V any] struct {
	m sync.Map
}

// Store 存储键值对
func (s *Map[K, V]) Store(key K, value V) {
	s.m.Store(key, value)
}

// Load 读取键值对，ok 为 false 表示键不存在
func (s *Map[K, V]) Load(key K) (V, bool) {
	val, ok := s.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// LoadOrStore 若键已存在则返回原值，不存在则存入新值
func (s *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	actual, loaded := s.m.LoadOrStore(key, value)
	return actual.(V), loaded
}

// LoadAndDelete 读取并删除键值对
func (s *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	val, loaded := s.m.LoadAndDelete(key)
	if !loaded {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Delete 删除键值对
func (s *Map[K, V]) Delete(key K) {
	s.m.Delete(key)
}

// Range 遍历所有元素，返回 false 则中断遍历
func (s *Map[K, V]) Range(f func(key K, value V) bool) {
	s.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}
