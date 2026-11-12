package api

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/devicescan"
	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewDeviceScanHolidayGuestSchools binds the kiosk roster to Enrollment's
// hints „Gast aus …“ (#5630), so the tablet names the school a holiday guest
// comes from during its week, like the school portal does (PyrePortal#499).
// It reads in the tenant transaction of the device's school, so no other
// school sees the hint, inside a savepoint, so a failed read leaves that
// transaction usable for the roster. A nil today uses the Berlin calendar day.
func NewDeviceScanHolidayGuestSchools(today func() calendar.Date) devicescan.HolidayGuestSchools {
	return deviceScanHolidayGuestSchools(enrollmentCompose.NewHolidayGuestBadgeList(today))
}

func deviceScanHolidayGuestSchools(badges enrollmentOwner.HolidayGuestBadgeList) devicescan.HolidayGuestSchools {
	return func(ctx context.Context, schoolID int64) (map[int64]string, error) {
		var list []enrollmentOwner.HolidayGuestBadge
		if err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
			var err error
			list, err = badges.HolidayGuestBadges(ctx, schoolID)
			return err
		}); err != nil {
			return nil, err
		}
		schools := make(map[int64]string, len(list))
		for _, badge := range list {
			if badge.SchoolName != "" {
				schools[badge.StudentID] = badge.SchoolName
			}
		}
		return schools, nil
	}
}
