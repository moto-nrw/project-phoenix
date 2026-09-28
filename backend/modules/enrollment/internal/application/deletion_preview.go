package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// DeletionQueries are the owner reads of the deletion preview.
type DeletionQueries interface {
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error)
	DeletionRequestCounts(context.Context, int64) (*enrollment.DeletionRequestCounts, error)
	DeletionChildTarget(context.Context, int64, int64) (*enrollment.DeletionChildTarget, error)
	DeletionChildCounts(context.Context, int64, int64) (*enrollment.DeletionChildCounts, error)
	DeletionGuardianProfileIDs(context.Context, int64) ([]int64, error)
	DeletionBlockingStudentIDs(context.Context, int64, *int64) ([]int64, error)
}

// DirectoryGuardian is the People Directory projection the deletion preview
// reads: users.guardian_profiles belongs to that owner (#2663), so the
// profiles of a request's account and their link counts are resolved through
// it instead of a foreign join.
type DirectoryGuardian struct {
	ID        int64
	AccountID *int64
}

// GuardianDirectory supplies tenant-scoped People Directory facts.
type GuardianDirectory interface {
	// ListGuardiansByAccount returns the tenant's profiles linked to the
	// accounts.
	ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]DirectoryGuardian, error)
	// ListGuardiansByID returns the tenant's profiles for the ids.
	ListGuardiansByID(ctx context.Context, ids []int64) ([]DirectoryGuardian, error)
	// CountGuardianLinks counts the tenant's links per profile; profiles
	// without a link are absent.
	CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error)
}

var errGuardianDirectoryRequired = errors.New("enrollment repositories: guardian directory is not bound")

// DeletionPreviewDependencies bind the deletion preview to its owners.
type DeletionPreviewDependencies struct {
	Enrollment DeletionQueries
	Guardians  GuardianDirectory
	// CountAuditAdjustments counts the audited offering adjustments of a
	// request or one of its children.
	CountAuditAdjustments func(context.Context, int64, *int64) (int, error)
	// CountBookings counts Care Plan's bookings of the request children.
	CountBookings func(context.Context, []int64) (int, error)
	Runtime       Runtime
}

// DeletionPreview combines owner queries into the impact of deleting a
// request or one of its children, without accessing persistence itself.
type DeletionPreview struct {
	deps DeletionPreviewDependencies
}

