package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type renameFailureEngine struct {
	engine
	stage   string
	failure error
}

func (e renameFailureEngine) LockSchemaLineages(context.Context) error { return nil }
func (e renameFailureEngine) Schema(context.Context, int64) (*FormSchema, error) {
	if e.stage == "load" {
		return nil, e.failure
	}
	return &FormSchema{ID: 42, Name: "School year"}, nil
}
func (e renameFailureEngine) SchemaNameExists(context.Context, string) (bool, error) {
	if e.stage == "exists" {
		return false, e.failure
	}
	return false, nil
}
func (e renameFailureEngine) RenameSchemaLineage(context.Context, string, string) error {
	return e.failure
}

type renameTestTransaction struct{}

func (renameTestTransaction) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestRenameSchemaStorageFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ stage, message string }{
		{"load", "load source schema"},
		{"exists", "check existing name"},
		{"rename", "rename schema"},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			t.Parallel()
			failure := errors.New("storage unavailable")
			module := NewModule(renameFailureEngine{stage: tc.stage, failure: failure}, renameTestTransaction{})
			schema, err := module.RenameSchema(t.Context(), 42, "Holiday")
			require.Nil(t, schema)
			require.ErrorIs(t, err, failure)
			require.ErrorContains(t, err, tc.message)
			require.NotErrorIs(t, err, ErrFormSchemaNotFound)
			require.NotErrorIs(t, err, ErrFormSchemaNameExists)
		})
	}
}
