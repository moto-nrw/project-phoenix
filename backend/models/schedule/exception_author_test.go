package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStudentPickupException_Validate_Author exercises the source/author
// invariant that migration 1.15.136 introduced: a guardian-authored row is
// keyed by created_by_guardian (not created_by), and an unknown source is
// rejected. The staff-author branches are covered by the original
// TestStudentPickupException_Validate cases; these add the guardian + invalid
// branches.
func TestStudentPickupException_Validate_Author(t *testing.T) {
	t.Parallel()

	date := NewDate(2026, 6, 15)
	pickup := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)
	guardian := int64(42)

	tests := []struct {
		name    string
		setup   func() *StudentPickupException
		wantErr bool
		errMsg  string
	}{
		{
			name: "guardian source with guardian id is valid",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:         1,
					ExceptionDate:     date,
					PickupTime:        &pickup,
					Source:            ExceptionSourceGuardian,
					CreatedByGuardian: &guardian,
				}
			},
			wantErr: false,
		},
		{
			name: "guardian source without guardian id is rejected",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: date,
					PickupTime:    &pickup,
					Source:        ExceptionSourceGuardian,
				}
			},
			wantErr: true,
			errMsg:  errMsgGuardianAuthorRequired,
		},
		{
			name: "guardian source with non-positive guardian id is rejected",
			setup: func() *StudentPickupException {
				zero := int64(0)
				return &StudentPickupException{
					StudentID:         1,
					ExceptionDate:     date,
					PickupTime:        &pickup,
					Source:            ExceptionSourceGuardian,
					CreatedByGuardian: &zero,
				}
			},
			wantErr: true,
			errMsg:  errMsgGuardianAuthorRequired,
		},
		{
			name: "unknown source is rejected",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:         1,
					ExceptionDate:     date,
					PickupTime:        &pickup,
					Source:            "robot",
					CreatedByGuardian: &guardian,
				}
			},
			wantErr: true,
			errMsg:  "invalid exception source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup().Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestStudentArrivalException_Validate_Author mirrors the pickup author test for
// the arrival leg — both models share validateExceptionAuthor, but each model's
// Validate is a separate call site.
func TestStudentArrivalException_Validate_Author(t *testing.T) {
	t.Parallel()

	date := NewDate(2026, 6, 15)
	arrival := time.Date(2026, 6, 15, 8, 15, 0, 0, time.UTC)
	guardian := int64(42)

	tests := []struct {
		name    string
		setup   func() *StudentArrivalException
		wantErr bool
		errMsg  string
	}{
		{
			name: "guardian source with guardian id is valid",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:         1,
					ExceptionDate:     date,
					ExpectedArrival:   &arrival,
					Source:            ExceptionSourceGuardian,
					CreatedByGuardian: &guardian,
				}
			},
			wantErr: false,
		},
		{
			name: "guardian source without guardian id is rejected",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:       1,
					ExceptionDate:   date,
					ExpectedArrival: &arrival,
					Source:          ExceptionSourceGuardian,
				}
			},
			wantErr: true,
			errMsg:  errMsgGuardianAuthorRequired,
		},
		{
			name: "unknown source is rejected",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:         1,
					ExceptionDate:     date,
					ExpectedArrival:   &arrival,
					Source:            "robot",
					CreatedByGuardian: &guardian,
				}
			},
			wantErr: true,
			errMsg:  "invalid exception source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup().Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}
