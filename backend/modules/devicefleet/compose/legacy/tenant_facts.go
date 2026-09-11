package legacy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// TenantFactDependencies are the readers that answer the pre-tenant questions
// a display token raises.
type TenantFactDependencies struct {
	Schools  platformModels.SchoolRepository
	Settings configService.SettingsService
}

type tenantFacts struct{ deps TenantFactDependencies }

// NewTenantFacts adapts the school and settings readers to the owner's port.
func NewTenantFacts(deps TenantFactDependencies) compose.TenantFacts {
	if deps.Schools == nil || deps.Settings == nil {
		panic("devicefleet tenant facts: all dependencies are required")
	}
	return tenantFacts{deps: deps}
}

// School reports the tenant's name and lifecycle state. A school that cannot
// be found is reported as an unknown display, so a dead link never reveals
// whether the tenant ever existed.
func (f tenantFacts) School(ctx context.Context, tenantID int64) (compose.School, error) {
	school, err := f.deps.Schools.FindByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return compose.School{}, compose.ErrDisplayNotFound
		}
		return compose.School{}, fmt.Errorf("failed to load display school: %w", err)
	}
	if school == nil {
		return compose.School{}, compose.ErrDisplayNotFound
	}
	return compose.School{Name: school.Name, Active: school.Active, Deleted: school.IsDeleted()}, nil
}

// DisplayEnabled resolves the opt-in display.enabled toggle for one tenant.
func (f tenantFacts) DisplayEnabled(ctx context.Context, tenantID int64) (bool, error) {
	return f.deps.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyDisplayEnabled)
}
