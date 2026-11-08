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

type DocumentPushStudent struct {
	TenantID int64
	GroupID  *int64
	Status   string
}

type DocumentPushAccounts interface {
	DocumentPushAccounts(context.Context, []int64) (map[int64]DocumentPushAccount, error)
}
type DocumentPushRelationships interface {
	DocumentPushRelationships(context.Context, []int64) ([]DocumentPushRelationship, error)
}
type DocumentPushStudents interface {
	DocumentPushStudents(context.Context, []int64) (map[int64]DocumentPushStudent, error)
}

type DocumentPushCheck struct {
	Key, ObjectID int64
	Email         string
	TenantID      int64
	StudentIDs    []int64
	GroupIDs      []int64
}
type DocumentPushObjects interface {
	ValidDocumentPushes(context.Context, int64, []DocumentPushCheck) (map[int64]bool, error)
}

type DocumentPushAuthorization struct {
	Runtime                      Runtime
	TraegerRuntime               TraegerRuntime
	Schools                      SchoolTraeger
	Accounts                     DocumentPushAccounts
	Relationships                DocumentPushRelationships
	Students                     DocumentPushStudents
	Billing, Contracts, Releases DocumentPushObjects
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
	var studentIDs []int64
	for _, push := range pushes {
		accounts = append(accounts, push.AccountID)
		studentIDs = append(studentIDs, push.StudentIDs...)
		schools = append(schools, push.TenantID)
	}
	accounts = slices.Compact(slices.Sorted(slices.Values(accounts)))
	schools = slices.Compact(slices.Sorted(slices.Values(schools)))
	var addresses map[int64]DocumentPushAccount
	var relationships []DocumentPushRelationship
	var organizations map[int64]Traeger
	var students map[int64]DocumentPushStudent
	err := a.Runtime.WithinAdmin(ctx, func(txCtx context.Context) error {
		var err error
		addresses, err = a.Accounts.DocumentPushAccounts(txCtx, accounts)
		if err != nil {
			return err
		}
		relationships, err = a.Relationships.DocumentPushRelationships(txCtx, accounts)
		if err != nil {
			return err
		}
		students, err = a.Students.DocumentPushStudents(txCtx, slices.Compact(slices.Sorted(slices.Values(studentIDs))))
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
	type relationshipKey struct{ tenantID, accountID, studentID int64 }
	permissions := make(map[relationshipKey][]string, len(relationships))
	for _, link := range relationships {
		permissions[relationshipKey{link.TenantID, link.AccountID, link.StudentID}] = link.Permissions
	}
	for _, push := range pushes {
		account, exists := addresses[push.AccountID]
		organization, found := organizations[push.TenantID]
		if !exists || !found || account.Email == "" || push.ObjectID <= 0 {
			continue
		}
		var children, groups []int64
		for _, id := range push.StudentIDs {
			student, ok := students[id]
			rights := permissions[relationshipKey{push.TenantID, push.AccountID, id}]
			if !ok || student.TenantID != push.TenantID || student.Status == "alumnus" || !slices.Contains(rights, GuardianPermissionPortalAccess) {
				continue
			}
			if push.Kind != DocumentPushRelease && !slices.Contains(rights, GuardianPermissionEnrollmentsView) {
				continue
			}
			children = append(children, id)
			if student.GroupID != nil {
				groups = append(groups, *student.GroupID)
			}
		}
		if len(children) == 0 {
			continue
		}
		key := group{organization.ID, push.Kind}
		batches[key] = append(batches[key], DocumentPushCheck{Key: push.Key, ObjectID: push.ObjectID, Email: account.Email, TenantID: push.TenantID, StudentIDs: children, GroupIDs: groups})
	}
	for key, candidates := range batches {
		err := a.TraegerRuntime.WithinTraeger(ctx, key.organizationID, func(txCtx context.Context) error {
			objects := a.objectsForKind(key.kind)
			if objects == nil {
				return nil
			}
			allowed, err := objects.ValidDocumentPushes(txCtx, key.organizationID, candidates)
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

func (a *DocumentPushAuthorization) objectsForKind(kind string) DocumentPushObjects {
	switch kind {
	case DocumentPushBilling:
		return a.Billing
	case DocumentPushContract:
		return a.Contracts
	case DocumentPushRelease:
		return a.Releases
	default:
		return nil
	}
}
