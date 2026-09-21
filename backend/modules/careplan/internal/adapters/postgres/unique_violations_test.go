package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

func TestStudentDataRequestCreateErrorMarksOnlyThePendingFieldIndex(t *testing.T) {
	t.Parallel()
	pending := requestConstraintFixture{"23505", pendingStudentDataFieldIndex}
	for name, test := range map[string]struct {
		err     error
		pending bool
	}{
		"pending field index":      {pending, true},
		"wrapped pending index":    {fmt.Errorf("insert: %w", pending), true},
		"other unique index":       {requestConstraintFixture{"23505", "student_data_change_requests_pkey"}, false},
		"foreign key on the index": {requestConstraintFixture{"23503", "other_fk"}, false},
		"unrelated failure":        {errors.New("connection reset"), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := studentDataRequestCreateError(test.err)
			assert.Equal(t, test.pending, errors.Is(err, careplan.ErrStudentDataRequestFieldPending))
			assert.ErrorIs(t, err, test.err, "the raw database error stays in the chain")
		})
	}
}
