//go:build testhooks

package application

func fixtureNativeRows[T, R any](rows []*R, decode func(*R) *T) []*T {
	if rows == nil {
		return nil
	}
	values := make([]*T, len(rows))
	for i, row := range rows {
		values[i] = decode(row)
	}
	return values
}
