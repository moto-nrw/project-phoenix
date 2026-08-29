package parent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	RequestShareMasterData   = "master_data"
	RequestShareCareSchedule = "care_schedule"
	RequestSharePickupChange = "pickup_change"
	RequestShareOffering     = "offering"
	RequestShareExcused      = "excused"
)

var (
	ErrRequestSharingInvalid   = errors.New("parent: invalid request sharing choice")
	ErrRequestSharingForbidden = errors.New("parent: request sharing is forbidden")
	ErrRequestSharingNotFound  = errors.New("parent: request not found")
)

type RequestSharingRecipient struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	Selected          bool
	accountID         int64
}

type RequestSharingState struct {
	FamilyProtected bool
	Recipients      []RequestSharingRecipient
}

type RequestSharingService interface {
	GetRequestSharingOptions(context.Context, int64, int64) (*RequestSharingState, error)
	GetRequestSharing(context.Context, int64, int64, string, int64) (*RequestSharingState, error)
	SetRequestSharing(context.Context, int64, int64, string, int64, []int64) (*RequestSharingState, error)
}

func (s *service) GetRequestSharingOptions(
	ctx context.Context, accountID, studentID int64,
) (*RequestSharingState, error) {
	if !s.requestSharingConfigured() {
		return nil, errors.New("parent: request sharing service is not configured")
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var state *RequestSharingState
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		protected, _, loadErr := s.currentFamilyProtection(txCtx, studentID)
		if loadErr != nil {
			return loadErr
		}
		recipients, _, loadErr := s.shareRecipients(txCtx, accountID, studentID, nil, false)
		state = &RequestSharingState{FamilyProtected: protected, Recipients: recipients}
		return loadErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: get request sharing options: %w", err)
	}
	return state, nil
}

type requestShareKey struct {
	typ string
	id  int64
}

func (s *service) GetRequestSharing(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
) (*RequestSharingState, error) {
	return s.changeRequestSharing(ctx, accountID, studentID, requestType, requestID, nil, false)
}

func (s *service) SetRequestSharing(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
	recipientGuardianProfileIDs []int64,
) (*RequestSharingState, error) {
	return s.changeRequestSharing(ctx, accountID, studentID, requestType, requestID, recipientGuardianProfileIDs, true)
}

func (s *service) changeRequestSharing(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
	recipientProfileIDs []int64, write bool,
) (*RequestSharingState, error) {
	if !s.requestSharingConfigured() {
		return nil, errors.New("parent: request sharing service is not configured")
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var state *RequestSharingState
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var changeErr error
		state, changeErr = s.changeRequestSharingInTx(txCtx, accountID, studentID, requestType, requestID, recipientProfileIDs, write)
		return changeErr
	})
	if err != nil {
		return nil, fmt.Errorf("parent: change request sharing: %w", err)
	}
	return state, nil
}

func (s *service) requestSharingConfigured() bool {
	return s.ParentRequestShares != nil && s.FamilyProtectionEvents != nil && s.StudentRepo != nil &&
		s.StudentGuardianRepo != nil && s.GuardianProfileRepo != nil
}

func (s *service) changeRequestSharingInTx(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
	profileIDs []int64, write bool,
) (*RequestSharingState, error) {
	if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
		return nil, err
	}
	if err := s.requireOwnedShareableRequest(ctx, accountID, studentID, requestType, requestID); err != nil {
		return nil, err
	}
	protected, protectedAt, err := s.currentFamilyProtection(ctx, studentID)
	if err != nil {
		return nil, err
	}
	recipients, accountIDs, err := s.shareRecipients(ctx, accountID, studentID, profileIDs, write)
	if err != nil {
		return nil, err
	}
	if write {
		if protected && len(accountIDs) > 0 {
			return nil, ErrRequestSharingForbidden
		}
		if err := s.createRequestShare(ctx, accountID, studentID, requestType, requestID, accountIDs); err != nil {
			return nil, err
		}
	}
	return s.requestSharingState(ctx, studentID, requestType, requestID, protected, protectedAt, recipients)
}

func (s *service) createRequestShare(ctx context.Context, accountID, studentID int64, requestType string, requestID int64, recipients []int64) error {
	event := &userModels.ParentRequestShareEvent{
		StudentID: studentID, RequestType: requestType, RequestID: requestID,
		AuthorAccountID: accountID, RecipientAccountIDs: recipients,
	}
	return s.ParentRequestShares.Create(ctx, event)
}

