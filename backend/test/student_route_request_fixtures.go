package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Parent-request and guardian fixtures for the student route suites (#2731).
// The suites describe their rows with the Care Plan contract types; these
// fixtures own the mapping onto the stored tables.

// MasterDataChangeRequestRow is one parent Stammdaten request as the fixture
// stores it. Its fields match masterdatarequests.Request, so a suite converts
// the contract value with (*testpkg.MasterDataChangeRequestRow)(request).
type MasterDataChangeRequestRow struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StudentID    int64
	SubmittedBy  int64
	Target       string
	TargetRefID  *int64
	FieldKey     string
	OldValue     json.RawMessage
	NewValue     json.RawMessage
	Status       string
	ReviewReason *string
	ReviewedBy   *int64
	ReviewedAt   *time.Time
	AppliedAt    *time.Time
}

// CareScheduleChangeRequestRow is one parent care-schedule request as the
// fixture stores it. Its fields match carerequests.Request, so a suite
// converts the contract value with (*testpkg.CareScheduleChangeRequestRow)(request).
type CareScheduleChangeRequestRow struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	SubmittedBy      int64
	RequestKind      string
	Payload          json.RawMessage
	Status           string
	DecisionReason   *string
	ReviewedBy       *int64
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
	DecisionSnapshot json.RawMessage
}

// InsertTestMasterDataChangeRequest stores one parent Stammdaten request in
// users.student_data_change_requests. A zero CreatedAt or UpdatedAt takes the
// database default. The stored ID and timestamps are written back to request.
func InsertTestMasterDataChangeRequest(tb testing.TB, db bun.IDB, request *MasterDataChangeRequestRow) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &users.StudentDataChangeRequest{
		StudentID:    request.StudentID,
		SubmittedBy:  request.SubmittedBy,
		Target:       request.Target,
		TargetRefID:  request.TargetRefID,
		FieldKey:     request.FieldKey,
		OldValue:     request.OldValue,
		NewValue:     request.NewValue,
		Status:       request.Status,
		ReviewReason: request.ReviewReason,
		ReviewedBy:   request.ReviewedBy,
		ReviewedAt:   request.ReviewedAt,
		AppliedAt:    request.AppliedAt,
	}
	row.ID = request.ID
	row.TenantID = request.TenantID
	row.CreatedAt = request.CreatedAt
	row.UpdatedAt = request.UpdatedAt
	_, err := db.NewInsert().Model(row).Exec(ctx)
	require.NoError(tb, err, "Failed to create test master-data change request")
	request.ID, request.CreatedAt, request.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
}

// InsertTestCareScheduleChangeRequest stores one parent care-schedule request
// in schedule.care_schedule_change_requests. Payload and DecisionSnapshot are
// JSON; a zero CreatedAt or UpdatedAt takes the database default. The stored
// ID and timestamps are written back to request.
func InsertTestCareScheduleChangeRequest(tb testing.TB, db bun.IDB, request *CareScheduleChangeRequestRow) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.CareScheduleChangeRequest{
		StudentID:      request.StudentID,
		SubmittedBy:    request.SubmittedBy,
		RequestKind:    request.RequestKind,
		Status:         request.Status,
		DecisionReason: request.DecisionReason,
		ReviewedBy:     request.ReviewedBy,
		ReviewedAt:     request.ReviewedAt,
		AppliedAt:      request.AppliedAt,
	}
	require.NoError(tb, json.Unmarshal(request.Payload, &row.Payload), "care request payload must be a JSON object")
	if len(request.DecisionSnapshot) > 0 {
		row.DecisionSnapshot = &schedule.CareRequestDecisionSnapshot{}
		require.NoError(tb, json.Unmarshal(request.DecisionSnapshot, row.DecisionSnapshot), "care request decision snapshot")
	}
	row.ID = request.ID
	row.TenantID = request.TenantID
	row.CreatedAt = request.CreatedAt
	row.UpdatedAt = request.UpdatedAt
	_, err := db.NewInsert().Model(row).Exec(ctx)
	require.NoError(tb, err, "Failed to create test care-schedule change request")
	request.ID, request.CreatedAt, request.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
}

// GuardianProfileWithEmail describes a guardian profile whose email must be
// stored exactly as given, for suites that assert on email collisions. Empty
// PreferredContactMethod and LanguagePreference take the database defaults.
type GuardianProfileWithEmail struct {
	FirstName              string
	LastName               string
	Email                  string
	PreferredContactMethod string
	LanguagePreference     string
}

// CreateTestGuardianProfileWithExactEmail stores a guardian profile in
// tenantID without making the email unique, unlike
// CreateTestGuardianProfileForTenant, and returns the stored profile.
func CreateTestGuardianProfileWithExactEmail(tb testing.TB, db bun.IDB, tenantID int64, profile GuardianProfileWithEmail) *users.GuardianProfile {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := profile.Email
	row := &users.GuardianProfile{
		FirstName:              profile.FirstName,
		LastName:               profile.LastName,
		Email:                  &email,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).Exec(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")
	return row
}
