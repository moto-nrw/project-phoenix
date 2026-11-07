package care

import (
	"context"
	"slices"
)

// DocumentPush is the queued, school-bound announcement of an object. Kind
// selects its owner; Key correlates decisions without exposing personal data.
type DocumentPush struct {
	Key, TenantID, AccountID, ObjectID int64
	Kind                               string
	StudentIDs                         []int64
}

type DocumentPushAccount struct {
	ID    int64
	Email string
}
type DocumentPushRelationship struct {
	TenantID, AccountID, StudentID int64
	Permissions                    []string
}

type DocumentPushRecipients interface {
	DocumentPushAccounts(context.Context, []int64) (map[int64]DocumentPushAccount, error)
	DocumentPushRelationships(context.Context, []int64) ([]DocumentPushRelationship, error)
}

type DocumentPushCheck struct {
	Key, ObjectID int64
	Email         string
	TenantID      int64
	StudentIDs    []int64
}
type DocumentPushObjects interface {
	ValidDocumentPushes(context.Context, int64, string, []DocumentPushCheck) (map[int64]bool, error)
}

type DocumentPushAuthorization struct {
	Runtime        Runtime
	TraegerRuntime TraegerRuntime
	Schools        SchoolTraeger
	Recipients     DocumentPushRecipients
	Objects        DocumentPushObjects
}

// AuthorizeDocumentPushes re-resolves relationships and current account
// addresses once per batch. Owner checks run once per Träger/kind, not device.
func (a *DocumentPushAuthorization) AuthorizeDocumentPushes(ctx context.Context, pushes []DocumentPush) (map[int64]bool, error) {
	result := make(map[int64]bool)
	if len(pushes) == 0 {
		return result, nil
	}
	var accounts []int64
	var schools []int64
	for _, push := range pushes {
		accounts = append(accounts, push.AccountID)
		schools = append(schools, push.TenantID)
	}
	accounts = slices.Compact(slices.Sorted(slices.Values(accounts)))
	schools = slices.Compact(slices.Sorted(slices.Values(schools)))
	var addresses map[int64]DocumentPushAccount
	var relationships []DocumentPushRelationship
	var organizations map[int64]Traeger
	err := a.Runtime.WithinAdmin(ctx, func(txCtx context.Context) error {
		var err error
		addresses, err = a.Recipients.DocumentPushAccounts(txCtx, accounts)
		if err != nil {
			return err
		}
		relationships, err = a.Recipients.DocumentPushRelationships(txCtx, accounts)
		if err != nil {
			return err
		}
		organizations, err = a.Schools.TraegerOfSchools(txCtx, schools)
		return err
	})
	if err != nil {
		return nil, err
	}
	type group struct {
		organizationID int64
		kind           string
	}
	batches := make(map[group][]DocumentPushCheck)
	for _, push := range pushes {
		account, exists := addresses[push.AccountID]
		organization, found := organizations[push.TenantID]
		if !exists || !found || account.Email == "" || push.ObjectID <= 0 {
			continue
		}
		children := permittedPushChildren(push, relationships)
		if len(children) == 0 {
			continue
		}
		key := group{organization.ID, push.Kind}
		batches[key] = append(batches[key], DocumentPushCheck{Key: push.Key, ObjectID: push.ObjectID, Email: account.Email, TenantID: push.TenantID, StudentIDs: children})
	}
	for key, candidates := range batches {
		err := a.TraegerRuntime.WithinTraeger(ctx, key.organizationID, func(txCtx context.Context) error {
			allowed, err := a.Objects.ValidDocumentPushes(txCtx, key.organizationID, key.kind, candidates)
			if err != nil {
				return err
			}
			for _, candidate := range candidates {
				result[candidate.Key] = allowed[candidate.Key]
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func permittedPushChildren(push DocumentPush, relationships []DocumentPushRelationship) []int64 {
	var children []int64
	for _, link := range relationships {
		if link.TenantID != push.TenantID || link.AccountID != push.AccountID || !slices.Contains(push.StudentIDs, link.StudentID) {
			continue
		}
		if !slices.Contains(link.Permissions, GuardianPermissionPortalAccess) || !slices.Contains(link.Permissions, GuardianPermissionEnrollmentsView) {
			continue
		}
		children = append(children, link.StudentID)
	}
	return children
}
