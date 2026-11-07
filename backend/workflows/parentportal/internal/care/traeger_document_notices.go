package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The push „Neues Dokument vom Träger“ of a sent care contract and of a
// document a Träger released for parents (#5941). Both run the way the push
// of a letter of the Elternabrechnung runs (ADR 0126): the Träger's write
// puts one deferred push on Delivery's outbox, the worker calls the flow when
// it sends, and the flow decides who may hear of the document now, with the
// same relationship rules the inbox applies when it lists it:
//
//   - A contract reaches the account of the address it went to, only through
//     the relationship to the contract's own child, which must grant
//     parent_portal.enrollments.view. A right for a sibling opens nothing.
//   - A release reaches each account whose children's relationships grant
//     parent_portal.access at a school, and for a group release at the group,
//     the release covers. A parent who already sees the document through an
//     older active release hears nothing again.
//
// The contract, the release and the relationship are read again when the
// worker sends, so a revocation, a deletion, an ended contract or a removed
// right before then sends nothing. Every push goes through the same
// notifier, type and consent as the letters; its key names the kind, the
// document and the account, so a retried send pushes once.

// The kinds of a document push. The letters of the Elternabrechnung keep the
// empty kind they always had.
const (
	DocumentPushBilling  = ""
	DocumentPushContract = "care_contract"
	DocumentPushRelease  = "traeger_document_release"
)

// ContractNoticeTarget is the school and the child of a contract in the
// inbox of an address.
type ContractNoticeTarget struct {
	SchoolID  int64
	StudentID int64
}

// ContractInboxNotices reads, in the caller's Träger transaction, whether a
// contract still lies in the inbox of an address; it records no read.
type ContractInboxNotices interface {
	ContractNoticeTarget(ctx context.Context, organizationID, contractID int64, email string) (ContractNoticeTarget, bool, error)
}

// ReleaseNoticeAudience is whom an active release for parents reaches:
// SchoolID nil for every school of the Träger, GroupID a group of that
// school.
type ReleaseNoticeAudience struct {
	DocumentID int64
	SchoolID   *int64
	GroupID    *int64
}

// ReleaseNotices reads a release for its push in the caller's Träger
// transaction; neither operation records a read.
type ReleaseNotices interface {
	ActiveParentRelease(ctx context.Context, organizationID, releaseID int64) (ReleaseNoticeAudience, bool, error)
	ReleaseIsNewForParent(ctx context.Context, organizationID, releaseID int64, reader SharedDocumentReader) (bool, error)
}

// ReleaseSchools lists the undeleted schools of a Träger, outside any
// transaction, for a release to every school.
type ReleaseSchools interface {
	TraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
}

// ReleaseRecipients finds the accounts a release may reach at a school, in
// the tenant transaction of schoolID: those whose relationship to a child
// there, in groupID when set, grants parent_portal.access. Each candidate is
// checked again through its own relationships before a push.
type ReleaseRecipients interface {
	GuardianAccounts(ctx context.Context, schoolID int64, groupID *int64) ([]int64, error)
}

// TraegerDocumentNotifier sends the push of a contract or a release through
// the school's router, with the account's consent, in the caller's tenant
// transaction. sent tells whether a device push was enqueued there: a school
// with notifications switched off or without the Eltern-App enqueues none.
type TraegerDocumentNotifier interface {
	NotifyTraegerDocument(ctx context.Context, push BillingDocumentPush) (sent bool, err error)
}

// ReleaseFanOut puts one deferred push per candidate account of a release on
// Delivery's outbox, in the caller's Träger transaction. Each account then
// runs as its own outbox entry with its own retries, so a large audience
// never shares one send's time limit and a retry never repeats the accounts
// already done.
type ReleaseFanOut interface {
	EnqueueReleaseAccounts(ctx context.Context, organizationID, releaseID int64, accountIDs []int64) error
}

// NotifyContractDocument sends the push of a contract sent to recipientEmail.
// An address without an account, a contract no longer in the inbox, a child
// of another Träger or a relationship without the inbox right gets nothing.
func (s *Service) NotifyContractDocument(ctx context.Context, organizationID, contractID int64, recipientEmail string) error {
	if s.SchoolTraeger == nil || s.ParentAccounts == nil || s.TraegerDocumentNotifier == nil || s.TraegerRuntime == nil || s.ContractInboxNotices == nil {
		return errors.New("parent: contract notice is not configured")
	}
	email := strings.TrimSpace(recipientEmail)
	if organizationID <= 0 || contractID <= 0 || email == "" {
		return errors.New("parent: contract notice needs a Träger, a contract and an address")
	}
	accountID, access, err := s.documentRecipient(ctx, email)
	if err != nil {
		return fmt.Errorf("parent: resolve contract notice: %w", err)
	}
	if accountID <= 0 {
		return nil
	}
	var target ContractNoticeTarget
	var found bool
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var readErr error
		target, found, readErr = s.ContractInboxNotices.ContractNoticeTarget(txCtx, organizationID, contractID, email)
		return readErr
	})
	if err != nil {
		return fmt.Errorf("parent: read contract notice: %w", err)
	}
	if !found || access.schools[target.SchoolID].ID != organizationID {
		return nil
	}
	index := slices.IndexFunc(access.children[target.SchoolID], func(child *ChildSummary) bool { return child.StudentID == target.StudentID })
	if index < 0 {
		return nil
	}
	_, err = s.notifyDocumentAt(ctx, DocumentPushContract, target.SchoolID, accountID, contractID, access.children[target.SchoolID][index:index+1])
	return err
}

