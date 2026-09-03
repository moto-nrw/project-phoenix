package peopledirectory_test

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The guardian half of the recording engine: the row-level reads the
// composition supplies next to the person and student engines.
type guardianCall struct {
	accountID  int64
	accountIDs []int64
	ids        []int64
}

func (e *recordingEngine) ListGuardianLinksByAccount(_ context.Context, accountID int64) ([]peopledirectory.GuardianLink, error) {
	e.calls++
	e.guardian = guardianCall{accountID: accountID}
	return nil, nil
}

func (e *recordingEngine) ListGuardiansByAccounts(_ context.Context, accountIDs []int64) ([]peopledirectory.Guardian, error) {
	e.calls++
	e.guardian = guardianCall{accountIDs: accountIDs}
	return nil, nil
}

func (e *recordingEngine) ListGuardiansByIDs(_ context.Context, ids []int64) ([]peopledirectory.Guardian, error) {
	e.calls++
	e.guardian = guardianCall{ids: ids}
	return nil, nil
}

func (e *recordingEngine) CountGuardianLinks(_ context.Context, ids []int64) (map[int64]int, error) {
	e.calls++
	e.guardian = guardianCall{ids: ids}
	return map[int64]int{}, nil
}

func (e *recordingEngine) ObserveGuardianOperation(operation string, _ time.Duration, err error) {
	e.observed = append(e.observed, operation+":"+peopledirectory.ErrorCode(err))
}
