package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Documents a Träger released for parents (#4692, spec #4611 user story
// 98). The Träger releases a document for the parents of all its schools, of
// one school or of one group; the account finds it in the document inbox.
// The relationship decides, never membership alone:
//
//   - Only children whose relationship grants parent_portal.access count.
//     Their schools and, per school, their groups are what a release must
//     cover. A bare guardian link or a school membership opens nothing.
//   - A release for all schools reaches the account only at a Träger where
//     such a child goes to school.
//
// The documents belong to Organisation & Tenancy; the flow reads them in the
// Träger's transaction through TraegerSharedDocuments, which records every
// opening with the account. A revoked release ends the access at once.

// ErrSharedDocumentNotFound reports a document no active release covers for
// the account.
var ErrSharedDocumentNotFound = errors.New("parent: shared document not found")

// SharedDocument is a document a Träger released for the account.
type SharedDocument struct {
	ID             int64
	OrganizationID int64
	TraegerName    string
	Title          string
	FileName       string
	ContentType    string
	SizeBytes      int
	ReleasedAt     time.Time
}

// SharedDocumentReader is the account with the schools of its permitted
// children at one Träger and, per school, the groups of those children.
type SharedDocumentReader struct {
	AccountID int64
	Schools   map[int64][]int64
}

// TraegerSharedDocuments is the port to the shared documents of one Träger.
// Both operations join the caller's Träger transaction of organizationID.
type TraegerSharedDocuments interface {
	ListSharedDocuments(ctx context.Context, organizationID int64, reader SharedDocumentReader) ([]SharedDocument, error)
	SharedDocumentFile(ctx context.Context, organizationID, documentID int64, reader SharedDocumentReader) (BillingDocumentFile, error)
}

// sharedAccess is, per Träger, what the account's relationships open.
type sharedAccess struct {
	traeger []Traeger
	readers map[int64]SharedDocumentReader
}

// ListSharedDocuments lists the documents the Träger of the account's
// children released for it, the newest release first. An account whose
// relationships grant the portal at no school gets ErrGuardianPermissionDenied.
func (s *Service) ListSharedDocuments(ctx context.Context, accountID int64) ([]SharedDocument, error) {
	access, err := s.sharedDocumentAccess(ctx, accountID)
	if err != nil {
		return nil, err
	}
	documents := []SharedDocument{}
	for _, organization := range access.traeger {
		err := s.TraegerRuntime.WithinTraeger(ctx, organization.ID, func(txCtx context.Context) error {
			found, listErr := s.SharedDocuments.ListSharedDocuments(txCtx, organization.ID, access.readers[organization.ID])
			for _, document := range found {
				document.OrganizationID, document.TraegerName = organization.ID, organization.Name
				documents = append(documents, document)
			}
			return listErr
		})
		if err != nil {
			return nil, fmt.Errorf("parent: list shared documents: %w", err)
		}
	}
	slices.SortStableFunc(documents, func(a, b SharedDocument) int {
		if order := b.ReleasedAt.Compare(a.ReleasedAt); order != 0 {
			return order
		}
		return int(b.ID - a.ID)
	})
	return documents, nil
}

// SharedDocumentFile reads the file of one document the Träger released for
// the account and records the opening.
func (s *Service) SharedDocumentFile(ctx context.Context, accountID, organizationID, documentID int64) (BillingDocumentFile, error) {
	access, err := s.sharedDocumentAccess(ctx, accountID)
	if err != nil {
		return BillingDocumentFile{}, err
	}
	reader, ok := access.readers[organizationID]
	if !ok || documentID <= 0 {
		return BillingDocumentFile{}, ErrSharedDocumentNotFound
	}
	var file BillingDocumentFile
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var readErr error
		file, readErr = s.SharedDocuments.SharedDocumentFile(txCtx, organizationID, documentID, reader)
		return readErr
	})
	return file, err
}

// sharedDocumentAccess resolves, in one administrative transaction, the
// children whose relationship grants parent_portal.access, their groups and
// the Träger of their schools.
func (s *Service) sharedDocumentAccess(ctx context.Context, accountID int64) (sharedAccess, error) {
	if s.SharedDocuments == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.StudentRepo == nil {
		return sharedAccess{}, errors.New("parent: shared documents are not configured")
	}
	var access sharedAccess
	err := s.billingDocumentAccess(ctx, accountID, func(adminCtx context.Context, children []*ChildSummary) error {
		groups, err := s.permittedSchoolGroups(adminCtx, children)
		if err != nil || len(groups) == 0 {
			return err
		}
		access, err = s.sharedReaders(adminCtx, accountID, groups)
		return err
	})
	if err != nil {
		return sharedAccess{}, fmt.Errorf("parent: resolve shared documents: %w", err)
	}
	if len(access.traeger) == 0 {
		return sharedAccess{}, ErrGuardianPermissionDenied
	}
	return access, nil
}

// permittedSchoolGroups keeps the children whose relationship grants
// parent_portal.access and returns, per school, the groups of those children.
func (s *Service) permittedSchoolGroups(ctx context.Context, children []*ChildSummary) (map[int64][]int64, error) {
	groups := map[int64][]int64{}
	for _, child := range children {
		if child == nil || !childHasPermission(child, GuardianPermissionPortalAccess) {
			continue
		}
		student, err := s.StudentRepo.FindByID(ctx, child.StudentID)
		if err != nil {
			return nil, fmt.Errorf("read the group of child %d: %w", child.StudentID, err)
		}
		schoolGroups := groups[child.TenantID]
		if schoolGroups == nil {
			schoolGroups = []int64{}
		}
		if student != nil && student.GroupID != nil && !slices.Contains(schoolGroups, *student.GroupID) {
			schoolGroups = append(schoolGroups, *student.GroupID)
		}
		groups[child.TenantID] = schoolGroups
	}
	return groups, nil
}

// sharedReaders groups the schools by their Träger, in a stable order.
func (s *Service) sharedReaders(ctx context.Context, accountID int64, groups map[int64][]int64) (sharedAccess, error) {
	schoolIDs := make([]int64, 0, len(groups))
	for schoolID := range groups {
		schoolIDs = append(schoolIDs, schoolID)
	}
	slices.Sort(schoolIDs)
	of, err := s.SchoolTraeger.TraegerOfSchools(ctx, schoolIDs)
	if err != nil {
		return sharedAccess{}, err
	}
	access := sharedAccess{readers: map[int64]SharedDocumentReader{}}
	for _, schoolID := range schoolIDs {
		organization, ok := of[schoolID]
		if !ok {
			continue
		}
		reader, seen := access.readers[organization.ID]
		if !seen {
			access.traeger = append(access.traeger, organization)
			reader = SharedDocumentReader{AccountID: accountID, Schools: map[int64][]int64{}}
		}
		reader.Schools[schoolID] = groups[schoolID]
		access.readers[organization.ID] = reader
	}
	return access, nil
}
