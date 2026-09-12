package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type RequestReviewSchool struct {
	Context   context.Context
	TenantID  int64
	StudentID int64
	GroupName string
	Rows      map[string]int64
}

// CreateTestRequestReviewSchool creates one request of each kind with a real
// student, guardian account, group, and approved enrollment FK chain.
func CreateTestRequestReviewSchool(t *testing.T, db *bun.DB, side string) RequestReviewSchool {
	t.Helper()
	ctx, tenantID := Ctx(t), Tenant(t)
	teacher, account := CreateTestTeacherWithAccount(t, db, "ReviewRLS", side)
	group := CreateTestEducationGroup(t, db, "ReviewRLS-"+side)
	CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	student := CreateTestStudent(t, db, "ReviewRLS", side, "RR1")
	AssignStudentToGroup(t, db, student.ID, group.ID)
	result := RequestReviewSchool{TenantID: tenantID, StudentID: student.ID, GroupName: group.Name, Rows: map[string]int64{}}
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_data_change_requests
 (tenant_id,student_id,submitted_by,target,field_key,new_value,status)
 VALUES (?,?,?,'person','first_name','"Neu"'::jsonb,'pending') RETURNING id`, tenantID, student.ID, account.ID).Scan(ctx, &id))
	result.Rows["users.student_data_change_requests"] = id
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.care_schedule_change_requests
 (tenant_id,student_id,submitted_by,request_kind,payload,status)
 VALUES (?,?,?,'weekly_schedule','{"weekdays":[{"weekday":1,"pickup":"15:00"}]}'::jsonb,'pending') RETURNING id`, tenantID, student.ID, account.ID).Scan(ctx, &id))
	result.Rows["schedule.care_schedule_change_requests"] = id
	require.NoError(t, db.NewRaw(`INSERT INTO active.excused_absence_requests
 (tenant_id,student_id,submitted_by,dates,note,absence_status,status)
 VALUES (?,?,?,?::jsonb,'Arzttermin','excused','pending') RETURNING id`, tenantID, student.ID, account.ID, fmt.Sprintf(`["%s"]`, timezone.TodayDate().AddDays(3))).Scan(ctx, &id))
	result.Rows["active.excused_absence_requests"] = id
	phase := CreateTestEnrollmentPhase(t, db)
	care := CreateTestCareOffering(t, db, phase.ID, "Ganztag "+side)
	lunch := CreateTestCareOffering(t, db, phase.ID, "Mittagessen "+side)
	var requestID, childID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests
 (tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token)
 VALUES (?,?,'Erzieh',?,?,?) RETURNING id`, tenantID, phase.ID, side, fmt.Sprintf("review-rls-%s-%d@example.test", side, UniqueSuffix()), fmt.Sprintf("tok-%d", UniqueSuffix())).Scan(ctx, &requestID))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children
 (tenant_id,request_id,first_name,last_name,date_of_birth,status,created_student_id)
 VALUES (?,?,'ReviewRLS',?,?::date,'approved',?) RETURNING id`, tenantID, requestID, side, "2019-10-20", student.ID).Scan(ctx, &childID))
	for _, offeringID := range []int64{care.ID, lunch.ID} {
		_, err := db.NewRaw(`INSERT INTO enrollment.request_child_offerings
  (tenant_id,request_child_id,care_offering_id) VALUES (?,?,?)`, tenantID, childID, offeringID).Exec(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.offering_change_requests
 (tenant_id,student_id,request_child_id,submitted_by,payload,effective_from,status)
 VALUES (?,?,?,?,?::jsonb,?::date,'pending') RETURNING id`, tenantID, student.ID, childID, account.ID, fmt.Sprintf(`{"offerings":[{"offering_id":%d}]}`, care.ID), timezone.TodayDate().AddDays(30).String()).Scan(ctx, &id))
	result.Rows["enrollment.offering_change_requests"] = id
	result.Context = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(account.ID)})
	result.Context = context.WithValue(result.Context, jwt.CtxPermissions, []string{"admin:*", "users:read", "users:update", "users:absence"})
	return result
}