// NotifyTraegerDocumentRelease fans the push of a release for parents out
// to one outbox entry per candidate account at the schools the release
// names. A revoked release, a deleted document or a release for schools
// reaches nobody. Each entry is checked again when it runs
// (NotifyTraegerDocumentReleaseAccount).
func (s *Service) NotifyTraegerDocumentRelease(ctx context.Context, organizationID, releaseID int64) error {
	if err := s.releaseNoticeConfigured(organizationID, releaseID); err != nil {
		return err
	}
	if s.ReleaseFanOut == nil || s.ReleaseRecipients == nil {
		return errors.New("parent: release fan-out is not configured")
	}
	audience, found, err := s.activeRelease(ctx, organizationID, releaseID)
	if err != nil || !found {
		return err
	}
	schools, err := s.releaseSchools(ctx, organizationID, audience)
	if err != nil {
		return err
	}
	accounts, err := s.releaseCandidates(ctx, schools, audience.GroupID)
	if err != nil || len(accounts) == 0 {
		return err
	}
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		return s.ReleaseFanOut.EnqueueReleaseAccounts(txCtx, organizationID, releaseID, accounts)
	})
	if err != nil {
		return fmt.Errorf("parent: fan out release notice: %w", err)
	}
	return nil
}

// NotifyTraegerDocumentReleaseAccount sends the push of a release to one
// account, after reading the release and the account's relationships again.
func (s *Service) NotifyTraegerDocumentReleaseAccount(ctx context.Context, organizationID, releaseID, accountID int64) error {
	if err := s.releaseNoticeConfigured(organizationID, releaseID); err != nil {
		return err
	}
	if accountID <= 0 {
		return errors.New("parent: release notice needs an account")
	}
	audience, found, err := s.activeRelease(ctx, organizationID, releaseID)
	if err != nil || !found {
		return err
	}
	schools, err := s.releaseSchools(ctx, organizationID, audience)
	if err != nil {
		return err
	}
	return s.notifyReleaseTo(ctx, organizationID, releaseID, accountID, schools, audience.GroupID)
}

func (s *Service) releaseNoticeConfigured(organizationID, releaseID int64) error {
	if s.TraegerRuntime == nil || s.ReleaseNotices == nil || s.ReleaseSchools == nil || s.TraegerDocumentNotifier == nil ||
		s.SchoolTraeger == nil || s.StudentRepo == nil {
		return errors.New("parent: release notice is not configured")
	}
	if organizationID <= 0 || releaseID <= 0 {
		return errors.New("parent: release notice needs a Träger and a release")
	}
	return nil
}

func (s *Service) activeRelease(ctx context.Context, organizationID, releaseID int64) (ReleaseNoticeAudience, bool, error) {
	var audience ReleaseNoticeAudience
	var found bool
	err := s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var readErr error
		audience, found, readErr = s.ReleaseNotices.ActiveParentRelease(txCtx, organizationID, releaseID)
		return readErr
	})
	if err != nil {
		return ReleaseNoticeAudience{}, false, fmt.Errorf("parent: read release notice: %w", err)
	}
	return audience, found, nil
}

// releaseSchools are the schools the release names, all schools of the
// Träger for a release without a school, in ascending order.
func (s *Service) releaseSchools(ctx context.Context, organizationID int64, audience ReleaseNoticeAudience) ([]int64, error) {
	if audience.SchoolID != nil {
		return []int64{*audience.SchoolID}, nil
	}
	schools, err := s.ReleaseSchools.TraegerSchoolIDs(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent: list schools of the release: %w", err)
	}
	sorted := slices.Clone(schools)
	slices.Sort(sorted)
	return slices.Compact(sorted), nil
}

// releaseCandidates lists, once each and in ascending order, the accounts
// with a portal relationship at the release's schools.
func (s *Service) releaseCandidates(ctx context.Context, schools []int64, groupID *int64) ([]int64, error) {
	var accounts []int64
	for _, schoolID := range schools {
		err := s.Runtime.WithinTenant(ctx, schoolID, func(txCtx context.Context) error {
			found, listErr := s.ReleaseRecipients.GuardianAccounts(txCtx, schoolID, groupID)
			accounts = append(accounts, found...)
			return listErr
		})
		if err != nil {
			return nil, fmt.Errorf("parent: list parents of school %d for the release: %w", schoolID, err)
		}
	}
	slices.Sort(accounts)
	return slices.Compact(accounts), nil
}

