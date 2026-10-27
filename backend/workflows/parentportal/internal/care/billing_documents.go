package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Document inbox of the parents portal (spec #4611 user stories 93 and 96,
// #4671, ADR 0126). A Träger sends a confirmation letter to the address of
// the family that registered; the account with that address finds it here
// and gets a push. Two facts must hold, neither alone is enough:
//
//   - The relationship: the account holds parent_portal.enrollments.view
//     on a child at a school of the Träger. Only those Träger are read, so
//     membership or a bare guardian link opens nothing.
//   - The payer (ADR 0088 section 7): the letter is addressed to the current
//     address of the account. A letter of another family of the same Träger
//     never shows, even to a guardian of the same school.
//
// The documents belong to the Elternabrechnung of the Träger; the flow reads
// them in that Träger's transaction through BillingDocuments, which records
// every read with the account.

// ErrBillingDocumentNotFound reports a document that is not in the inbox of
// the account.
var ErrBillingDocumentNotFound = errors.New("parent: document not found in the inbox")

// BillingDocumentReader is the account that reads its inbox, with its
// current address.
type BillingDocumentReader struct {
	AccountID int64
	Email     string
}

// BillingDocument is one document of a Träger in the inbox.
type BillingDocument struct {
	ID             int64
	OrganizationID int64
	TraegerName    string
	Kind           string
	Number         string
	PeriodName     string
	ServiceFrom    calendar.Date
	ServiceUntil   calendar.Date
	IssuedOn       calendar.Date
	TotalCents     int64
	ChildNames     []string
	SentAt         time.Time
}

// BillingDocumentFile is the PDF of one document.
type BillingDocumentFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

// BillingDocuments is the port to the Elternabrechnung's inbox of one
// Träger. Both operations join the caller's Träger transaction of
// organizationID and record the read.
type BillingDocuments interface {
	ListBillingDocuments(ctx context.Context, organizationID int64, reader BillingDocumentReader) ([]BillingDocument, error)
	BillingDocumentLetter(ctx context.Context, organizationID, documentID int64, reader BillingDocumentReader) (BillingDocumentFile, error)
}

// Traeger is the Träger of a school with the name the inbox shows.
type Traeger struct {
	ID   int64
	Name string
}

// SchoolTraeger resolves the active Träger of schools, in the caller's
// administrative transaction. A school without one is left out.
type SchoolTraeger interface {
	TraegerOfSchools(ctx context.Context, schoolIDs []int64) (map[int64]Traeger, error)
}

// TraegerRuntime opens the Träger transaction of an organization (ADR 0105).
type TraegerRuntime interface {
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
}

// ParentAccounts reads the address of an account and the account of an
// address, in the caller's administrative transaction. found is false for an
// address without an account.
type ParentAccounts interface {
	AccountEmail(ctx context.Context, accountID int64) (string, error)
	AccountIDByEmail(ctx context.Context, email string) (accountID int64, found bool, err error)
}

// BillingDocumentPush is the push that a document is in the inbox, for one
// account at one school, about the children the relationship covers.
type BillingDocumentPush struct {
	TenantID   int64
	AccountID  int64
	StudentIDs []int64
	DocumentID int64
	Locale     string
}

// BillingDocumentNotifier sends that push through the school's notification
// router, with the account's consent, in the caller's tenant transaction.
type BillingDocumentNotifier interface {
	NotifyBillingDocument(ctx context.Context, push BillingDocumentPush) error
}

// ListBillingDocuments lists the documents in the inbox of the account, the
// newest first. An account whose relationships grant the inbox at no school
// gets ErrGuardianPermissionDenied.
func (s *Service) ListBillingDocuments(ctx context.Context, accountID int64) ([]BillingDocument, error) {
	traeger, reader, err := s.billingDocumentAccess(ctx, accountID)
	if err != nil {
		return nil, err
	}
	documents := []BillingDocument{}
	for _, organization := range traeger {
		err := s.TraegerRuntime.WithinTraeger(ctx, organization.ID, func(txCtx context.Context) error {
			found, listErr := s.BillingDocuments.ListBillingDocuments(txCtx, organization.ID, reader)
			for _, document := range found {
				document.OrganizationID, document.TraegerName = organization.ID, organization.Name
				documents = append(documents, document)
			}
			return listErr
		})
		if err != nil {
			return nil, fmt.Errorf("parent: list documents: %w", err)
		}
	}
	slices.SortStableFunc(documents, func(a, b BillingDocument) int {
		if order := b.SentAt.Compare(a.SentAt); order != 0 {
			return order
		}
		return int(b.ID - a.ID)
	})
	return documents, nil
}

// BillingDocumentLetter reads the PDF of one document in the inbox of the
// account. The document must belong to a Träger the account's relationships
// open and be addressed to the account.
func (s *Service) BillingDocumentLetter(ctx context.Context, accountID, organizationID, documentID int64) (BillingDocumentFile, error) {
	traeger, reader, err := s.billingDocumentAccess(ctx, accountID)
	if err != nil {
		return BillingDocumentFile{}, err
	}
	if !slices.ContainsFunc(traeger, func(t Traeger) bool { return t.ID == organizationID }) || documentID <= 0 {
		return BillingDocumentFile{}, ErrBillingDocumentNotFound
	}
	var file BillingDocumentFile
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var readErr error
		file, readErr = s.BillingDocuments.BillingDocumentLetter(txCtx, organizationID, documentID, reader)
		return readErr
	})
	if err != nil {
		return BillingDocumentFile{}, err
	}
	return file, nil
}

