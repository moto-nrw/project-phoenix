package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

	row := &masterDataChangeRequestInsert{
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

	row := &careScheduleChangeRequestInsert{
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
		var snapshot careRequestDecisionSnapshot
		require.NoError(tb, json.Unmarshal(request.DecisionSnapshot, &snapshot), "care request decision snapshot")
		encoded, err := json.Marshal(snapshot)
		require.NoError(tb, err, "care request decision snapshot")
		row.DecisionSnapshot = encoded
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
func CreateTestGuardianProfileWithExactEmail(tb testing.TB, db bun.IDB, tenantID int64, profile GuardianProfileWithEmail) *GuardianProfile {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := profile.Email
	row := &GuardianProfile{
		FirstName:              profile.FirstName,
		LastName:               profile.LastName,
		Email:                  &email,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
	}
	row.TenantID = tenantID
	_, err := db.NewInsert().Model(row).Exec(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")
	return row
}

// masterDataChangeRequestInsert is the users.student_data_change_requests row
// InsertTestMasterDataChangeRequest writes.
type masterDataChangeRequestInsert struct {
	bun.BaseModel `bun:"table:users.student_data_change_requests,alias:student_data_change_request"`
	ID            int64           `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	StudentID     int64           `bun:"student_id,notnull"`
	SubmittedBy   int64           `bun:"submitted_by,notnull"`
	Target        string          `bun:"target,notnull"`
	TargetRefID   *int64          `bun:"target_ref_id"`
	FieldKey      string          `bun:"field_key,notnull"`
	OldValue      json.RawMessage `bun:"old_value,type:jsonb"`
	NewValue      json.RawMessage `bun:"new_value,type:jsonb,notnull"`
	Status        string          `bun:"status,notnull,default:'pending'"`
	ReviewReason  *string         `bun:"review_reason"`
	ReviewedBy    *int64          `bun:"reviewed_by"`
	ReviewedAt    *time.Time      `bun:"reviewed_at"`
	AppliedAt     *time.Time      `bun:"applied_at"`
}

// careScheduleChangeRequestInsert is the schedule.care_schedule_change_requests
// row InsertTestCareScheduleChangeRequest writes.
type careScheduleChangeRequestInsert struct {
	bun.BaseModel    `bun:"table:schedule.care_schedule_change_requests,alias:care_schedule_change_request"`
	ID               int64           `bun:"id,pk,autoincrement"`
	CreatedAt        time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID         int64           `bun:"tenant_id,notnull"`
	StudentID        int64           `bun:"student_id,notnull"`
	SubmittedBy      int64           `bun:"submitted_by,notnull"`
	RequestKind      string          `bun:"request_kind,notnull,default:'weekly_schedule'"`
	Payload          map[string]any  `bun:"payload,type:jsonb,notnull"`
	Status           string          `bun:"status,notnull,default:'pending'"`
	DecisionReason   *string         `bun:"decision_reason"`
	ReviewedBy       *int64          `bun:"reviewed_by"`
	ReviewedAt       *time.Time      `bun:"reviewed_at"`
	AppliedAt        *time.Time      `bun:"applied_at"`
	DecisionSnapshot json.RawMessage `bun:"decision_snapshot,type:jsonb,nullzero"`
}

// careRequestDecisionSnapshot is the stored shape of a care request's
// decision snapshot; decoding into it validates and normalizes the arranged
// JSON the way the owner model does.
type careRequestDecisionSnapshot struct {
	Diff []struct {
		Label    string   `json:"label"`
		Old      string   `json:"old,omitempty"`
		New      string   `json:"new,omitempty"`
		Weekday  int      `json:"weekday,omitempty"`
		CareKind string   `json:"care_kind,omitempty"`
		OldModes []string `json:"old_modes,omitempty"`
		NewMode  string   `json:"new_mode,omitempty"`
	} `json:"diff"`
}
