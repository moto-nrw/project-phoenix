package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// The document inbox of the parents portal (#4671, ADR 0126): the root binds
// the workflow's ports to the Elternabrechnung, Organisation & Tenancy,
// Identity & Access and Delivery.

// parentBillingDocumentType is the notification type of the push that a
// document arrived; it equals Delivery's catalogue key.
const parentBillingDocumentType = "parent_billing_document"

// parentBillingDocumentDeepLink is the inbox in the parents portal.
const parentBillingDocumentDeepLink = "/documents"

// WithinTraeger opens the Träger transaction of an organization for the
// inbox (ADR 0105, ADR 0126).
func (ParentPortalRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

// parentBillingDocuments binds the inbox to the Elternabrechnung, which
// reads in the caller's Träger transaction and records every read.
type parentBillingDocuments struct {
	documents parentbilling.ParentDocuments
}

// NewParentBillingDocuments binds the inbox to the Elternabrechnung.
func NewParentBillingDocuments() parentportal.BillingDocumentsPort {
	return parentBillingDocuments{documents: parentBillingCompose.NewParentDocuments()}
}

func (d parentBillingDocuments) ListBillingDocuments(ctx context.Context, organizationID int64, reader parentportal.BillingDocumentReader) ([]parentportal.BillingDocument, error) {
	found, err := d.documents.ListParentDocuments(ctx, organizationID, parentbilling.ParentDocumentReader(reader))
	if errors.Is(err, parentbilling.ErrParentDocumentNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	documents := make([]parentportal.BillingDocument, len(found))
	for i, document := range found {
		documents[i] = parentportal.BillingDocument{
			ID: document.DocumentID, OrganizationID: organizationID, Kind: document.Kind, Number: document.Number,
			PeriodName: document.PeriodName, ServiceFrom: document.ServiceFrom, ServiceUntil: document.ServiceUntil,
			IssuedOn: document.IssuedOn, TotalCents: document.TotalCents, ChildNames: document.ChildNames, SentAt: document.SentAt,
		}
	}
	return documents, nil
}

func (d parentBillingDocuments) BillingDocumentLetter(ctx context.Context, organizationID, documentID int64, reader parentportal.BillingDocumentReader) (parentportal.BillingDocumentFile, error) {
	file, err := d.documents.ParentDocumentLetter(ctx, organizationID, documentID, parentbilling.ParentDocumentReader(reader))
	if errors.Is(err, parentbilling.ErrParentDocumentNotFound) {
		return parentportal.BillingDocumentFile{}, fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	}
	if err != nil {
		return parentportal.BillingDocumentFile{}, err
	}
	return parentportal.BillingDocumentFile(file), nil
}

// parentSchoolTraeger binds the Träger of a school to Organisation &
// Tenancy: an undeleted, active Träger of an undeleted school.
type parentSchoolTraeger struct {
	organizations organizationtenancy.Query
}

// NewParentSchoolTraeger binds the Träger of the parents' schools.
func NewParentSchoolTraeger(organizations organizationtenancy.Query) parentportal.SchoolTraegerPort {
	return parentSchoolTraeger{organizations: organizations}
}

func (t parentSchoolTraeger) TraegerOfSchools(ctx context.Context, schoolIDs []int64) (map[int64]parentportal.Traeger, error) {
	schools, err := t.organizations.ListSchoolsByID(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	var organizationIDs []int64
	for _, school := range schools {
		if !school.IsDeleted() && school.OrganizationID > 0 {
			organizationIDs = append(organizationIDs, school.OrganizationID)
		}
	}
	if len(organizationIDs) == 0 {
		return map[int64]parentportal.Traeger{}, nil
	}
	organizations, err := t.organizations.ListOrganizationsByID(ctx, organizationIDs)
	if err != nil {
		return nil, err
	}
	active := make(map[int64]string, len(organizations))
	for _, organization := range organizations {
		if organization.Active && !organization.IsDeleted() {
			active[organization.ID] = organization.Name
		}
	}
	of := make(map[int64]parentportal.Traeger, len(schools))
	for _, school := range schools {
		if name, ok := active[school.OrganizationID]; ok && !school.IsDeleted() {
			of[school.ID] = parentportal.Traeger{ID: school.OrganizationID, Name: name}
		}
	}
	return of, nil
}

// parentAccounts binds the address of an account to Identity & Access.
type parentAccounts struct {
	accounts identityaccess.GuardianAccessQuery
}

// NewParentAccounts binds the accounts of the inbox.
func NewParentAccounts(accounts identityaccess.GuardianAccessQuery) parentportal.ParentAccountsPort {
	return parentAccounts{accounts: accounts}
}

func (a parentAccounts) AccountEmail(ctx context.Context, accountID int64) (string, error) {
	account, err := a.accounts.FindAccount(ctx, accountID)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return "", nil
	}
	return account.Email, err
}

func (a parentAccounts) AccountIDByEmail(ctx context.Context, email string) (int64, bool, error) {
	account, err := a.accounts.FindAccountByEmail(ctx, email)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return account.ID, true, nil
}

// DocumentNotificationPreferences is the consent of the parents to a
// notification type, read in the school's tenant transaction.
type DocumentNotificationPreferences interface {
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}

// parentDocumentNotifier sends the push that a document arrived through the
// school's notification router: only with the parent's consent, in the
// parent's language, naming no child and no amount.
type parentDocumentNotifier struct {
	notifications deliveryPolicy.NotificationCommands
	preferences   DocumentNotificationPreferences
}

// NewParentDocumentNotifier binds the push of the inbox to Delivery.
func NewParentDocumentNotifier(notifications deliveryPolicy.NotificationCommands, preferences DocumentNotificationPreferences) parentportal.BillingDocumentNotifierPort {
	return parentDocumentNotifier{notifications: notifications, preferences: preferences}
}

func (n parentDocumentNotifier) NotifyBillingDocument(ctx context.Context, push parentportal.BillingDocumentPush) error {
	optedIn, err := n.preferences.FilterOptedIn(ctx, parentBillingDocumentType, []int64{push.AccountID})
	if err != nil || len(optedIn) == 0 {
		return err
	}
	title, body := deliveryPolicy.ParentDeliveryPolicy{}.ParentBillingDocumentCopy(push.Locale)
	_, err = n.notifications.NotifyEventDurably(ctx, deliveryPolicy.NotificationEvent{
		Type:           parentBillingDocumentType,
		IdempotencyKey: fmt.Sprintf("parent-billing-document:%d:%d", push.DocumentID, push.AccountID),
		RelatedType:    parentbilling.PushRelatedBillingDocument, RelatedID: push.DocumentID,
		Audience: deliveryPolicy.NotificationAudience{
			TenantID: push.TenantID, Scope: "guardian", GuardianAccountIDs: optedIn, StudentIDs: push.StudentIDs,
		},
		Priority: "normal", Title: title, Body: body, DeepLink: parentBillingDocumentDeepLink,
	})
	policy := deliveryPolicy.ParentDeliveryPolicy{}
	if policy.IsNotificationsDisabled(err) || policy.IsOutsideActiveWindow(err) {
		return nil
	}
	return err
}

// parentDocumentPushes routes the Träger's deferred push that a document
// arrived (ADR 0126) to the parents portal, which finds the accounts and
// their schools when the worker sends it.
type parentDocumentPushes struct {
	notices parentportal.DocumentNotices
}

func (p parentDocumentPushes) send(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
	var notice parentbilling.BillingDocumentNotice
	if err := json.Unmarshal(intent.DeferredPayload, &notice); err != nil {
		return fmt.Errorf("%w: document notice payload: %w", deliveryContract.ErrCancelled, err)
	}
	if intent.OrganizationID <= 0 || notice.OrganizationID != intent.OrganizationID ||
		intent.RelatedEntityID == nil || *intent.RelatedEntityID != notice.DocumentID {
		return fmt.Errorf("%w: document notice scope mismatch", deliveryContract.ErrCancelled)
	}
	return p.notices.NotifyBillingDocument(ctx, notice.OrganizationID, notice.DocumentID, notice.RecipientEmail)
}

// ParentDocumentNoticeSources are what the push of a new document reads
// (ADR 0126): the children of the parents portal, the Träger of their
// schools, the accounts, the guardians' languages and the school's
// notification router.
type ParentDocumentNoticeSources struct {
	Portal        carePlanCompose.ParentPortalRepositories
	Organizations organizationtenancy.Query
	Identity      identityaccess.GuardianAccessQuery
	People        PeopleRows
	Notifications DeliveryNotifications
}

// BuildParentDocumentNotices composes the push of a new document for the
// Delivery worker over the children at the schools that book the parents
// portal.
func BuildParentDocumentNotices(env RootEnvironment, modules settingsValues.ModuleBookingReader, sources ParentDocumentNoticeSources) (parentportal.DocumentNotices, error) {
	children, err := ParentChildReadsForBookedSchools(NewParentChildReads(carePlanCompose.NewParentChildQueries(sources.Portal.ParentChild, securityruntime.GuardianPermissionGranted)), modules)
	if err != nil {
		return nil, fmt.Errorf("compose parent document notices: %w", err)
	}
	return parentportalcompose.NewDocumentNotices(parentportalcompose.Dependencies{
		Runtime: ParentPortalRuntime{}, ChildRepo: children, SchoolTraeger: NewParentSchoolTraeger(sources.Organizations),
		ParentAccounts: NewParentAccounts(sources.Identity), GuardianProfileRepo: NewParentGuardianProfileReads(sources.People.GuardianProfiles),
		DocumentNotifier: NewParentDocumentNotifier(sources.Notifications.Notifier, sources.Notifications.Preferences),
		Logger:           env.Logger.With("service", "parent-document-notices"),
	}), nil
}

// deferredPushes routes a claimed deferred push by its kind: the Träger's
// push of a new document to the parents portal, every other kind to the
// appointment reminders.
func deferredPushes(reminders func(context.Context, deliveryContract.ClaimedIntent) error, documents parentportal.DocumentNotices) func(context.Context, deliveryContract.ClaimedIntent) error {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
		if intent.Template == parentbilling.PushKindBillingDocument {
			if documents == nil {
				return errors.New("delivery: document notices are not configured")
			}
			return parentDocumentPushes{notices: documents}.send(ctx, intent)
		}
		if reminders == nil {
			return errors.New("delivery: deferred push dispatcher is not configured")
		}
		return reminders(ctx, intent)
	}
}
