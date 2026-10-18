//go:build testhooks

package api

import (
	"context"
	"errors"

	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The constructors below hand suites that may not import the Workforce
// adapters themselves the retained record contracts over the Workforce
// facade (#2690), built the way the service graph builds them. Workforce's
// own suites call its composition; the People Directory and Data Import
// suites build it through the root (ADR 0053 point 7.2, #4005).

// FindStaffMasterData reads a staff member's Stammdaten from Workforce; a
// staff member without a row yet yields (nil, nil), an empty Stammdaten tab
// being a normal state.
func FindStaffMasterData(ctx context.Context, workTime workforceModule.Capability, staffID int64) (*workforceModule.StaffMasterData, error) {
	value, err := workTime.FindStaffMasterData(ctx, staffID)
	if errors.Is(err, workforceModule.ErrStaffMasterDataNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}
