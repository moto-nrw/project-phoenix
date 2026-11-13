package importpkg

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
	"github.com/stretchr/testify/require"
)

func TestPreparedPreviewReadersPreserveCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	originalError := fmt.Errorf("original directory cancellation: %w", context.Canceled)
	persons := previewPersons{PersonDirectory: PersonDirectoryFuncs{SearchPersonsFunc: func(context.Context, ports.PersonFilter) ([]ports.Person, error) { return nil, originalError }}, names: map[ports.PersonName][]ports.Person{
		previewName("First", "Last"): nil,
	}}
	_, err := persons.SearchPersons(ctx, ports.PersonFilter{FirstNameEquals: "First", LastNameEquals: "Last"})
	require.Same(t, originalError, err)
	_, err = (previewStudents{StudentDirectory: cancelledPreviewStudents{err: originalError}}).ListStudentsByPersonID(ctx, nil)
	require.Same(t, originalError, err)
}

type cancelledPreviewStudents struct {
	ports.StudentDirectory
	err error
}

func (s cancelledPreviewStudents) ListStudentsByPersonID(context.Context, []int64) ([]ports.Student, error) {
	return nil, s.err
}
