package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	applicationNative "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// NewHolidayPeriods builds the holiday periods of a Träger (#4659). They run
// in the Träger transaction the Träger-Portal route opens (ADR 0105).
func NewHolidayPeriods() enrollment.HolidayPeriods {
	return applicationNative.NewHolidayPeriods(postgres.NewHolidayPeriodStore())
}
