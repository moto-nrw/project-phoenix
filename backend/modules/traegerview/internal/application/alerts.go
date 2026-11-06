package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// SchoolsAlerts returns the hints of every school of the Träger (#4675). The
// figures come from the same owner reads as SchoolsToday, so a hint never
// disagrees with the figures beside it; the waiting children come from
// Enrollment. The thresholds are read after the school set, in the Träger's
// own transaction: a Träger without schools, or one the moto team
// deactivated, fails on the school set first. The answer holds aggregated
// numbers only, so the read is not written to the access log (ADR 0084
// section 7).
func (o *Overview) SchoolsAlerts(ctx context.Context, organizationID int64) (traegerview.SchoolsAlerts, error) {
	if organizationID <= 0 {
		return traegerview.SchoolsAlerts{}, traegerview.ErrTraegerRequired
	}
	var (
		schools []traegerview.School
		figures map[int64]traegerview.SchoolFigures
		open    map[int64]int
	)
	err := o.read(ctx, organizationID, func(adminCtx context.Context, ids []int64) error {
		var err error
		if schools, err = o.rows.Schools(adminCtx, ids); err != nil {
			return err
		}
		if figures, err = o.schoolFigures(adminCtx, ids); err != nil {
			return err
		}
		open, err = o.enrollments.OpenEnrollmentChildrenByTenant(adminCtx, ids)
		return err
	})
	if err != nil {
		return traegerview.SchoolsAlerts{}, err
	}
	thresholds, err := o.alerts.AlertThresholds(ctx, organizationID)
	if err != nil {
		return traegerview.SchoolsAlerts{}, err
	}
	now := o.now()
	result := traegerview.SchoolsAlerts{Thresholds: thresholds, Schools: make([]traegerview.SchoolAlerts, 0, len(schools)), AsOf: now}
	for _, school := range schools {
		row := traegerview.SchoolAlerts{School: school, Figures: figures[school.ID], OpenEnrollments: open[school.ID]}
		row.Figures.AsOf = now
		row.Alerts = schoolAlerts(row.Figures, row.OpenEnrollments, thresholds)
		result.Schools = append(result.Schools, row)
	}
	return result, nil
}

// schoolAlerts applies the thresholds to the figures of one school. A
// threshold of 0 or less switches its hint off.
//
//   - Staff shortage: children are present and either no staff member is on
//     site or more children than the threshold come on one staff member.
//   - Sick children: at least one child is sick and the sick share of all
//     children reaches the threshold in percent.
//   - Open enrollments: the waiting children reach the threshold.
func schoolAlerts(figures traegerview.SchoolFigures, openEnrollments int, thresholds traegerview.AlertThresholds) []string {
	alerts := []string{}
	if limit := thresholds.ChildrenPerStaff; limit > 0 && figures.ChildrenPresent > 0 &&
		(figures.StaffPresent <= 0 || figures.ChildrenPresent > limit*figures.StaffPresent) {
		alerts = append(alerts, traegerview.AlertStaffShortage)
	}
	if limit := thresholds.SickPercent; limit > 0 && figures.ChildrenSick > 0 && figures.ChildrenTotal > 0 &&
		figures.ChildrenSick*100 >= limit*figures.ChildrenTotal {
		alerts = append(alerts, traegerview.AlertSickChildren)
	}
	if limit := thresholds.OpenEnrollments; limit > 0 && openEnrollments >= limit {
		alerts = append(alerts, traegerview.AlertOpenEnrollments)
	}
	return alerts
}
