// Package ptrtest provides pointer literals for tests without adding helpers
// to production model APIs.
package ptrtest

// Ptr returns a pointer to value.
func Ptr[T any](value T) *T { return &value }
