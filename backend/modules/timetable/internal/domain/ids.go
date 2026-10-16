package domain

// UniqueInOrder returns values without duplicates in first-occurrence order;
// empty or nil input is returned as is.
func UniqueInOrder[T comparable](values []T) []T {
	if len(values) == 0 {
		return values
	}
	seen := make(map[T]struct{}, len(values))
	result := make([]T, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// UniquePositiveIDs returns the positive IDs without duplicates in
// first-occurrence order, nil for empty input.
func UniquePositiveIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	positive := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			positive = append(positive, id)
		}
	}
	return UniqueInOrder(positive)
}
