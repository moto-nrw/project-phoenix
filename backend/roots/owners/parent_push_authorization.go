package owners

import (
	"context"

	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	billingcompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	portalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

type parentPushRecipients struct {
	identity identityaccess.AccountBatchQuery
	people   peopledirectory.GuardianRelationshipQuery
}

func (r parentPushRecipients) DocumentPushAccounts(ctx context.Context, ids []int64) (map[int64]parentportal.DocumentPushAccount, error) {
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

func (r parentPushRecipients) DocumentPushRelationships(ctx context.Context, ids []int64) ([]parentportal.DocumentPushRelationship, error) {
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

type parentPushObjects struct {
	billing parentbilling.DocumentPushValidity
}

func (o parentPushObjects) ValidDocumentPushes(ctx context.Context, organizationID int64, kind string, checks []parentportal.DocumentPushCheck) (map[int64]bool, error) {
	if kind != parentbilling.PushRelatedBillingDocument {
		return map[int64]bool{}, nil
	}
	candidates := make([]parentbilling.DocumentPushCandidate, len(checks))
	for i, check := range checks {
		candidates[i] = parentbilling.DocumentPushCandidate{Key: check.Key, DocumentID: check.ObjectID, Email: check.Email}
	}
	return o.billing.ValidDocumentPushes(ctx, organizationID, candidates)
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
		Recipients: parentPushRecipients{identity: NewPeopleIdentityAccess(db, nil), people: people},
		Objects:    parentPushObjects{billing: billingcompose.NewDocumentPushValidity()},
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
			if intent.RelatedEntityType == nil || intent.RelatedEntityID == nil || *intent.RelatedEntityType != parentbilling.PushRelatedBillingDocument {
				continue
			}
			pushes = append(pushes, parentportal.DocumentPush{Key: intent.ID, TenantID: intent.TenantID, AccountID: intent.PushRecipient.AccountID, ObjectID: *intent.RelatedEntityID, Kind: *intent.RelatedEntityType, StudentIDs: intent.PushRecipient.StudentIDs})
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