func (s *service) requestSharingState(
	ctx context.Context, studentID int64, requestType string, requestID int64,
	protected bool, protectedAt time.Time, recipients []RequestSharingRecipient,
) (*RequestSharingState, error) {
	current, err := s.currentRequestShares(ctx, studentID)
	if err != nil {
		return nil, err
	}
	selected := effectiveRecipientSet(current[requestShareKey{typ: requestType, id: requestID}], protected, protectedAt)
	for i := range recipients {
		recipients[i].Selected = selected[recipients[i].accountID]
	}
	return &RequestSharingState{FamilyProtected: protected, Recipients: recipients}, nil
}

func effectiveRecipientSet(event *userModels.ParentRequestShareEvent, protected bool, protectedAt time.Time) map[int64]bool {
	selected := map[int64]bool{}
	if event == nil || protected || (!protectedAt.IsZero() && !event.CreatedAt.After(protectedAt)) {
		return selected
	}
	for _, accountID := range event.RecipientAccountIDs {
		selected[accountID] = true
	}
	return selected
}

func (s *service) currentRequestShares(ctx context.Context, studentID int64) (map[requestShareKey]*userModels.ParentRequestShareEvent, error) {
	if s.ParentRequestShares == nil {
		return map[requestShareKey]*userModels.ParentRequestShareEvent{}, nil
	}
	rows, err := s.ParentRequestShares.CurrentForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make(map[requestShareKey]*userModels.ParentRequestShareEvent, len(rows))
	for _, row := range rows {
		result[requestShareKey{typ: row.RequestType, id: row.RequestID}] = row
	}
	return result, nil
}

func (s *service) currentFamilyProtection(ctx context.Context, studentID int64) (bool, time.Time, error) {
	if s.FamilyProtectionEvents == nil {
		return false, time.Time{}, nil
	}
	current, err := s.FamilyProtectionEvents.CurrentForStudents(ctx, []int64{studentID})
	if err != nil {
		return false, time.Time{}, err
	}
	event := current[studentID]
	if event == nil {
		return false, time.Time{}, nil
	}
	return event.Enabled, event.CreatedAt, nil
}

func (s *service) shareRecipients(
	ctx context.Context, authorAccountID, studentID int64, selectedProfileIDs []int64, validate bool,
) ([]RequestSharingRecipient, []int64, error) {
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	profiles := portalGuardianProfileIDs(links)
	profileRows, err := s.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, profiles)
	if err != nil {
		return nil, nil, err
	}
	selected, err := selectedGuardianProfiles(selectedProfileIDs)
	if err != nil {
		return nil, nil, err
	}
	recipients, accountIDs := buildSharingRecipients(profileRows, authorAccountID, selected)
	if validate && len(selected) > 0 {
		return nil, nil, ErrRequestSharingInvalid
	}
	sortSharingRecipients(recipients, accountIDs)
	return recipients, accountIDs, nil
}

func portalGuardianProfileIDs(links []*userModels.StudentGuardian) []int64 {
	profiles := make([]int64, 0, len(links))
	for _, link := range links {
		if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
			profiles = append(profiles, link.GuardianProfileID)
		}
	}
	return profiles
}

func selectedGuardianProfiles(profileIDs []int64) (map[int64]struct{}, error) {
	selected := make(map[int64]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		if id <= 0 {
			return nil, ErrRequestSharingInvalid
		}
		if _, duplicate := selected[id]; duplicate {
			return nil, ErrRequestSharingInvalid
		}
		selected[id] = struct{}{}
	}
	return selected, nil
}

func buildSharingRecipients(
	profiles map[int64]*userModels.GuardianProfile, authorAccountID int64, selected map[int64]struct{},
) ([]RequestSharingRecipient, []int64) {
	recipients := make([]RequestSharingRecipient, 0, len(profiles))
	accountIDs := make([]int64, 0, len(selected))
	for profileID, profile := range profiles {
		if profile == nil || profile.AccountID == nil || *profile.AccountID == authorAccountID {
			continue
		}
		recipients = append(recipients, RequestSharingRecipient{
			GuardianProfileID: profileID, FirstName: profile.FirstName, LastName: profile.LastName,
			accountID: *profile.AccountID,
		})
		if _, ok := selected[profileID]; ok {
			accountIDs = append(accountIDs, *profile.AccountID)
			delete(selected, profileID)
		}
	}
	return recipients, accountIDs
}

