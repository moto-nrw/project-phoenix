package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	found, err := d.documents.ListParentDocuments(ctx, organizationID, parentBillingReader(reader))
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
	file, err := d.documents.ParentDocumentLetter(ctx, organizationID, documentID, parentBillingReader(reader))
	if errors.Is(err, parentbilling.ErrParentDocumentNotFound) {
		return parentportal.BillingDocumentFile{}, fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	}
	if err != nil {
		return parentportal.BillingDocumentFile{}, err
	}
	return parentportal.BillingDocumentFile(file), nil
}

// parentContractDocuments binds the contracts of the inbox to the
// Betreuungsvertrag (#4678, ADR 0153), which reads in the caller's Träger
// transaction and records every read.
type parentContractDocuments struct {
	inbox   carecontract.Inbox
	billing carecontract.ContractBilling
	signing carecontract.InboxSigning
}

// NewParentContractDocuments binds the inbox's contracts to the
// Betreuungsvertrag; signing is the signature in the inbox (#4679), holidays
// dates the withdrawal period the list offers (#6350).
func NewParentContractDocuments(signing carecontract.InboxSigning, holidays carecontract.SchoolHolidays) parentportal.ContractDocumentsPort {
	if holidays == nil {
		panic("parent contract documents: the school holidays are required")
	}
	return parentContractDocuments{inbox: careContractCompose.NewInbox(holidays), billing: careContractCompose.NewContractVersions(), signing: signing}
}

