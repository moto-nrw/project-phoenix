package application

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// E1 (#5896): class clocks decorate a recurring marker, never create or cancel
// a child's care day. Per-child cancellations and booking coverage still win.
func TestCareDayStatusDoesNotConsumeClassClocks(t *testing.T) {
	t.Parallel()
	id := int64(len(t.Name()))
	date := careDayMonday
	clock := careDayClock("13:00")
	for _, authoritative := range []bool{false, true} {
		for _, booked := range []bool{false, true} {
			for storedMode := range 4 {
				for _, classClock := range []bool{false, true} {
					for classException := range 3 {
						for arrivalException := range 3 {
							for pickupException := range 3 {
								for _, pickupPlan := range []bool{false, true} {
									label := fmt.Sprintf("authoritative=%t/booked=%t/stored=%d/class=%t/classException=%d/arrival=%d/pickup=%d/pickupPlan=%t", authoritative, booked, storedMode, classClock, classException, arrivalException, pickupException, pickupPlan)
									stored := careplan.ArrivalWeek{}
									if storedMode != 0 {
										weekday := 1
										if storedMode == 3 {
											weekday = 2
										}
										row := &careplan.ArrivalSchedule{StudentID: id, Weekday: weekday}
										if storedMode == 2 {
											row.ExpectedArrival = clock
										}
										stored[weekday] = row
									}
									var days careplan.CareDayIndex
									if authoritative {
										days = careplan.CareDayIndex{id: {date: {1: booked}}}
									}
									var class *domain.ClassArrivalBaseline
									if classClock {
										class = &domain.ClassArrivalBaseline{ArrivalTimes: map[int]time.Time{1: clock}}
									}
									var exceptions map[int64]careplan.ClassArrivalExceptionsByDate
									if classException != 0 {
										exception := &careplan.ArrivalBaselineException{}
										if classException == 2 {
											exception.ArrivalTime = clock
										}
										exceptions = map[int64]careplan.ClassArrivalExceptionsByDate{id: {date: exception}}
									}
									status := func(withClocks bool) careplan.CareDayStatus {
										times := class
										if !withClocks {
											times = nil
										}
										weekly, _ := domain.ProjectArrivalWeeks(id, stored, times, days, date, date)
										projection := &careplan.ArrivalBaselineProjection{WeeklyByStudentDate: careplan.ArrivalPlansByStudent{id: weekly}}
										if withClocks {
											projection.ClassExceptionsByStudentDate = exceptions
										}
										plans := newCarePlans()
										plans.bookingsAuthoritative = authoritative
										if projection.HasPlan(id, date) {
											plans.markHasPlan(id, date)
										}
										if row := projection.ForDate(id, date); row != nil {
											plans.arrivalByStudentDate[id] = map[calendar.Date]*careplan.ArrivalSchedule{date: row}
										}
										if pickupPlan {
											plans.markHasPlan(id, date)
											plans.pickupByStudentDate[id] = map[calendar.Date]*careplan.PickupSchedule{date: {StudentID: id, Weekday: 1, PickupTime: clock}}
										}
										if arrivalException != 0 {
											row := &careplan.ArrivalException{StudentID: id, ExceptionDate: careplan.Date(date)}
											if arrivalException == 2 {
												row.ExpectedArrival = &clock
											}
											plans.addArrivalException(row)
										}
										if pickupException != 0 {
											row := &careplan.PickupException{StudentID: id, ExceptionDate: careplan.Date(date)}
											if pickupException == 2 {
												row.PickupTime = &clock
											}
											plans.addPickupException(row)
										}
										return careplan.ResolveCareDay(plans.factsFor(id, date))
									}
									require.Equal(t, status(true), status(false), label)
								}
							}
						}
					}
				}
			}
		}
	}
}
