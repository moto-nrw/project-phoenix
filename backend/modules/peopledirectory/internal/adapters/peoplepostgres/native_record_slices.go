package users

func mapRecordSlice[S, T any](values []S, convert func(S) T) []T {
	if values == nil {
		return nil
	}
	result := make([]T, len(values))
	for i, value := range values {
		result[i] = convert(value)
	}
	return result
}

func mapRecordValues[K comparable, S, T any](values map[K]S, convert func(S) T) map[K]T {
	if values == nil {
		return nil
	}
	result := make(map[K]T, len(values))
	for k, value := range values {
		result[k] = convert(value)
	}
	return result
}
