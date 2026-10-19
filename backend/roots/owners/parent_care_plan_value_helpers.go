package owners

func parentCarePlanPointer[A, B any](value *A, convert func(A) B) *B {
	if value == nil {
		return nil
	}
	result := convert(*value)
	return &result
}

func parentCarePlanSlice[A, B any](values []A, convert func(A) B) []B {
	if values == nil {
		return nil
	}
	result := make([]B, len(values))
	for i, value := range values {
		result[i] = convert(value)
	}
	return result
}
