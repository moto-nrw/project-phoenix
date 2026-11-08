package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	contractcompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identitycompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationcompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	billingcompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	portalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

func parentPushAccounts(identity identityaccess.AccountBatchQuery) parentportal.DocumentPushAccounts {
	return func(ctx context.Context, ids []int64) (map[int64]parentportal.DocumentPushAccount, error) {
		rows, err := identity(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]parentportal.DocumentPushAccount, len(rows))
		for id, row := range rows {
			result[id] = parentportal.DocumentPushAccount{ID: id, Email: row.Email}
		}
		return result, nil
	}
}

func parentPushRelationships(people peopledirectory.GuardianRelationshipQuery) parentportal.DocumentPushRelationships {
	return func(ctx context.Context, ids []int64) ([]parentportal.DocumentPushRelationship, error) {
		profiles, err := people.ListGuardiansByAccount(ctx, ids)
		if err != nil {
			return nil, err
		}
		links, err := people.ListGuardianLinksByAccounts(ctx, ids)
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
}

func parentPushStudents(students peopledirectory.StudentQuery) parentportal.DocumentPushStudents {
	return func(ctx context.Context, ids []int64) (map[int64]parentportal.DocumentPushStudent, error) {
		rows, err := students.ListStudentsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]parentportal.DocumentPushStudent, len(rows))
		for _, row := range rows {
			result[row.ID] = parentportal.DocumentPushStudent{TenantID: row.TenantID, GroupID: row.GroupID, Status: row.Status}
		}
		return result, nil
	}
}

func parentBillingPushObjects(billing parentbilling.DocumentPushValidity) parentportal.DocumentPushObjects {
	return func(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
		candidates := make([]parentbilling.DocumentPushCandidate, len(checks))
		for i, c := range checks {
			candidates[i] = parentbilling.DocumentPushCandidate{Key: c.Key, DocumentID: c.ObjectID, Email: c.Email}
		}
		return billing.ValidDocumentPushes(ctx, organizationID, candidates)
	}
}
func parentContractPushObjects(contracts carecontract.DocumentPushValidity) parentportal.DocumentPushObjects {
	return func(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
		candidates := make([]carecontract.DocumentPushCandidate, len(checks))
		for i, c := range checks {
			candidates[i] = carecontract.DocumentPushCandidate{Key: c.Key, ContractID: c.ObjectID, Email: c.Email, SchoolID: c.TenantID, StudentIDs: c.StudentIDs}
		}
		return contracts.ValidContractPushes(ctx, organizationID, candidates)
	}
}
func parentReleasePushObjects(releases organizationtenancy.ReleasePushValidity) parentportal.DocumentPushObjects {
	return func(ctx context.Context, organizationID int64, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
		candidates := make([]organizationtenancy.ReleasePushCandidate, len(checks))
		for i, c := range checks {
			candidates[i] = organizationtenancy.ReleasePushCandidate{Key: c.Key, ReleaseID: c.ObjectID, SchoolID: c.TenantID, GroupIDs: c.GroupIDs}
		}
		return releases.ValidReleasePushes(ctx, organizationID, candidates)
	}
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
	accounts, err := identitycompose.NewAccountBatchQuery(db.Pool())
	if err != nil {
		return nil, err
	}
	gate := portalcompose.NewDocumentPushAuthorization(parentportal.DocumentPushAuthorizationDependencies{
		Runtime: ParentPortalRuntime{}, TraegerRuntime: ParentPortalRuntime{}, Schools: NewParentSchoolTraeger(organizations),

		Accounts: parentPushAccounts(accounts), Relationships: parentPushRelationships(people), Students: parentPushStudents(people),
		Billing: parentBillingPushObjects(billingcompose.NewParentDocuments()), Contracts: parentContractPushObjects(contractcompose.NewInbox()), Releases: parentReleasePushObjects(organizationcompose.NewSharedTraegerDocuments()),
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
