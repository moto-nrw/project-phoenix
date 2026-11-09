package runtimebinding

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// ImportSavepoint marks only failures of its own savepoint statements, told
// apart by what the savepoint returns; a failed savepoint nested in fn stays
// fn's error (#5552 review of 1bb4bde258).
func TestImportSavepointMarksOnlyItsOwnControlFailures(t *testing.T) {
	t.Parallel()
	nested := errors.Join(errors.New("link failed"), errors.New("nested rollback failed"))
	own := errors.New("rollback failed")
	cases := []struct {
		name      string
		savepoint func(context.Context, func(context.Context) error) error
		fn        func(context.Context) error
		marked    bool
		returned  error
	}{
		{"success", func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, func(context.Context) error { return nil }, false, nil},
		{"fn fails, own rollback succeeds", func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
			func(context.Context) error { return nested }, false, nested},
		{"own rollback fails", func(ctx context.Context, fn func(context.Context) error) error { return errors.Join(fn(ctx), own) },
			func(context.Context) error { return nested }, true, nil},
		{"own create fails", func(context.Context, func(context.Context) error) error { return own },
			func(context.Context) error { return nil }, true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := ImportSavepoint(context.Background(), c.fn, c.savepoint)
			if marked := errors.Is(err, dataimport.ErrSavepointControl); marked != c.marked {
				t.Fatalf("marked = %t, want %t (%v)", marked, c.marked, err)
			}
			if !c.marked && err != c.returned { //nolint:errorlint // identity: fn's own error comes back unchanged
				t.Fatalf("returned %v, want fn's error %v", err, c.returned)
			}
		})
	}
}
