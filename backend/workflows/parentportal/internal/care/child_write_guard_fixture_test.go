package care

import (
	"context"
	"encoding/json"
)

// summaryGuard answers the child write guard from a fixture child summary,
// for the unit tests that compose the flows from stubs: the membership is
// active, the profile belongs to accountID and the locked relationship holds
// the summary's permissions. The guard's real locks are covered by the
// database tests of the workflow (#6529).
type summaryGuard struct {
	accountID int64
	child     *ChildSummary
}

const summaryGuardProfileID int64 = 4711

// install wires the guard ports into svc and gives the fixture child the
// guardian profile the stubs answer for.
func (g summaryGuard) install(svc *Service) {
	if g.child != nil && g.child.GuardianProfileID == 0 {
		g.child.GuardianProfileID = summaryGuardProfileID
	}
	svc.Memberships = g
	svc.GuardianProfileRepo = summaryGuardProfiles{guard: g}
	svc.StudentGuardianRepo = summaryGuardRelationships{guard: g}
	if svc.GuardianPolicy == nil {
		svc.GuardianPolicy = summaryGuardPolicy{}
	}
}

func (g summaryGuard) LockActiveSchoolMembership(context.Context, int64) (bool, error) {
	return true, nil
}

type summaryGuardProfiles struct {
	GuardianProfileReads
	guard summaryGuard
}

func (p summaryGuardProfiles) LockByIDForUpdate(context.Context, int64) error { return nil }

func (p summaryGuardProfiles) FindByID(_ context.Context, id int64) (*GuardianProfile, error) {
	account := p.guard.accountID
	profile := &GuardianProfile{AccountID: &account}
	profile.ID = id
	return profile, nil
}

type summaryGuardRelationships struct {
	StudentGuardianReads
	guard summaryGuard
}

func (r summaryGuardRelationships) FindByStudentAndGuardianForUpdate(_ context.Context, studentID, profileID int64) (*GuardianRelationship, error) {
	if r.guard.child == nil {
		return nil, ErrStudentGuardianNotFound
	}
	permissions, err := json.Marshal(r.guard.child.GuardianPermissions)
	if err != nil {
		return nil, err
	}
	return &GuardianRelationship{StudentID: studentID, GuardianProfileID: profileID, Permissions: permissions}, nil
}

// summaryGuardPolicy reads a stored permission map; nothing else is asked.
type summaryGuardPolicy struct{ GuardianPolicy }

func (summaryGuardPolicy) PermissionGrantedJSON(raw json.RawMessage, permission string) bool {
	var granted map[string]bool
	return json.Unmarshal(raw, &granted) == nil && granted[permission]
}
