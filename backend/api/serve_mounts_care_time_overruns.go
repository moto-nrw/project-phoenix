package api

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	careContractHTTP "github.com/moto-nrw/project-phoenix/modules/carecontract/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mountCareTimeOverrunRoutes serves the evaluation of overruns over all
// schools of the Träger (#4683, ADR 0156) at /traeger/care-time-overruns:
// the figures per school behind "Berichte lesen", the children by name
// behind "Verträge verwalten".
func mountCareTimeOverrunRoutes(root chi.Router, organizations HolidayOrganizations, statistics studentpresence.StatisticsReports,
	accessLog owners.DataAccessAppender) {
	root.Mount("/traeger/care-time-overruns", NewCareTimeOverrunHTTP(organizations, statistics, accessLog).Router(ProtectedTraegerRoutes))
}

// NewCareTimeOverrunHTTP binds the evaluation's ports: the schools of the
// Träger to Organisation & Tenancy, each school's overrun list to Soll und
// Ist of Student Presence (ADR 0139) in that school's tenant transaction,
// and the names it shows to the school's access log.
func NewCareTimeOverrunHTTP(organizations HolidayOrganizations, statistics studentpresence.StatisticsReports,
	accessLog owners.DataAccessAppender) *careContractHTTP.OverrunResource {
	evaluations := careContractCompose.NewOverrunEvaluations(careContractCompose.OverrunPorts{
		Schools: careContractSchools{organizations: organizations}, CareTimes: careContractCareTimes{statistics: statistics},
		Access: careContractAccessLog{log: accessLog}, Runtime: careContractRuntime{},
	})
	return careContractHTTP.NewOverrunResource(evaluations,
		common.RequiresPermission(identityaccess.TraegerRightReportsRead),
		common.RequiresPermission(identityaccess.TraegerRightContractsManage))
}

// careContractCareTimes reads a school's overrun list from Soll und Ist in
// its tenant transaction. The report records the read in the school's access
// log with the Träger-Büro's account (scope care_times).
type careContractCareTimes struct {
	statistics studentpresence.StatisticsReports
}

func (c careContractCareTimes) Overruns(ctx context.Context, reviewer carecontract.Reviewer, from, to calendar.Date) (carecontract.SchoolOverrunList, error) {
	result, err := c.statistics.CareTimeOverruns(ctx, studentpresence.CareTimeFilters{From: from, To: to},
		studentpresence.StatisticsActor{AccountID: reviewer.AccountID, Role: traegerAccessActorRole})
	if err != nil {
		return carecontract.SchoolOverrunList{}, err
	}
	rows := make([]carecontract.SchoolOverrunRow, len(result.Students))
	for i, student := range result.Students {
		rows[i] = carecontract.SchoolOverrunRow{
			StudentID: student.StudentID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, OverrunDays: student.Summary.OverrunDays,
			UnbookedDays: student.Summary.UnbookedDays, ExtraMinutes: student.Summary.ExtraMinutes,
		}
	}
	return carecontract.SchoolOverrunList{DataFrom: result.DataFrom, ToleranceMinutes: result.ToleranceMinutes,
		ComparedDays: result.ComparedDays, UncomparedDays: result.UncomparedDays, Rows: rows}, nil
}
