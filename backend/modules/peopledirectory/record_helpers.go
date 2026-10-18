package peopledirectory

func recordDeref[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
