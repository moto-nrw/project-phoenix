package application

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Overruns serves carecontract.OverrunEvaluations (#4683, ADR 0154). It
// lists the schools of the Träger, then reads each school's overrun list in
// that school's tenant transaction, one after the other; no transaction
// holds the Träger and a school at once (ADR 0105).
type Overruns struct {
	schools   carecontract.TraegerSchools
	careTimes carecontract.SchoolCareTimes
	access    carecontract.SchoolAccessLog
	runtime   carecontract.Runtime
	today     func() calendar.Date
}

// OverrunPorts are the root-bound ports of the evaluation.
type OverrunPorts struct {
	Schools   carecontract.TraegerSchools
	CareTimes carecontract.SchoolCareTimes
	Access    carecontract.SchoolAccessLog
	Runtime   carecontract.Runtime
}

var _ carecontract.OverrunEvaluations = (*Overruns)(nil)

// NewOverruns builds the evaluation.
func NewOverruns(ports OverrunPorts, today func() calendar.Date) *Overruns {
	if ports.Schools == nil || ports.CareTimes == nil || ports.Access == nil || ports.Runtime == nil || today == nil {
		panic("care contract: the schools, the care times, the access log, the runtime and the clock are required")
	}
	return &Overruns{schools: ports.Schools, careTimes: ports.CareTimes, access: ports.Access, runtime: ports.Runtime, today: today}
}

// OverrunSummary answers the figures per school.
func (o *Overruns) OverrunSummary(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.OverrunFilter) (carecontract.OverrunEvaluation, error) {
	return o.evaluate(ctx, reviewer, filter, false)
}

// OverrunChildren answers the figures and the children by name.
func (o *Overruns) OverrunChildren(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.OverrunFilter) (carecontract.OverrunEvaluation, error) {
	return o.evaluate(ctx, reviewer, filter, true)
}

func (o *Overruns) evaluate(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.OverrunFilter, named bool) (carecontract.OverrunEvaluation, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.OverrunEvaluation{}, err
	}
	if err := checkOverrunWindow(filter, o.today()); err != nil {
		return carecontract.OverrunEvaluation{}, err
	}
	schools, err := o.selectSchools(ctx, reviewer, filter.SchoolID)
	if err != nil {
		return carecontract.OverrunEvaluation{}, err
	}
	result := carecontract.OverrunEvaluation{From: filter.From, To: filter.To,
		Schools: make([]carecontract.SchoolOverruns, 0, len(schools)), Children: []carecontract.OverrunChild{}}
	for _, school := range schools {
		list, err := o.readSchool(ctx, reviewer, school, filter, named)
		if err != nil {
			return carecontract.OverrunEvaluation{}, err
		}
		result.ToleranceMinutes = max(result.ToleranceMinutes, list.ToleranceMinutes)
		figures := schoolFigures(list.Rows)
		result.Schools = append(result.Schools, carecontract.SchoolOverruns{
			SchoolID: school.ID, SchoolName: school.Name, DataFrom: list.DataFrom, Figures: figures,
		})
		result.Total = addFigures(result.Total, figures)
		if named {
			result.Children = append(result.Children, namedChildren(school, list.Rows)...)
		}
	}
	sortChildren(result.Children)
	return result, nil
}

// readSchool reads one school's list in its tenant transaction. A named
// read records the children it shows in that school's access log in the
// same transaction: no name leaves without its entry.
func (o *Overruns) readSchool(ctx context.Context, reviewer carecontract.Reviewer, school carecontract.School,
	filter carecontract.OverrunFilter, named bool) (carecontract.SchoolOverrunList, error) {
	var list carecontract.SchoolOverrunList
	err := o.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
		var err error
		if list, err = o.careTimes.Overruns(txCtx, reviewer, filter.From, filter.To); err != nil || !named {
			return err
		}
		ids := make([]int64, len(list.Rows))
		for i, row := range list.Rows {
			ids[i] = row.StudentID
		}
		return o.access.RecordAccess(txCtx, carecontract.SchoolAccess{
			OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID,
			Data: carecontract.SchoolAccessOverruns, View: "care_time_overrun_children", Rows: len(ids), StudentIDs: ids,
			RangeStart: filter.From, RangeEnd: filter.To,
		})
	})
	if err != nil {
		return carecontract.SchoolOverrunList{}, fmt.Errorf("care contract: overruns of school %d: %w", school.ID, err)
	}
	return list, nil
}

func (o *Overruns) selectSchools(ctx context.Context, reviewer carecontract.Reviewer, schoolID int64) ([]carecontract.School, error) {
	schools, err := o.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	if schoolID == 0 {
		return schools, nil
	}
	index := slices.IndexFunc(schools, func(s carecontract.School) bool { return s.ID == schoolID })
	if index < 0 {
		return nil, carecontract.ErrSchoolOutsideTraeger
	}
	return []carecontract.School{schools[index]}, nil
}

// checkOverrunWindow accepts a window of at most MaxOverrunWindowDays that
// does not reach past today.
func checkOverrunWindow(filter carecontract.OverrunFilter, today calendar.Date) error {
	if filter.From == "" || filter.To == "" || filter.To.Before(filter.From) || filter.To.After(today) ||
		filter.From.DaysUntil(filter.To) >= carecontract.MaxOverrunWindowDays {
		return carecontract.ErrOverrunWindowInvalid
	}
	return nil
}

func schoolFigures(rows []carecontract.SchoolOverrunRow) carecontract.OverrunFigures {
	figures := carecontract.OverrunFigures{}
	for _, row := range rows {
		if row.OverrunDays+row.UnbookedDays == 0 {
			continue
		}
		figures.Children++
		figures.OverrunDays += row.OverrunDays
		figures.UnbookedDays += row.UnbookedDays
		figures.ExtraMinutes += row.ExtraMinutes
	}
	return figures
}

func addFigures(a, b carecontract.OverrunFigures) carecontract.OverrunFigures {
	return carecontract.OverrunFigures{Children: a.Children + b.Children, OverrunDays: a.OverrunDays + b.OverrunDays,
		UnbookedDays: a.UnbookedDays + b.UnbookedDays, ExtraMinutes: a.ExtraMinutes + b.ExtraMinutes}
}

func namedChildren(school carecontract.School, rows []carecontract.SchoolOverrunRow) []carecontract.OverrunChild {
	children := make([]carecontract.OverrunChild, 0, len(rows))
	for _, row := range rows {
		if row.OverrunDays+row.UnbookedDays == 0 {
			continue
		}
		children = append(children, carecontract.OverrunChild{
			SchoolID: school.ID, SchoolName: school.Name, StudentID: row.StudentID, FirstName: row.FirstName,
			LastName: row.LastName, SchoolClass: row.SchoolClass, OverrunDays: row.OverrunDays,
			UnbookedDays: row.UnbookedDays, ExtraMinutes: row.ExtraMinutes,
		})
	}
	return children
}

// sortChildren puts the children with the most extra time first.
func sortChildren(children []carecontract.OverrunChild) {
	slices.SortStableFunc(children, func(a, b carecontract.OverrunChild) int {
		return cmp.Or(cmp.Compare(b.ExtraMinutes, a.ExtraMinutes),
			cmp.Compare(b.OverrunDays+b.UnbookedDays, a.OverrunDays+a.UnbookedDays),
			cmp.Compare(a.LastName, b.LastName), cmp.Compare(a.FirstName, b.FirstName), cmp.Compare(a.StudentID, b.StudentID))
	})
}
