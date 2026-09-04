package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStoreReadPathsPreserveDatabaseRuntimeFailure(t *testing.T) {
	t.Parallel()
	databaseErr := errors.New("database runtime unavailable")
	// Keep the module test independent of Bun while injecting the adapter's
	// typed database callback.
	databaseType := reflect.TypeOf(New).In(0)
	database := reflect.MakeFunc(databaseType, func([]reflect.Value) []reflect.Value {
		return []reflect.Value{reflect.Zero(databaseType.Out(0)), reflect.ValueOf(databaseErr)}
	}).Interface().(Database)
	store := New(database)
	ctx := context.Background()

	tests := map[string]func() error{
		"get": func() error {
			_, _, err := store.Get(ctx, 11)
			return err
		},
		"get for mutation": func() error {
			_, _, err := store.GetForMutation(ctx, 11)
			return err
		},
		"list": func() error {
			_, _, err := store.List(ctx, true)
			return err
		},
		"view stats": func() error {
			_, _, err := store.ViewStats(ctx, 11)
			return err
		},
	}
	for name, call := range tests {
		call := call
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, call(), databaseErr)
		})
	}
}
