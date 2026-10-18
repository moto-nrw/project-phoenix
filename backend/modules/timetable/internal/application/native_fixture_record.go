//go:build testhooks

package application

import "context"

type fixtureRawRecord[R any] interface {
	Create(context.Context, *R) error
	Update(context.Context, *R) error
	FindByID(context.Context, any) (*R, error)
}
type fixtureRecord[T, R any] struct {
	rows   fixtureRawRecord[R]
	encode func(*T) (*R, error)
	decode func(*R) *T
}

func (r fixtureRecord[T, R]) Create(ctx context.Context, value *T) error {
	row, err := r.encode(value)
	if err != nil {
		return err
	}
	err = r.rows.Create(ctx, row)
	if value != nil && row != nil {
		*value = *r.decode(row)
	}
	return err
}
func (r fixtureRecord[T, R]) Update(ctx context.Context, value *T) error {
	row, err := r.encode(value)
	if err != nil {
		return err
	}
	err = r.rows.Update(ctx, row)
	if value != nil && row != nil {
		*value = *r.decode(row)
	}
	return err
}
func (r fixtureRecord[T, R]) FindByID(ctx context.Context, id any) (*T, error) {
	row, err := r.rows.FindByID(ctx, id)
	return r.decode(row), err
}
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