// NewDeletionPreview composes the deletion preview.
func NewDeletionPreview(deps DeletionPreviewDependencies) *DeletionPreview {
	return &DeletionPreview{deps: deps}
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
func (r *DeletionPreview) previewGuardianPreservation(ctx context.Context, requestID int64, guardianAccountID *int64) (guardianPreservation, error) {
	guardians := r.deps.Guardians
	if guardians == nil {
		return guardianPreservation{}, errGuardianDirectoryRequired
	}
	candidateProfiles, err := r.candidateGuardianProfiles(ctx, requestID, guardianAccountID)
	if err != nil {
		return guardianPreservation{}, err
	}
	candidateAccounts, err := r.candidateGuardianAccounts(ctx, candidateProfiles, guardianAccountID)
	if err != nil {
		return guardianPreservation{}, err
	}
	linkCounts, err := guardians.CountGuardianLinks(ctx, candidateProfiles)
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

// candidateGuardianProfiles are the request's co-guardian profiles plus the
// profiles of the request's account, sorted.
func (r *DeletionPreview) candidateGuardianProfiles(ctx context.Context, requestID int64, guardianAccountID *int64) ([]int64, error) {
	requestProfileIDs, err := r.deps.Enrollment.DeletionGuardianProfileIDs(ctx, requestID)
	if err != nil {
		return nil, err
	}
	profileIDs := make(map[int64]struct{}, len(requestProfileIDs))
	for _, id := range requestProfileIDs {
		profileIDs[id] = struct{}{}
	}
	if guardianAccountID != nil {
		accountProfiles, err := r.deps.Guardians.ListGuardiansByAccount(ctx, []int64{*guardianAccountID})
		if err != nil {
			return nil, err
		}
		for _, profile := range accountProfiles {
			profileIDs[profile.ID] = struct{}{}
		}
	}
	return sortedIDs(profileIDs), nil
}

// candidateGuardianAccounts are the request's account plus the accounts of
// the candidate profiles, sorted.
func (r *DeletionPreview) candidateGuardianAccounts(ctx context.Context, candidateProfiles []int64, guardianAccountID *int64) ([]int64, error) {
	accountIDs := make(map[int64]struct{})
	if guardianAccountID != nil {
		accountIDs[*guardianAccountID] = struct{}{}
	}
	profiles, err := r.deps.Guardians.ListGuardiansByID(ctx, candidateProfiles)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile.AccountID != nil {
			accountIDs[*profile.AccountID] = struct{}{}
		}
	}
	return sortedIDs(accountIDs), nil
}

// countAccountsWithoutStudents counts the accounts none of whose profiles
// still holds a child link. The profiles of an account may reach beyond the
// candidate profiles, so they are resolved from the account side.
func (r *DeletionPreview) countAccountsWithoutStudents(ctx context.Context, accountIDs []int64) (int, error) {
	accountProfiles, err := r.deps.Guardians.ListGuardiansByAccount(ctx, accountIDs)
	if err != nil {
		return 0, err
	}
	profileIDs := make([]int64, 0, len(accountProfiles))
	for _, profile := range accountProfiles {
		profileIDs = append(profileIDs, profile.ID)
	}
	linkCounts, err := r.deps.Guardians.CountGuardianLinks(ctx, profileIDs)
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

func (r *DeletionPreview) bookingCount(ctx context.Context, ids []int64) (int, error) {
	if r.deps.CountBookings == nil {
		return 0, fmt.Errorf("deletion preview requires Care Plan booking counts")
	}
	return r.deps.CountBookings(ctx, ids)
}

func (r *DeletionPreview) requireTenant(ctx context.Context) error {
	if r.deps.Runtime.TenantID(ctx) <= 0 {
		return fmt.Errorf("tenant context is required for enrollment deletion")
	}
	return nil
}

// PreviewRequest counts every row a deletion of the request affects.
func (r *DeletionPreview) PreviewRequest(ctx context.Context, requestID int64) (*enrollment.DeletionImpact, error) {
	if err := r.requireTenant(ctx); err != nil {
		return nil, err
	}
	row, err := r.requestCounts(ctx, requestID)
	if err != nil {
		return nil, err
	}
	preserved, err := r.previewGuardianPreservation(ctx, requestID, row.GuardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	impact := &enrollment.DeletionImpact{
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
	impact.BlockingStudentIDs, err = r.deps.Enrollment.DeletionBlockingStudentIDs(ctx, requestID, nil)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

// requestCounts reads the owner's counts of a request and adds the Care
// Plan bookings and audited adjustments of its children.
func (r *DeletionPreview) requestCounts(ctx context.Context, requestID int64) (*enrollment.DeletionRequestCounts, error) {
	row, err := r.deps.Enrollment.DeletionRequestCounts(ctx, requestID)
	if err != nil {
		return nil, err
	}
	children, err := r.deps.Enrollment.ChildrenForRequest(ctx, requestID, false)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	row.RequestChildOfferings, err = r.bookingCount(ctx, ids)
	if err != nil {
		return nil, err
	}
	if r.deps.CountAuditAdjustments == nil {
		return nil, fmt.Errorf("preview enrollment request deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.deps.CountAuditAdjustments(ctx, requestID, nil)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	return row, nil
}

// PreviewChild counts every row a deletion of one child affects; deleting
// the request's only child deletes the request.
func (r *DeletionPreview) PreviewChild(ctx context.Context, requestID, childID int64) (*enrollment.DeletionImpact, error) {
	if err := r.requireTenant(ctx); err != nil {
		return nil, err
	}
	meta, err := r.deps.Enrollment.DeletionChildTarget(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if meta.TargetChildren == 0 {
		return &enrollment.DeletionImpact{RequestID: requestID, ChildID: &childID}, nil
	}
	if meta.AllChildren == 1 {
		impact, previewErr := r.PreviewRequest(ctx, requestID)
		if previewErr != nil {
			return nil, previewErr
		}
		impact.ChildID = &childID
		return impact, nil
	}
	counts, err := r.childCounts(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	impact := &enrollment.DeletionImpact{RequestID: requestID, ChildID: &childID, Counts: counts}
	impact.BlockingStudentIDs, err = r.deps.Enrollment.DeletionBlockingStudentIDs(ctx, requestID, &childID)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func (r *DeletionPreview) childCounts(ctx context.Context, requestID, childID int64) (enrollment.DeletionCounts, error) {
	row, err := r.deps.Enrollment.DeletionChildCounts(ctx, requestID, childID)
	if err != nil {
		return enrollment.DeletionCounts{}, err
	}
	row.Offerings, err = r.bookingCount(ctx, []int64{childID})
	if err != nil {
		return enrollment.DeletionCounts{}, err
	}
	if r.deps.CountAuditAdjustments == nil {
		return enrollment.DeletionCounts{}, fmt.Errorf("preview enrollment child deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.deps.CountAuditAdjustments(ctx, requestID, &childID)
	if err != nil {
		return enrollment.DeletionCounts{}, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	return enrollment.DeletionCounts{
		RequestChildren:           1,
		RequestChildOfferings:     row.Offerings,
		ChangeRequests:            row.ChangeRequests,
		ChangeRequestMessages:     row.ChangeRequestMessages,
		OfferingAdjustments:       row.OfferingAdjustments,
		RolloverLinksCleared:      row.RolloverLinks,
		StudentSourceLinksCleared: row.StudentSourceLinks,
	}, nil
}

func deletionCountsFromRow(row *enrollment.DeletionRequestCounts) enrollment.DeletionCounts {
	return enrollment.DeletionCounts{
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