// notifyReleaseTo checks one account through its own relationships and the
// release's coverage and sends it one push: at the first school, in
// ascending order, where the release reaches one of its children and the
// push can go out. A school with notifications switched off or without the
// Eltern-App passes the push on to the next one.
func (s *Service) notifyReleaseTo(ctx context.Context, organizationID, releaseID, accountID int64, schools []int64, groupID *int64) error {
	reached, reader, err := s.releaseChildren(ctx, organizationID, accountID, schools, groupID)
	if err != nil || len(reached) == 0 {
		return err
	}
	var isNew bool
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var checkErr error
		isNew, checkErr = s.ReleaseNotices.ReleaseIsNewForParent(txCtx, organizationID, releaseID, reader)
		return checkErr
	})
	if err != nil {
		return fmt.Errorf("parent: check release notice: %w", err)
	}
	if !isNew {
		return nil
	}
	for start := 0; start < len(reached); {
		schoolID := reached[start].TenantID
		end := start
		for end < len(reached) && reached[end].TenantID == schoolID {
			end++
		}
		sent, err := s.notifyDocumentAt(ctx, DocumentPushRelease, schoolID, accountID, releaseID, reached[start:end])
		if err != nil || sent {
			return err
		}
		start = end
	}
	return nil
}

// releaseChildren resolves, in one administrative transaction, the account's
// children whose relationship grants parent_portal.access at a school of the
// Träger, the reader the inbox would use for it, and those of them the
// release reaches, ordered by school.
func (s *Service) releaseChildren(ctx context.Context, organizationID, accountID int64, schools []int64, groupID *int64) ([]*ChildSummary, SharedDocumentReader, error) {
	var reached []*ChildSummary
	var reader SharedDocumentReader
	err := s.billingDocumentAccess(ctx, accountID, func(adminCtx context.Context, children []*ChildSummary) error {
		groups, err := s.permittedSchoolGroups(adminCtx, children)
		if err != nil || len(groups) == 0 {
			return err
		}
		access, err := s.sharedReaders(adminCtx, accountID, groups)
		if err != nil {
			return err
		}
		reader = access.readers[organizationID]
		reached, err = s.reachedChildren(adminCtx, children, reader, schools, groupID)
		return err
	})
	if err != nil {
		return nil, SharedDocumentReader{}, fmt.Errorf("parent: resolve release notice: %w", err)
	}
	return reached, reader, nil
}

// reachedChildren keeps the permitted children at a school of the reader the
// release names, in the group of a group release.
func (s *Service) reachedChildren(ctx context.Context, children []*ChildSummary, reader SharedDocumentReader, schools []int64, groupID *int64) ([]*ChildSummary, error) {
	var reached []*ChildSummary
	for _, child := range children {
		if child == nil || !childHasPermission(child, GuardianPermissionPortalAccess) || !slices.Contains(schools, child.TenantID) {
			continue
		}
		if _, ok := reader.Schools[child.TenantID]; !ok {
			continue
		}
		if groupID != nil {
			student, err := s.StudentRepo.FindByID(ctx, child.StudentID)
			if err != nil {
				return nil, fmt.Errorf("read the group of child %d: %w", child.StudentID, err)
			}
			if student == nil || student.GroupID == nil || *student.GroupID != *groupID {
				continue
			}
		}
		reached = append(reached, child)
	}
	slices.SortStableFunc(reached, func(a, b *ChildSummary) int { return int(a.TenantID - b.TenantID) })
	return reached, nil
}

// notifyDocumentAt sends the push of one document at one school in its
// tenant transaction, in the language the guardian chose there, and tells
// whether a device push was enqueued.
func (s *Service) notifyDocumentAt(ctx context.Context, kind string, schoolID, accountID, documentID int64, children []*ChildSummary) (bool, error) {
	studentIDs := make([]int64, len(children))
	for i, child := range children {
		studentIDs[i] = child.StudentID
	}
	var sent bool
	err := s.Runtime.WithinTenant(ctx, schoolID, func(txCtx context.Context) error {
		locale, err := s.guardianLocale(txCtx, children[0].GuardianProfileID)
		if err != nil {
			return err
		}
		sent, err = s.TraegerDocumentNotifier.NotifyTraegerDocument(txCtx, BillingDocumentPush{
			Kind: kind, TenantID: schoolID, AccountID: accountID, StudentIDs: studentIDs, DocumentID: documentID, Locale: locale,
		})
		return err
	})
	return sent, err
}
