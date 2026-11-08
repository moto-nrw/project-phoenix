package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	contractcompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationcompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	billingcompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	portalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

type parentPushAccounts struct {
	identity identityaccess.AccountBatchQuery
}
type parentPushRelationships struct {
	people peopledirectory.GuardianRelationshipQuery
}
type parentPushStudents struct{ students peopledirectory.StudentQuery }

func (r parentPushAccounts) DocumentPushAccounts(ctx context.Context, ids []int64) (map[int64]parentportal.DocumentPushAccount, error) {
	rows, err := r.identity.FindActiveAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]parentportal.DocumentPushAccount, len(rows))
	for id, row := range rows {
		result[id] = parentportal.DocumentPushAccount{ID: id, Email: row.Email}
	}
	return result, nil
}

func (r parentPushRelationships) DocumentPushRelationships(ctx context.Context, ids []int64) ([]parentportal.DocumentPushRelationship, error) {
	profiles, err := r.people.ListGuardiansByAccount(ctx, ids)
	if err != nil {
		return nil, err
	}
	links, err := r.people.ListGuardianLinksByAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	type profileKey struct{ id, tenant int64 }
	accounts := make(map[profileKey]int64, len(profiles))
	for _, profile := range profiles {
		if profile.AccountID != nil {
			accounts[profileKey{profile.ID, profile.TenantID}] = *profile.AccountID
		}
	}
	result := make([]parentportal.DocumentPushRelationship, 0, len(links))
	for _, link := range links {
		result = append(result, parentportal.DocumentPushRelationship{TenantID: link.TenantID, AccountID: accounts[profileKey{link.GuardianProfileID, link.TenantID}], StudentID: link.StudentID, Permissions: link.Permissions})
	}
	return result, nil
}

func (r parentPushStudents) DocumentPushStudents(ctx context.Context, ids []int64) (map[int64]parentportal.DocumentPushStudent, error) {
	rows, err := r.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]parentportal.DocumentPushStudent, len(rows))
	for _, row := range rows {
		result[row.ID] = parentportal.DocumentPushStudent{TenantID: row.TenantID, GroupID: row.GroupID, Status: row.Status}
	}
	return result, nil
}

type parentBillingPushObjects struct {
	billing parentbilling.DocumentPushValidity
}
type parentContractPushObjects struct {
	contracts carecontract.DocumentPushValidity
}
type parentReleasePushObjects struct {
	releases organizationtenancy.ReleasePushValidity
}

func (o parentBillingPushObjects) ValidDocumentPushes(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
	candidates := make([]parentbilling.DocumentPushCandidate, len(checks))
	for i, c := range checks {
		candidates[i] = parentbilling.DocumentPushCandidate{Key: c.Key, DocumentID: c.ObjectID, Email: c.Email}
	}
	return o.billing.ValidDocumentPushes(ctx, organizationID, candidates)
}
func (o parentContractPushObjects) ValidDocumentPushes(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
	candidates := make([]carecontract.DocumentPushCandidate, len(checks))
	for i, c := range checks {
		candidates[i] = carecontract.DocumentPushCandidate{Key: c.Key, ContractID: c.ObjectID, Email: c.Email, SchoolID: c.TenantID, StudentIDs: c.StudentIDs}
	}
	return o.contracts.ValidContractPushes(ctx, organizationID, candidates)
}
func (o parentReleasePushObjects) ValidDocumentPushes(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
	candidates := make([]organizationtenancy.ReleasePushCandidate, len(checks))
	for i, c := range checks {
		candidates[i] = organizationtenancy.ReleasePushCandidate{Key: c.Key, ReleaseID: c.ObjectID, SchoolID: c.TenantID, GroupIDs: c.GroupIDs}
	}
	return o.releases.ValidReleasePushes(ctx, organizationID, candidates)
}

// NewParentPushObjectAuthorization binds Delivery's batch port to the parent
// workflow and its object owners. Delivery knows no document or guardian rule.
func NewParentPushObjectAuthorization(db transactionruntime.Database) (func(context.Context, []delivery.ClaimedIntent) (map[int64]bool, error), error) {
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	gate := portalcompose.NewDocumentPushAuthorization(portalcompose.DocumentPushAuthorizationDependencies{
		Runtime: ParentPortalRuntime{}, TraegerRuntime: ParentPortalRuntime{}, Schools: NewParentSchoolTraeger(organizations),

		Accounts: parentPushAccounts{identity: NewPeopleIdentityAccess(db, nil)}, Relationships: parentPushRelationships{people: people}, Students: parentPushStudents{students: people},
		Billing: parentBillingPushObjects{billing: billingcompose.NewDocumentPushValidity()}, Contracts: parentContractPushObjects{contracts: contractcompose.NewDocumentPushValidity()}, Releases: parentReleasePushObjects{releases: organizationcompose.NewReleasePushValidity()},
	})
	return BindParentPushObjectAuthorization(gate), nil
}

// BindParentPushObjectAuthorization translates only the device envelope.
func BindParentPushObjectAuthorization(gate parentportal.DocumentPushAuthorization) func(context.Context, []delivery.ClaimedIntent) (map[int64]bool, error) {
	return func(ctx context.Context, intents []delivery.ClaimedIntent) (map[int64]bool, error) {
		result := make(map[int64]bool, len(intents))
		var pushes []parentportal.DocumentPush
		for _, intent := range intents {
			if intent.Template != parentbilling.PushKindBillingDocument {
				result[intent.ID] = true
				continue
			}
			if intent.RelatedEntityType == nil || intent.RelatedEntityID == nil {
				continue
			}
			var kind string
			switch *intent.RelatedEntityType {
			case parentbilling.PushRelatedBillingDocument:
				kind = parentportal.DocumentPushBilling
			case carecontract.PushRelatedContractDocument:
				kind = parentportal.DocumentPushContract
			case organizationtenancy.PushRelatedTraegerDocumentRelease:
				kind = parentportal.DocumentPushRelease
			default:
				continue
			}
			pushes = append(pushes, parentportal.DocumentPush{Key: intent.ID, TenantID: intent.TenantID, AccountID: intent.PushRecipient.AccountID, ObjectID: *intent.RelatedEntityID, Kind: kind, StudentIDs: intent.PushRecipient.StudentIDs})
		}
		allowed, err := gate.AuthorizeDocumentPushes(ctx, pushes)
		if err != nil {
			return nil, err
		}
		for key, value := range allowed {
			result[key] = value
		}
		return result, nil
	}
}
