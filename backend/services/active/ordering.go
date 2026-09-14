package active

import "log/slog"

// ordered lists the field types the retained sort helpers compare.
type ordered interface {
	~int | ~int64 | ~float64 | ~string
}

// compareOrdered returns -1, 0 or +1 like a three-way comparison.
func compareOrdered[T ordered](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// orderOr returns the first non-zero comparison result, so callers can chain
// a primary order with a tiebreak.
func orderOr(order, tiebreak int) int {
	if order != 0 {
		return order
	}
	return tiebreak
}

// loggerOrDefault keeps bare-constructed services nil-safe.
func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
