// Package jcollections reproduces the iteration order of Java collections
// whose order Go's built-in maps don't preserve.
package jcollections

import (
	"fmt"
	"slices"
)

// HashSet iterates in the same order as a java.util.HashSet created with
// new HashSet<>(), given the same hashCode() values and the same sequence
// of Add and Clear calls.
//
// It copies java.util.HashMap: hashes spread with h ^ (h >>> 16), a
// power-of-two table that starts at 16 buckets and doubles when the size
// passes 75% of the buckets, new entries appended to the end of their
// bucket, and iteration bucket by bucket. Clear empties the set but keeps
// the table size, so the order depends on how big the set has ever been.
//
// Java also resizes when one bucket grows past 8 entries in a table smaller
// than 64, which is modelled. Past that, Java turns the bucket into a tree
// with a different order. That isn't modelled: Add panics instead.
type HashSet[T comparable] struct {
	hash    func(T) int32
	buckets [][]T
	size    int
}

const (
	defaultCapacity   = 16
	treeifyThreshold  = 8
	minTreeifyCapcity = 64
)

// NewHashSet returns an empty set that hashes elements with hash, which
// must return the element's Java hashCode().
func NewHashSet[T comparable](hash func(T) int32) *HashSet[T] {
	return &HashSet[T]{hash: hash}
}

func spread(h int32) uint32 { return uint32(h) ^ uint32(h)>>16 }

func (s *HashSet[T]) index(v T) int {
	return int(spread(s.hash(v)) & uint32(len(s.buckets)-1))
}

// Add inserts v and reports whether it was not already present.
func (s *HashSet[T]) Add(v T) bool {
	if s.buckets == nil {
		s.buckets = make([][]T, defaultCapacity)
	}
	i := s.index(v)
	if slices.Contains(s.buckets[i], v) {
		return false
	}
	s.buckets[i] = append(s.buckets[i], v)
	if len(s.buckets[i]) > treeifyThreshold {
		if len(s.buckets) < minTreeifyCapcity {
			s.resize()
		} else {
			panic(fmt.Sprintf("jcollections: a Java HashMap would turn a bucket of %d entries into a tree; that order isn't modelled",
				len(s.buckets[i])))
		}
	}
	s.size++
	if s.size > len(s.buckets)*3/4 {
		s.resize()
	}
	return true
}

// resize doubles the table. Each bucket splits into the same index and
// index+old size, keeping the entries' relative order, as Java does.
func (s *HashSet[T]) resize() {
	old := s.buckets
	s.buckets = make([][]T, len(old)*2)
	for _, bucket := range old {
		for _, v := range bucket {
			i := s.index(v)
			s.buckets[i] = append(s.buckets[i], v)
		}
	}
}

// Contains reports whether v is in the set.
func (s *HashSet[T]) Contains(v T) bool {
	if s.buckets == nil {
		return false
	}
	return slices.Contains(s.buckets[s.index(v)], v)
}

// Len returns the number of elements.
func (s *HashSet[T]) Len() int { return s.size }

// Clear removes every element but keeps the table size, like
// HashMap.clear().
func (s *HashSet[T]) Clear() {
	for i := range s.buckets {
		s.buckets[i] = s.buckets[i][:0]
	}
	s.size = 0
}

// All returns the elements in Java iteration order.
func (s *HashSet[T]) All() []T {
	out := make([]T, 0, s.size)
	for _, bucket := range s.buckets {
		out = append(out, bucket...)
	}
	return out
}
