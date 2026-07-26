// Package sliceutil provides small slice helpers shared across the backend.
// It replaces the per-package dedup copies consolidated in the 2026-07
// code-reduction audit (B7). For sorted output, prefer stdlib
// slices.Sort + slices.Compact directly.
package sliceutil

// Unique returns xs with duplicates removed, preserving first-occurrence
// order. Empty or nil input is returned as-is.
func Unique[T comparable](xs []T) []T {
	if len(xs) == 0 {
		return xs
	}
	seen := make(map[T]struct{}, len(xs))
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// UniquePositive returns the positive values of ids with duplicates removed,
// preserving first-occurrence order. Returns nil for empty input.
func UniquePositive(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