// billingDocumentAccess resolves the Träger the account's relationships open
// and the account's current address.
func (s *Service) billingDocumentAccess(ctx context.Context, accountID int64) ([]Traeger, BillingDocumentReader, error) {
	if s.BillingDocuments == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.ParentAccounts == nil {
		return nil, BillingDocumentReader{}, errors.New("parent: document inbox is not configured")
	}
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, BillingDocumentReader{}, err
	}
	var access documentAccess
	var email string
	err = s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var resolveErr error
		if access, resolveErr = s.documentAccess(adminCtx, children); resolveErr != nil || len(access.traeger) == 0 {
			return resolveErr
		}
		email, resolveErr = s.ParentAccounts.AccountEmail(adminCtx, accountID)
		return resolveErr
	})
	if err != nil {
		return nil, BillingDocumentReader{}, fmt.Errorf("parent: resolve document inbox: %w", err)
	}
	if len(access.traeger) == 0 {
		return nil, BillingDocumentReader{}, ErrGuardianPermissionDenied
	}
	return access.traeger, BillingDocumentReader{AccountID: accountID, Email: strings.TrimSpace(email)}, nil
}

// documentAccess is what an account's relationships open for the inbox:
// the Träger in the order of the children, the permitted children by
// school and the Träger of each of those schools.
type documentAccess struct {
	traeger  []Traeger
	children map[int64][]*ChildSummary
	schools  map[int64]Traeger
}

// documentAccess keeps the children whose relationship grants the inbox and
// resolves their Träger.
func (s *Service) documentAccess(ctx context.Context, children []*ChildSummary) (documentAccess, error) {
	access := documentAccess{children: map[int64][]*ChildSummary{}}
	var schoolIDs []int64
	for _, child := range children {
		if child == nil || !childHasPermission(child, GuardianPermissionEnrollmentsView) {
			continue
		}
		if _, seen := access.children[child.TenantID]; !seen {
			schoolIDs = append(schoolIDs, child.TenantID)
		}
		access.children[child.TenantID] = append(access.children[child.TenantID], child)
	}
	if len(schoolIDs) == 0 {
		return access, nil
	}
	of, err := s.SchoolTraeger.TraegerOfSchools(ctx, schoolIDs)
	if err != nil {
		return documentAccess{}, err
	}
	access.schools = of
	for _, schoolID := range schoolIDs {
		organization, ok := of[schoolID]
		if ok && !slices.ContainsFunc(access.traeger, func(t Traeger) bool { return t.ID == organization.ID }) {
			access.traeger = append(access.traeger, organization)
		}
	}
	return access, nil
}

// NotifyBillingDocument sends the push that a document of the Träger
// organizationID is in the inbox of the account with recipientEmail. It
// goes to that account at every school of the Träger where a relationship
// grants the inbox, about the children it covers; an address without an
// account, or an account without such a relationship, gets nothing. The
// worker calls it when it sends the Träger's push (ADR 0126).
func (s *Service) NotifyBillingDocument(ctx context.Context, organizationID, documentID int64, recipientEmail string) error {
	if s.SchoolTraeger == nil || s.ParentAccounts == nil || s.DocumentNotifier == nil {
		return errors.New("parent: document notice is not configured")
	}
	email := strings.TrimSpace(recipientEmail)
	if organizationID <= 0 || documentID <= 0 || email == "" {
		return errors.New("parent: document notice needs a Träger, a document and an address")
	}
	var accountID int64
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		id, found, err := s.ParentAccounts.AccountIDByEmail(adminCtx, email)
		if found {
			accountID = id
		}
		return err
	})
	if err != nil || accountID == 0 {
		return err
	}
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return err
	}
	var access documentAccess
	err = s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var resolveErr error
		access, resolveErr = s.documentAccess(adminCtx, children)
		return resolveErr
	})
	if err != nil {
		return fmt.Errorf("parent: resolve document notice: %w", err)
	}
	schoolIDs := make([]int64, 0, len(access.children))
	for schoolID := range access.children {
		if access.schools[schoolID].ID == organizationID {
			schoolIDs = append(schoolIDs, schoolID)
		}
	}
	slices.Sort(schoolIDs)
	for _, schoolID := range schoolIDs {
		if err := s.notifyBillingDocumentAt(ctx, schoolID, accountID, documentID, access.children[schoolID]); err != nil {
			return err
		}
	}
	return nil
}

// notifyBillingDocumentAt sends the push at one school in its tenant
// transaction, in the language the guardian chose there.
func (s *Service) notifyBillingDocumentAt(ctx context.Context, schoolID, accountID, documentID int64, children []*ChildSummary) error {
	studentIDs := make([]int64, len(children))
	for i, child := range children {
		studentIDs[i] = child.StudentID
	}
	return s.Runtime.WithinTenant(ctx, schoolID, func(txCtx context.Context) error {
		locale, err := s.guardianLocale(txCtx, children[0].GuardianProfileID)
		if err != nil {
			return err
		}
		return s.DocumentNotifier.NotifyBillingDocument(txCtx, BillingDocumentPush{
			TenantID: schoolID, AccountID: accountID, StudentIDs: studentIDs, DocumentID: documentID, Locale: locale,
		})
	})
}

// guardianLocale is the portal language the guardian chose at the school,
// empty for the default.
func (s *Service) guardianLocale(ctx context.Context, guardianProfileID int64) (string, error) {
	if s.GuardianProfileRepo == nil || guardianProfileID <= 0 {
		return "", nil
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil || profile == nil || profile.PortalLocale == nil {
		return "", err
	}
	return *profile.PortalLocale, nil
}
