package base

import "time"

// Pointer helper functions for creating pointers to primitive values in tests.
// These are in a separate file for clarity but remain in the base package
// to avoid import cycles in model tests.

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string { return &s }

// IntPtr returns a pointer to the given int.
func IntPtr(i int) *int { return &i }

// Int64Ptr returns a pointer to the given int64.
func Int64Ptr(i int64) *int64 { return &i }

// TimePtr returns a pointer to the given time.Time.
func TimePtr(t time.Time) *time.Time { return &t }