func (d parentContractDocuments) ListContractDocuments(ctx context.Context, organizationID int64, reader parentportal.BillingDocumentReader) ([]parentportal.BillingDocument, error) {
	found, err := d.inbox.InboxContracts(ctx, organizationID, carecontract.InboxReader(reader))
	if errors.Is(err, carecontract.ErrInboxDocumentNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	documents := make([]parentportal.BillingDocument, len(found))
	for i, contract := range found {
		documents[i] = parentportal.BillingDocument{
			ID: contract.ID, OrganizationID: organizationID, Kind: parentportal.DocumentKindContract,
			Number: "Nr. " + strconv.FormatInt(contract.ID, 10), PeriodName: contract.ModulesText + " · " + contract.SchoolName,
			ServiceFrom: contract.EffectiveOn, ChildNames: contract.ChildNames, SentAt: contract.SentAt,
			ContractStatus: contract.Status, SignedAt: contract.SignedAt, CanSign: contract.CanSign, Replaced: contract.Replaced,
			EndsOn: contract.EndsOn, CanEndContract: contract.CanEnd, CanWithdrawContract: contract.CanWithdraw,
		}
	}
	return documents, nil
}

func (d parentContractDocuments) ContractDocumentFile(ctx context.Context, organizationID, contractID int64, reader parentportal.BillingDocumentReader) (parentportal.BillingDocumentFile, error) {
	file, err := d.inbox.InboxContractDocument(ctx, organizationID, contractID, carecontract.InboxReader(reader))
	if errors.Is(err, carecontract.ErrInboxDocumentNotFound) {
		return parentportal.BillingDocumentFile{}, fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	}
	if err != nil {
		return parentportal.BillingDocumentFile{}, err
	}
	return parentportal.BillingDocumentFile{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
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
	identityaccess.AccountPortalLocales
	accounts identityaccess.GuardianAccessQuery
}

// NewParentAccounts binds the accounts of the inbox.
func NewParentAccounts(accounts identityaccess.GuardianAccessQuery) parentportal.ParentAccountsPort {
	locales, ok := accounts.(identityaccess.AccountPortalLocales)
	if !ok {
		panic("parent accounts: account portal locales are required")
	}
	return parentAccounts{accounts: accounts, AccountPortalLocales: locales}
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

// ParentDocumentNotifier sends the push that a document arrived through the
// school's notification router: only with the parent's consent, in the
// parent's language, naming no child and no amount. It serves the letters'
// port and, for a contract or a release (#5941), the one that reports
// whether a push went out.
type ParentDocumentNotifier struct {
	notifications deliveryPolicy.NotificationCommands
	preferences   DocumentNotificationPreferences
}

var (
	_ parentportal.BillingDocumentNotifierPort = ParentDocumentNotifier{}
	_ parentportal.TraegerDocumentNotifierPort = ParentDocumentNotifier{}
)

// NewParentDocumentNotifier binds the push of the inbox to Delivery.
func NewParentDocumentNotifier(notifications deliveryPolicy.NotificationCommands, preferences DocumentNotificationPreferences) ParentDocumentNotifier {
	return ParentDocumentNotifier{notifications: notifications, preferences: preferences}
}

func (n ParentDocumentNotifier) NotifyBillingDocument(ctx context.Context, push parentportal.BillingDocumentPush) error {
	_, err := n.NotifyTraegerDocument(ctx, push)
	return err
}

// NotifyTraegerDocument sends the push of any document kind (#5941) and
// tells whether a device push was enqueued at the school. Without consent,
// with notifications switched off at the school or outside its active window
// none is.
func (n ParentDocumentNotifier) NotifyTraegerDocument(ctx context.Context, push parentportal.BillingDocumentPush) (bool, error) {
	optedIn, err := n.preferences.FilterOptedIn(ctx, parentBillingDocumentType, []int64{push.AccountID})
	if err != nil || len(optedIn) == 0 {
		return false, err
	}
	keyPrefix, relatedType, err := documentPushIdentity(push.Kind)
	if err != nil {
		return false, err
	}
	title, body := deliveryPolicy.ParentDeliveryPolicy{}.ParentBillingDocumentCopy(push.Locale)
	accepted, err := n.notifications.NotifyEventDurably(ctx, deliveryPolicy.NotificationEvent{
		Type:           parentBillingDocumentType,
		IdempotencyKey: fmt.Sprintf("%s:%d:%d", keyPrefix, push.DocumentID, push.AccountID),
		RelatedType:    relatedType, RelatedID: push.DocumentID,
		Audience: deliveryPolicy.NotificationAudience{
			TenantID: push.TenantID, Scope: "guardian", GuardianAccountIDs: optedIn, StudentIDs: push.StudentIDs,
		},
		Priority: "normal", Title: title, Body: body, DeepLink: parentBillingDocumentDeepLink,
	})
	policy := deliveryPolicy.ParentDeliveryPolicy{}
	if policy.IsNotificationsDisabled(err) || policy.IsOutsideActiveWindow(err) {
		return false, nil
	}
	return accepted > 0 && err == nil, err
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

// BuildParentDocumentNotices composes the push of a new document for the
// Delivery worker over the children of the parents portal, the Träger of
// their schools, the accounts, the guardians' languages and the school's
// notification router (ADR 0126). It reads the children without the
// portal's booking filter, which resolves settings per school inside the
// administrative lookup; Delivery's module gate still drops the push of a
// school that does not book the Eltern-App.
func BuildParentDocumentNotices(
	db transactionruntime.Database, env RootEnvironment, portal carePlanCompose.ParentPortalRepositories, organizations organizationtenancy.Query,
	identity identityaccess.GuardianAccessQuery, people PeopleRows, notifications DeliveryNotifications, outbox DeferredPushOutbox,
) parentportal.DocumentNotices {
	notifier := NewParentDocumentNotifier(notifications.Notifier, notifications.Preferences)
	return parentportalcompose.NewDocumentNotices(parentportalcompose.Dependencies{
		Runtime: ParentPortalRuntime{}, ChildRepo: NewParentChildReads(carePlanCompose.NewParentChildQueries(portal.ParentChild, securityruntime.GuardianPermissionGranted)),
		SchoolTraeger: NewParentSchoolTraeger(organizations), ParentAccounts: NewParentAccounts(identity),
		GuardianProfileRepo: NewParentGuardianProfileReads(people.GuardianProfiles),
		DocumentNotifier:    notifier, TraegerRuntime: ParentPortalRuntime{}, StudentRepo: NewParentStudentReads(people.Students),
		Logger: env.Logger.With("service", "parent-document-notices"),
	}, parentportal.DocumentPushPorts{
		// The pushes of a sent contract and of a release (#5941).
		ContractInboxNotices: NewParentContractInboxNotices(), ReleaseNotices: NewParentReleaseNotices(),
		ReleaseSchools: NewParentReleaseSchools(organizations), ReleaseRecipients: NewParentReleaseRecipients(NewParentGroupAudience(db)),
		ReleaseFanOut: ParentReleaseFanOut{Delivery: outbox}, TraegerDocumentNotifier: notifier,
	})
}

// deferredPushes routes a claimed deferred push by its kind: the Träger's
// pushes of a new document (a letter, a contract, a release) to the parents
// portal, every other kind to the appointment reminders.
func deferredPushes(reminders func(context.Context, deliveryContract.ClaimedIntent) error, documents parentportal.DocumentNotices) func(context.Context, deliveryContract.ClaimedIntent) error {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
		if isTraegerDocumentPush(intent.Template) {
			if documents == nil {
				return errors.New("delivery: document notices are not configured")
			}
			return parentDocumentPushes{notices: documents}.route(ctx, intent)
		}
		if reminders == nil {
			return errors.New("delivery: deferred push dispatcher is not configured")
		}
		return reminders(ctx, intent)
	}
}

// ContractSigningView reads a contract of the inbox to sign (#4679).
func (d parentContractDocuments) ContractSigningView(ctx context.Context, organizationID, contractID int64, reader parentportal.BillingDocumentReader) (parentportal.ContractSigningView, error) {
	view, err := d.signing.InboxSigningView(ctx, organizationID, contractID, carecontract.InboxReader(reader))
	return parentSigningView(view, err)
}

// SignContract signs a contract of the inbox (#4679).
func (d parentContractDocuments) SignContract(ctx context.Context, organizationID, contractID int64, reader parentportal.BillingDocumentReader, input parentportal.ContractSignatureInput) (parentportal.ContractSigningView, error) {
	view, err := d.signing.SignInInbox(ctx, organizationID, contractID, carecontract.InboxReader(reader), carecontract.SignatureInput(input))
	return parentSigningView(view, err)
}

// parentSigningView passes the owner's view through as the workflow's
// document and maps the owner's errors onto the workflow's.
func parentSigningView(view carecontract.SigningView, err error) (parentportal.ContractSigningView, error) {
	switch {
	case errors.Is(err, carecontract.ErrInboxDocumentNotFound):
		return nil, fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	case errors.Is(err, carecontract.ErrSignatureInvalid):
		return nil, fmt.Errorf("%w: %w", parentportal.ErrContractSignatureInvalid, err)
	case errors.Is(err, carecontract.ErrWithdrawalNoticeRequired):
		return nil, fmt.Errorf("%w: %w", parentportal.ErrContractWithdrawalNoticeRequired, err)
	case errors.Is(err, carecontract.ErrContractNotSignable):
		return nil, fmt.Errorf("%w: %w", parentportal.ErrContractNotSignable, err)
	case err != nil:
		return nil, err
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		return nil, fmt.Errorf("encode contract signing view: %w", err)
	}
	return encoded, nil
}

// parentBillingReader passes the account with the children whose
// relationship grants it the inbox now, so a monthly letter shows only for
// its own child (ADR 0126 point 2, ADR 0165).
func parentBillingReader(reader parentportal.BillingDocumentReader) parentbilling.ParentDocumentReader {
	children := make([]parentbilling.ParentDocumentChild, 0)
	for schoolID, students := range reader.Children {
		for _, studentID := range students {
			children = append(children, parentbilling.ParentDocumentChild{SchoolID: schoolID, StudentID: studentID})
		}
	}
	return parentbilling.ParentDocumentReader{AccountID: reader.AccountID, Email: reader.Email, Children: children}
}
