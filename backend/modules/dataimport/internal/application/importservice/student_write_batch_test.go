package importpkg

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	dataimport "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
)

type writeMatchTransactions struct {
	dataimport.Transactions
}

func (writeMatchTransactions) Savepoint(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestStudentWriteBatchReadFailureFallsBackWithoutRetainingFailure(t *testing.T) {
	t.Parallel()
	lookupError := errors.New("batch lookup failed")
	batchReads, scalarReads := 0, 0
	config := &StudentImportConfig{StudentImportDeps: ports.StudentImportDeps{
		Transactions: writeMatchTransactions{},
		Persons: PersonDirectoryFuncs{SearchPersonsFunc: func(_ context.Context, filter ports.PersonFilter) ([]ports.Person, error) {
			if len(filter.Names) > 0 {
				batchReads++
				return nil, lookupError
			}
			scalarReads++
			return nil, nil
		}},
	}}
	rows := make([]dataimport.StudentImportRow, 5)
	for i, last := range []string{"One", "Two", "Three", "Four", "Five"} {
		rows[i] = dataimport.StudentImportRow{FirstName: "First", LastName: last, SchoolClass: "1A"}
	}
	ctx := context.WithValue(context.Background(), importModeKey{}, dataimport.ImportModeCreate)
	prepared, err := config.BatchWriteConfig(ctx, rows)
	require.NoError(t, err)
	require.Same(t, config, prepared)
	for _, row := range rows {
		id, err := prepared.FindExisting(ctx, row)
		require.NoError(t, err)
		require.Nil(t, id)
	}
	require.Equal(t, 1, batchReads)
	require.Equal(t, len(rows), scalarReads)
	// Each new attempt rebuilds its read set instead of reusing the failure.
	again, err := config.BatchWriteConfig(ctx, rows)
	require.NoError(t, err)
	require.Same(t, config, again)
	require.Equal(t, 2, batchReads)
}

func TestStudentWriteBatchLeavesUpdateAndUpsertOnScalarReaders(t *testing.T) {
	t.Parallel()
	config := &StudentImportConfig{}
	for _, mode := range []dataimport.ImportMode{dataimport.ImportModeUpdate, dataimport.ImportModeUpsert} {
		ctx := context.WithValue(context.Background(), importModeKey{}, mode)
		prepared, err := config.BatchWriteConfig(ctx, make([]dataimport.StudentImportRow, 8))
		require.NoError(t, err)
		require.Same(t, config, prepared)
	}
}
