package students

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// TestCommonIsNotFoundMatchesRepositorySentinel pins api/common.IsNotFound
// against the real repository sentinel.
//
// api/common may import neither models/base (inbound-common/http may not
// import transaction-runtime/domain) nor database/sql (external class
// orm-sql), so IsNotFound matches models/base.ErrNotFound by its message text
// instead of by identity, and nothing inside api/common can prove that text is
// still the sentinel's. This package already sees both, and its handlers
// classify not-found the same way, so the pin lives here: if
// models/base.ErrNotFound's message ever changes, this test fails instead of
// every api/time-tracking handler silently answering 500 for a missing row.
//
// The error values mirror database/repositories/base.TranslateNotFound, which
// is the only producer of the sentinel: errors.Join(ErrNotFound, sql.ErrNoRows).
func TestCommonIsNotFoundMatchesRepositorySentinel(t *testing.T) {
	t.Parallel()

	repositoryResult := errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			// What every repository read returns for a missing row.
			name: "repository not-found result",
			err:  repositoryResult,
			want: true,
		},
		{
			// What services hand handlers after wrapping that result.
			name: "wrapped in a DatabaseError",
			err:  &modelBase.DatabaseError{Op: "find by id", Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)},
			want: true,
		},
		{
			name: "wrapped with %w",
			err:  fmt.Errorf("load staff: %w", errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("other"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := common.IsNotFound(tt.err); got != tt.want {
				t.Fatalf("common.IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
			// The two classifiers must stay interchangeable: handlers inside
			// the architecture boundary keep using models/base.IsNoRows.
			if got, reference := common.IsNotFound(tt.err), modelBase.IsNoRows(tt.err); got != reference {
				t.Fatalf("common.IsNotFound = %v but models/base.IsNoRows = %v for %v", got, reference, tt.err)
			}
		})
	}
}
