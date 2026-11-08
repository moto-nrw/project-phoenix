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

type DocumentPushAccounts func(context.Context, []int64) (map[int64]DocumentPushAccount, error)
type DocumentPushRelationships func(context.Context, []int64) ([]DocumentPushRelationship, error)
type DocumentPushStudents func(context.Context, []int64) (map[int64]DocumentPushStudent, error)

type DocumentPushCheck struct {
	Key, ObjectID int64
	Email         string
	TenantID      int64
	StudentIDs    []int64
	GroupIDs      []int64
}
type DocumentPushObjects func(context.Context, int64, []DocumentPushCheck) (map[int64]bool, error)

type DocumentPushAuthorization struct {
	withDocumentAccess           func(context.Context, func(context.Context) error) error
	TraegerRuntime               TraegerRuntime
	Schools                      SchoolTraeger
	Accounts                     DocumentPushAccounts
	Relationships                DocumentPushRelationships
	Students                     DocumentPushStudents
	Billing, Contracts, Releases DocumentPushObjects
}

// AuthorizeDocumentPushes re-resolves relationships and current account
// addresses once per batch. Owner checks run once per Träger/kind, not device.
type pushRelationshipKey struct{ tenantID, accountID, studentID int64 }
type pushObjectGroup struct {
	organizationID int64
	kind           string
}
type pushAuthority struct {
	accounts      map[int64]DocumentPushAccount
	permissions   map[pushRelationshipKey][]string
	students      map[int64]DocumentPushStudent
	organizations map[int64]Traeger
}

func (a *DocumentPushAuthorization) AuthorizeDocumentPushes(ctx context.Context, pushes []DocumentPush) (map[int64]bool, error) {
	result := make(map[int64]bool)
	if len(pushes) == 0 {
		return result, nil
	}
	authority, err := a.loadPushAuthority(ctx, pushes)
	if err != nil {
		return nil, err
	}
	for group, candidates := range authority.objectChecks(pushes) {
		objects := a.objectsForKind(group.kind)
		if objects == nil {
			continue
		}
		err := a.TraegerRuntime.WithinTraeger(ctx, group.organizationID, func(txCtx context.Context) error {
			allowed, err := objects(txCtx, group.organizationID, candidates)
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

func (a *DocumentPushAuthorization) loadPushAuthority(ctx context.Context, pushes []DocumentPush) (pushAuthority, error) {
	var accounts, schools, students []int64
	for _, push := range pushes {
		accounts = append(accounts, push.AccountID)
		schools = append(schools, push.TenantID)
		students = append(students, push.StudentIDs...)
	}
	accounts = slices.Compact(slices.Sorted(slices.Values(accounts)))
	schools = slices.Compact(slices.Sorted(slices.Values(schools)))
	students = slices.Compact(slices.Sorted(slices.Values(students)))
	authority := pushAuthority{permissions: make(map[pushRelationshipKey][]string)}
	err := a.withDocumentAccess(ctx, func(txCtx context.Context) error {
		var err error
		authority.accounts, err = a.Accounts(txCtx, accounts)
		if err != nil {
			return err
		}
		relationships, err := a.Relationships(txCtx, accounts)
		if err != nil {
			return err
		}
		for _, link := range relationships {
			authority.permissions[pushRelationshipKey{link.TenantID, link.AccountID, link.StudentID}] = link.Permissions
		}
		authority.students, err = a.Students(txCtx, students)
		if err != nil {
			return err
		}
		authority.organizations, err = a.Schools.TraegerOfSchools(txCtx, schools)
		return err
	})
	return authority, err
}

func (a pushAuthority) objectChecks(pushes []DocumentPush) map[pushObjectGroup][]DocumentPushCheck {
	batches := make(map[pushObjectGroup][]DocumentPushCheck)
	for _, push := range pushes {
		account, exists := a.accounts[push.AccountID]
		organization, found := a.organizations[push.TenantID]
		if !exists || !found || account.Email == "" || push.ObjectID <= 0 {
			continue
		}
		children, groups := a.permittedChildren(push)
		if len(children) == 0 {
			continue
		}
		group := pushObjectGroup{organization.ID, push.Kind}
		batches[group] = append(batches[group], DocumentPushCheck{Key: push.Key, ObjectID: push.ObjectID, Email: account.Email, TenantID: push.TenantID, StudentIDs: children, GroupIDs: groups})
	}
	return batches
}

func (a pushAuthority) permittedChildren(push DocumentPush) (children, groups []int64) {
	for _, id := range push.StudentIDs {
		student, ok := a.students[id]
		rights := a.permissions[pushRelationshipKey{push.TenantID, push.AccountID, id}]
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
	return children, groups
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

type DocumentPushAuthorizationDependencies struct {
	Runtime                      Runtime
	TraegerRuntime               TraegerRuntime
	Schools                      SchoolTraeger
	Accounts                     DocumentPushAccounts
	Relationships                DocumentPushRelationships
	Students                     DocumentPushStudents
	Billing, Contracts, Releases DocumentPushObjects
}

func NewDocumentPushAuthorization(deps DocumentPushAuthorizationDependencies) *DocumentPushAuthorization {
	if deps.Runtime == nil || deps.TraegerRuntime == nil || deps.Schools == nil || deps.Accounts == nil || deps.Relationships == nil || deps.Students == nil || deps.Billing == nil || deps.Contracts == nil || deps.Releases == nil {
		panic("parent portal: document push authorization requires all dependencies")
	}
	return &DocumentPushAuthorization{withDocumentAccess: (&Service{Config: Config{Runtime: deps.Runtime}}).billingDocumentAccess, TraegerRuntime: deps.TraegerRuntime, Schools: deps.Schools, Accounts: deps.Accounts, Relationships: deps.Relationships, Students: deps.Students, Billing: deps.Billing, Contracts: deps.Contracts, Releases: deps.Releases}
}