func sortSharingRecipients(recipients []RequestSharingRecipient, accountIDs []int64) {
	sort.Slice(recipients, func(i, j int) bool {
		return strings.ToLower(recipients[i].LastName+recipients[i].FirstName) < strings.ToLower(recipients[j].LastName+recipients[j].FirstName)
	})
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
}

func (s *service) requireOwnedShareableRequest(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
) error {
	if requestID <= 0 {
		return ErrRequestSharingInvalid
	}
	owner, child, kind, err := s.shareableRequestIdentity(ctx, requestType, requestID)
	if err != nil {
		return err
	}
	if owner != accountID || child != studentID || kind != requestType {
		return ErrRequestSharingNotFound
	}
	return nil
}

func (s *service) shareableRequestIdentity(ctx context.Context, requestType string, requestID int64) (int64, int64, string, error) {
	switch requestType {
	case RequestShareMasterData:
		return s.masterDataShareIdentity(ctx, requestID)
	case RequestShareCareSchedule, RequestSharePickupChange:
		return s.careRequestShareIdentity(ctx, requestType, requestID)
	case RequestShareOffering:
		return s.offeringShareIdentity(ctx, requestID)
	case RequestShareExcused:
		return s.excusedShareIdentity(ctx, requestID)
	default:
		return 0, 0, "", ErrRequestSharingInvalid
	}
}

func (s *service) masterDataShareIdentity(ctx context.Context, requestID int64) (int64, int64, string, error) {
	row, err := s.ChangeRequestRepo.FindByID(ctx, requestID)
	if err != nil || row == nil {
		return 0, 0, "", requestLookupError(err)
	}
	return row.SubmittedBy, row.StudentID, RequestShareMasterData, nil
}

func (s *service) careRequestShareIdentity(ctx context.Context, requestType string, requestID int64) (int64, int64, string, error) {
	row, err := s.CareRequestRepo.FindByID(ctx, requestID)
	if err != nil || row == nil {
		return 0, 0, "", requestLookupError(err)
	}
	expected := scheduleModels.CareRequestKindWeeklySchedule
	if requestType == RequestSharePickupChange {
		expected = scheduleModels.CareRequestKindPickupChange
	}
	if row.RequestKind != expected {
		return 0, 0, "", ErrRequestSharingNotFound
	}
	return row.SubmittedBy, row.StudentID, requestType, nil
}

func (s *service) offeringShareIdentity(ctx context.Context, requestID int64) (int64, int64, string, error) {
	row, err := s.OfferingChangeRequestRepo.FindByID(ctx, requestID)
	if err != nil || row == nil {
		return 0, 0, "", requestLookupError(err)
	}
	return row.SubmittedBy, row.StudentID, RequestShareOffering, nil
}

func (s *service) excusedShareIdentity(ctx context.Context, requestID int64) (int64, int64, string, error) {
	row, err := s.ExcusedRequestRepo.FindByID(ctx, requestID)
	if err != nil || row == nil {
		return 0, 0, "", requestLookupError(err)
	}
	return row.SubmittedBy, row.StudentID, RequestShareExcused, nil
}

func requestLookupError(err error) error {
	if err != nil {
		return err
	}
	return ErrRequestSharingNotFound
}

type requestShareVisibility struct {
	current     map[requestShareKey]*userModels.ParentRequestShareEvent
	protected   bool
	protectedAt time.Time
}

func (s *service) loadRequestShareVisibility(ctx context.Context, studentID int64) (*requestShareVisibility, error) {
	current, err := s.currentRequestShares(ctx, studentID)
	if err != nil {
		return nil, err
	}
	protected, protectedAt, err := s.currentFamilyProtection(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &requestShareVisibility{current: current, protected: protected, protectedAt: protectedAt}, nil
}

func (v *requestShareVisibility) allows(requestType string, requestID, accountID, submittedBy int64) bool {
	if submittedBy == accountID {
		return true
	}
	event := v.current[requestShareKey{typ: requestType, id: requestID}]
	return effectiveRecipientSet(event, v.protected, v.protectedAt)[accountID]
}
