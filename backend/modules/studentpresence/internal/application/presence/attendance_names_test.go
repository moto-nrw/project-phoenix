package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type attendanceStaffNameFunc func(context.Context, int64) (string, error)

func (f attendanceStaffNameFunc) StaffName(ctx context.Context, id int64) (string, error) {
	return f(ctx, id)
}

func TestAttendanceStaffNamePreservesDisplayAndLookupFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value string
		err   error
		want  string
	}{
		{name: "full name", value: "Anna Beispiel", want: "Anna Beispiel"},
		{name: "formatting unchanged", value: "Anna ", want: "Anna "},
		{name: "missing staff"},
		{name: "failed lookup", value: "partial", err: errors.New("directory unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			svc := &service{ServiceDependencies: ServiceDependencies{StaffNames: attendanceStaffNameFunc(func(got context.Context, id int64) (string, error) {
				assert.Equal(t, ctx, got)
				assert.Equal(t, int64(41), id)
				return tc.value, tc.err
			})}}
			assert.Equal(t, tc.want, svc.getStaffNameByID(ctx, 41))
		})
	}
}
