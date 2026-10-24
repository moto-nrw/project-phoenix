package database

import "github.com/uptrace/bun"

func ObservedBinder[T any](bind func(*bun.DB) (T, error), observe func(*bun.DB)) func(*bun.DB) (T, error) {
	return func(pool *bun.DB) (T, error) { observe(pool); return bind(pool) }
}
