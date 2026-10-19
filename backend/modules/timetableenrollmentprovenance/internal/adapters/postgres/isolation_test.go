package timetableenrollmentprovenance_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestEnrollmentProvenanceResolvesEligibleAndExistingRowsOnlyWithinTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	day := testpkg.Date(2026, 8, 24)
	reviewed := day.BerlinMidnight().Add(12 * time.Hour)
	type side struct{ tenantID, studentID, childID, groupID, enrollmentID int64 }
	var sides []side
	for _, name := range []string{"Own", "Foreign"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			phase := testpkg.CreateTestEnrollmentPhaseAround(t, db, day)
			request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, chain.Email, chain.Email)
			group := testpkg.CreateTestActivityGroup(t, db, name+" course")
			value := side{tenantID: testpkg.Tenant(t), studentID: chain.StudentID, groupID: group.ID}
			require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id,request_id,first_name,last_name,date_of_birth,status,created_student_id,reviewed_at) VALUES (?,?,'Child','Provenance',?,'approved',?,?) RETURNING id`, value.tenantID, request.ID, testpkg.Date(2018, 2, 3), chain.StudentID, reviewed).Scan(ctx, &value.childID))
			require.NoError(t, db.NewRaw(`INSERT INTO activities.student_enrollments (tenant_id,student_id,activity_group_id,valid_from,created_at) VALUES (?,?,?,?,?) RETURNING id`, value.tenantID, chain.StudentID, group.ID, day, reviewed).Scan(ctx, &value.enrollmentID))
			sides = append(sides, value)
		})
	}
	for _, side := range sides {
		check := func(ctx context.Context, tx bun.Tx) error {
			existing, err := compose.NewReader().ExistingRequestChildIDs(ctx, tx, side.tenantID, []int64{sides[0].childID, sides[1].childID})
			require.NoError(t, err)
			require.Equal(t, []int64{side.childID}, existing)
			for _, candidate := range sides {
				ids, err := compose.NewReader().EligibleEnrollmentIDs(ctx, tx, side.tenantID, candidate.studentID, candidate.childID, []int64{sides[0].groupID, sides[1].groupID})
				require.NoError(t, err)
				if candidate.tenantID == side.tenantID {
					require.Equal(t, []int64{side.enrollmentID}, ids)
				} else {
					require.Empty(t, ids)
				}
			}
			return nil
		}
		require.NoError(t, testpkg.WithAdminTx(t, context.Background(), db, check))
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, side.tenantID, check))
	}
}
