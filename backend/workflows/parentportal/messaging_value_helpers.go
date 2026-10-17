package parentportal

func messagingValuePointer[A, B any](v *A, convert func(A) B) *B {
	if v == nil {
		return nil
	}
	out := convert(*v)
	return &out
}
func messagingValueSlice[A, B any](v []A, convert func(A) B) []B {
	if v == nil {
		return nil
	}
	out := make([]B, len(v))
	for i, row := range v {
		out[i] = convert(row)
	}
	return out
}

func messagingValueMap[K comparable, A, B any](v map[K]A, convert func(A) B) map[K]B {
	if v == nil {
		return nil
	}
	out := make(map[K]B, len(v))
	for k, row := range v {
		out[k] = convert(row)
	}
	return out
}
