package enrollment

import (
	"context"
	"encoding/json"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type IntakeRequests interface {
	InsertRequest(context.Context, *capability.Request) error
	RequestByID(context.Context, int64, bool) (*capability.Request, error)
	RequestByToken(context.Context, string, bool) (*capability.Request, error)
	RequestsByID(context.Context, []int64) ([]*capability.Request, error)
	UpdateRequestGuardian(context.Context, *capability.Request, bool) error
	SetRequestWithdrawal(context.Context, int64, *time.Time) error
	AcquireSubmissionDedupLock(context.Context, int64, uint64) error
	AcquireExistingStudentMatchLock(context.Context, int64) error
	ActiveDuplicateChildren(context.Context, int64, string, []capability.DuplicateChildKey, int64) ([]capability.DuplicateChildKey, error)
	HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
	PinDecisionNotificationMode(context.Context, int64, string) (string, error)
}

type ReportRequests interface {
	AdminRequests(context.Context, capability.RequestListFilters) ([]*capability.Request, error)
}

type RequestIDReader interface {
	RequestByID(context.Context, int64, bool) (*capability.Request, error)
}

type RequestCreator interface {
	InsertRequest(context.Context, *capability.Request) error
}

type RequestBatchReader interface {
	RequestsByID(context.Context, []int64) ([]*capability.Request, error)
}

type RolloverRequests interface {
	RequestCreator
	RequestBatchReader
}

type OfferingRequestReader interface {
	RequestIDReader
	RequestBatchReader
}

type DecisionRequests interface {
	RequestIDReader
	ReportRequests
	SetRequestWithdrawal(context.Context, int64, *time.Time) error
	AcquireSubmissionDedupLock(context.Context, int64, uint64) error
	AcquireExistingStudentMatchLock(context.Context, int64) error
	ActiveDuplicateChildren(context.Context, int64, string, []capability.DuplicateChildKey, int64) ([]capability.DuplicateChildKey, error)
	HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
	PinDecisionNotificationMode(context.Context, int64, string) (string, error)
}

func listReportRequests(ctx context.Context, owner ReportRequests, filters capability.RequestListFilters) ([]*enrollmentModels.Request, error) {
	values, err := owner.AdminRequests(ctx, filters)
	if err != nil {
		return nil, err
	}
	return intakeRequestValues(values)
}

func createIntakeRequest(ctx context.Context, owner RequestCreator, req *enrollmentModels.Request) error {
	value, err := intakeRequestInput(req)
	if err != nil {
		return err
	}
	if err := owner.InsertRequest(ctx, value); err != nil {
		return err
	}
	result, err := intakeRequestValue(value)
	if err != nil {
		return err
	}
	*req = *result
	return nil
}
func updateIntakeRequest(ctx context.Context, owner IntakeRequests, req *enrollmentModels.Request, includeEmail bool) error {
	value, err := intakeRequestInput(req)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}
func intakeRequestByID(ctx context.Context, owner RequestIDReader, id int64, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByID(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	return intakeRequestValue(value)
}
func intakeRequestByToken(ctx context.Context, owner IntakeRequests, token string, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByToken(ctx, token, lock)
	if err != nil {
		return nil, err
	}
	return intakeRequestValue(value)
}
func intakeRequestsByID(ctx context.Context, owner RequestBatchReader, ids []int64) ([]*enrollmentModels.Request, error) {
	values, err := owner.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return intakeRequestValues(values)
}
func withdrawIntakeRequest(ctx context.Context, owner IntakeRequests, id int64, at time.Time) error {
	return owner.SetRequestWithdrawal(ctx, id, &at)
}
func intakeRequestInput(r *enrollmentModels.Request) (*capability.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &capability.Request{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID, PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID, SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires, SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode}
	var err error
	result.ConsentFlags, err = json.Marshal(r.ConsentFlags)
	if err != nil {
		return nil, err
	}
	result.LegalBlocksSnapshot, err = json.Marshal(r.LegalBlocksSnapshot)
	if err != nil {
		return nil, err
	}
	result.CustomData, err = json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.SourceMetadata, err = json.Marshal(r.SourceMetadata)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func intakeRequestValue(r *capability.Request) (*enrollmentModels.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollmentModels.Request{}
	result.ID = r.ID
	result.TenantID = r.TenantID
	result.CreatedAt = r.CreatedAt
	result.UpdatedAt = r.UpdatedAt
	result.SchemaID = r.SchemaID
	result.PhaseID = r.PhaseID
	result.GuardianFirstName = r.GuardianFirstName
	result.GuardianLastName = r.GuardianLastName
	result.GuardianEmail = r.GuardianEmail
	result.GuardianPhone = r.GuardianPhone
	result.GuardianAccountID = r.GuardianAccountID
	result.SubmissionSource = r.SubmissionSource
	result.StatusToken = r.StatusToken
	result.StatusTokenExpires = r.StatusTokenExpires
	result.SubmittedAt = r.SubmittedAt
	result.WithdrawnAt = r.WithdrawnAt
	result.DecisionNotificationMode = r.DecisionNotificationMode
	if len(r.ConsentFlags) > 0 {
		if err := json.Unmarshal(r.ConsentFlags, &result.ConsentFlags); err != nil {
			return nil, err
		}
	}
	if len(r.LegalBlocksSnapshot) > 0 {
		if err := json.Unmarshal(r.LegalBlocksSnapshot, &result.LegalBlocksSnapshot); err != nil {
			return nil, err
		}
	}
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	if len(r.SourceMetadata) > 0 {
		if err := json.Unmarshal(r.SourceMetadata, &result.SourceMetadata); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func intakeRequestValues(values []*capability.Request) ([]*enrollmentModels.Request, error) {
	var result []*enrollmentModels.Request
	for _, value := range values {
		converted, err := intakeRequestValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}
