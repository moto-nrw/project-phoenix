//go:build testhooks

package parentportal

import (
	"context"
	"encoding/json"

	"github.com/moto-ogs/project-phoenix/workflows/parentportal/internal/care"
)

type MasterDataPerson care.MasterDataPerson
type MasterDataStudent care.MasterDataStudent

func TrackBFieldStateFixture(target, field string, person *MasterDataPerson, student *MasterDataStudent, value json.RawMessage, policy DeparturePolicy) (json.RawMessage, json.RawMessage, bool, error) {
	return care.TrackBFieldStateFixture(target, field, (*care.MasterDataPerson)(person), (*care.MasterDataStudent)(student), value, policy)
}

// StudentGuardianReadsAfterLock calls afterLock with the write's context once
// FindByStudentAndGuardianForUpdate holds the relationship row, so a test can
// park the write while it holds that lock.
func StudentGuardianReadsAfterLock(inner StudentGuardianReads, afterLock func(context.Context)) StudentGuardianReads {
	return lockHookedStudentGuardianReads{StudentGuardianReads: inner, lock: inner.FindByStudentAndGuardianForUpdate, afterLock: afterLock}
}

type lockHookedStudentGuardianReads struct {
	StudentGuardianReads
	lock      func(ctx context.Context, studentID, guardianProfileID int64) (*care.GuardianRelationship, error)
	afterLock func(context.Context)
}

func (r lockHookedStudentGuardianReads) FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*care.GuardianRelationship, error) {
	link, err := r.lock(ctx, studentID, guardianProfileID)
	if err == nil && r.afterLock != nil {
		r.afterLock(ctx)
	}
	return link, err
}
