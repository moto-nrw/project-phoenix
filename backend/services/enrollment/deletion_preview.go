package enrollment

import (
	"context"
	"fmt"
	"sort"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DeletionPreview combines owner queries without accessing persistence.
type DeletionPreview interface {
	PreviewRequest(context.Context, int64) (*enrollmentModels.DeletionImpact, error)
	PreviewChild(context.Context, int64, int64) (*enrollmentModels.DeletionImpact, error)
}

type DeletionQueries interface {
	DeletionRequestCounts(context.Context, int64) (*capability.DeletionRequestCounts, error)
	DeletionChildTarget(context.Context, int64, int64) (*capability.DeletionChildTarget, error)
	DeletionChildCounts(context.Context, int64, int64) (*capability.DeletionChildCounts, error)
	DeletionGuardianProfileIDs(context.Context, int64) ([]int64, error)
	DeletionBlockingStudentIDs(context.Context, int64, *int64) ([]int64, error)
}

type deletionPreview struct {
	enrollment            DeletionQueries
	countAuditAdjustments func(context.Context, int64, *int64) (int, error)
	guardians             GuardianDirectory
}

func NewDeletionPreview(
	enrollment DeletionQueries,
	guardians GuardianDirectory,
	countAuditAdjustments func(context.Context, int64, *int64) (int, error),
) DeletionPreview {
	return &deletionPreview{enrollment: enrollment, guardians: guardians, countAuditAdjustments: countAuditAdjustments}
}

// guardianPreservation is what a request deletion leaves behind on the
// guardian side: the profiles and parent accounts that survive, and how
// many of them no child links to any more.
type guardianPreservation struct {
	profiles                int
	accounts                int
	unlinkedProfiles        int
	accountsWithoutStudents int
}

// previewGuardianPreservation resolves the guardian side of the preview
// through the People Directory (#2663): the candidate profiles are the
// request's co-guardian rows plus the profiles of the request's account,
// the candidate accounts are the request's account plus the accounts of
// those profiles. Everything is scoped to the tenant in context.
func (r *deletionPreview) previewGuardianPreservation(ctx context.Context, requestID int64, guardianAccountID *int64) (guardianPreservation, error) {
	if r.guardians == nil {
		return guardianPreservation{}, errGuardianDirectoryRequired
	}
	requestProfileIDs, err := r.enrollment.DeletionGuardianProfileIDs(ctx, requestID)
	if err != nil {
		return guardianPreservation{}, err
	}
	profileIDs := make(map[int64]struct{}, len(requestProfileIDs))
	for _, id := range requestProfileIDs {
		profileIDs[id] = struct{}{}
	}
	accountIDs := make(map[int64]struct{})
	if guardianAccountID != nil {
		accountIDs[*guardianAccountID] = struct{}{}
		accountProfiles, err := r.guardians.ListGuardiansByAccount(ctx, []int64{*guardianAccountID})
		if err != nil {
			return guardianPreservation{}, err
		}
		for _, profile := range accountProfiles {
			profileIDs[profile.ID] = struct{}{}
		}
	}
	candidateProfiles := sortedIDs(profileIDs)
	profiles, err := r.guardians.ListGuardiansByID(ctx, candidateProfiles)
	if err != nil {
		return guardianPreservation{}, err
	}
	for _, profile := range profiles {
		if profile.AccountID != nil {
			accountIDs[*profile.AccountID] = struct{}{}
		}
	}
	candidateAccounts := sortedIDs(accountIDs)

	linkCounts, err := r.guardians.CountGuardianLinks(ctx, candidateProfiles)
	if err != nil {
		return guardianPreservation{}, err
	}
	result := guardianPreservation{profiles: len(candidateProfiles), accounts: len(candidateAccounts)}
	for _, id := range candidateProfiles {
		if linkCounts[id] == 0 {
			result.unlinkedProfiles++
		}
	}
	result.accountsWithoutStudents, err = r.countAccountsWithoutStudents(ctx, candidateAccounts)
	return result, err
}

// countAccountsWithoutStudents counts the accounts none of whose profiles
// still holds a child link. The profiles of an account may reach beyond the
// candidate profiles, so they are resolved from the account side.
func (r *deletionPreview) countAccountsWithoutStudents(ctx context.Context, accountIDs []int64) (int, error) {
	accountProfiles, err := r.guardians.ListGuardiansByAccount(ctx, accountIDs)
	if err != nil {
		return 0, err
	}
	profileIDs := make([]int64, 0, len(accountProfiles))
	for _, profile := range accountProfiles {
		profileIDs = append(profileIDs, profile.ID)
	}
	linkCounts, err := r.guardians.CountGuardianLinks(ctx, profileIDs)
	if err != nil {
		return 0, err
	}
	linkedAccounts := make(map[int64]struct{})
	for _, profile := range accountProfiles {
		if profile.AccountID != nil && linkCounts[profile.ID] > 0 {
			linkedAccounts[*profile.AccountID] = struct{}{}
		}
	}
	count := 0
	for _, id := range accountIDs {
		if _, linked := linkedAccounts[id]; !linked {
			count++
		}
	}
	return count, nil
}

// sortedIDs returns the set's members in ascending order.
func sortedIDs(set map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (r *deletionPreview) PreviewRequest(ctx context.Context, requestID int64) (*enrollmentModels.DeletionImpact, error) {
	_, err := deletionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.enrollment.DeletionRequestCounts(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if r.countAuditAdjustments == nil {
		return nil, fmt.Errorf("preview enrollment request deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.countAuditAdjustments(ctx, requestID, nil)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	preserved, err := r.previewGuardianPreservation(ctx, requestID, row.GuardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	impact := &enrollmentModels.DeletionImpact{
		RequestID:                     requestID,
		DeletesRequest:                true,
		Counts:                        deletionCountsFromRow(row),
		PreservedGuardianProfiles:     preserved.profiles,
		PreservedParentAccounts:       preserved.accounts,
		UnlinkedGuardianProfiles:      preserved.unlinkedProfiles,
		ParentAccountsWithoutStudents: preserved.accountsWithoutStudents,
	}
	if row.Requests == 0 {
		return impact, nil
	}
	impact.BlockingStudentIDs, err = r.enrollment.DeletionBlockingStudentIDs(ctx, requestID, nil)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func (r *deletionPreview) PreviewChild(ctx context.Context, requestID, childID int64) (*enrollmentModels.DeletionImpact, error) {
	_, err := deletionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta, err := r.enrollment.DeletionChildTarget(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if meta.TargetChildren == 0 {
		return &enrollmentModels.DeletionImpact{RequestID: requestID, ChildID: &childID}, nil
	}
	if meta.AllChildren == 1 {
		impact, previewErr := r.PreviewRequest(ctx, requestID)
		if previewErr != nil {
			return nil, previewErr
		}
		impact.ChildID = &childID
		return impact, nil
	}

	row, err := r.enrollment.DeletionChildCounts(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if r.countAuditAdjustments == nil {
		return nil, fmt.Errorf("preview enrollment child deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.countAuditAdjustments(ctx, requestID, &childID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	impact := &enrollmentModels.DeletionImpact{
		RequestID: requestID,
		ChildID:   &childID,
		Counts: enrollmentModels.DeletionCounts{
			RequestChildren:           1,
			RequestChildOfferings:     row.Offerings,
			ChangeRequests:            row.ChangeRequests,
			ChangeRequestMessages:     row.ChangeRequestMessages,
			OfferingAdjustments:       row.OfferingAdjustments,
			RolloverLinksCleared:      row.RolloverLinks,
			StudentSourceLinksCleared: row.StudentSourceLinks,
		},
	}
	impact.BlockingStudentIDs, err = r.enrollment.DeletionBlockingStudentIDs(ctx, requestID, &childID)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func deletionTenantID(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, fmt.Errorf("tenant context is required for enrollment deletion")
	}
	return tenantID, nil
}

func deletionCountsFromRow(row *capability.DeletionRequestCounts) enrollmentModels.DeletionCounts {
	return enrollmentModels.DeletionCounts{
		Requests:                  row.Requests,
		RequestChildren:           row.RequestChildren,
		RequestChildOfferings:     row.RequestChildOfferings,
		RequestGuardians:          row.RequestGuardians,
		ChangeRequests:            row.ChangeRequests,
		ChangeRequestMessages:     row.ChangeRequestMessages,
		LateInvites:               row.LateInvites,
		OfferingAdjustments:       row.OfferingAdjustments,
		EmailOutbox:               row.EmailOutbox,
		RolloverLinksCleared:      row.RolloverLinksCleared,
		StudentSourceLinksCleared: row.StudentSourceLinksCleared,
	}
}
