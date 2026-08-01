package planexport

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// confidentialityNote matches the wording of every other printed export.
const confidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

// Renderer is the slice of listexport this service needs.
type Renderer interface {
	Render(doc listexport.Document, format listexport.Format, filenameBase string) (listexport.File, error)
}

// InstanceStudentCountReader returns the number of children per instance in
// one grouped query, so a Betreuungsplan week costs one count query rather
// than one per block.
type InstanceStudentCountReader interface {
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)
}

// ShiftTypeReader resolves Schichtart names for the shift cells.
type ShiftTypeReader interface {
	ListAll(ctx context.Context) ([]*scheduleModel.ShiftType, error)
}

// Dependencies are the reads the two exports need. The instance-side
// readers are the same narrow interfaces services/schedule already declares
// — the Betreuungsplan cannot reuse StaffScheduleOverview itself, because
// that projection drops cancelled blocks and only knows blocks that have
// staff assigned, while a care plan must print a block whether or not
// anyone is on it yet.
type Dependencies struct {
	Overview      scheduleSvc.StaffScheduleOverviewGetter
	ShiftTypes    ShiftTypeReader
	Instances     scheduleSvc.ActivityInstanceRangeReader
	InstanceStaff scheduleSvc.InstanceStaffBatchReader
	Students      InstanceStudentCountReader
	Rooms         scheduleSvc.RoomBatchReader
	Staff         scheduleSvc.StaffOverviewReader
	// ClosingDays and Holidays label the days nobody is expected to work.
	// Both are optional: without them a closed day simply prints as empty,
	// which is a worse sheet but never a wrong one.
	ClosingDays scheduleSvc.ClosingDayService
	Holidays    scheduleSvc.HolidayService
	Renderer    Renderer
}

// Service renders the printable weekly plans.
type Service interface {
	ExportDienstplan(ctx context.Context, params Params) (listexport.File, error)
	ExportBetreuungsplan(ctx context.Context, params Params) (listexport.File, error)
}

type service struct {
	deps   Dependencies
	logger *slog.Logger
}

// NewService creates the plan export service.
func NewService(deps Dependencies, logger *slog.Logger) Service {
	return &service{deps: deps, logger: logger}
}

func (s *service) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// nonWorkingDays maps a calendar day to the reason it is closed, e.g.
// "Schließtag: Betriebsferien" or "Feiertag: Christi Himmelfahrt". A closing
// day wins over a holiday when both apply — it is the tenant's own decision
// and carries the more specific wording.
func (s *service) nonWorkingDays(ctx context.Context, from, to timezone.Date) map[timezone.Date]string {
	labels := map[timezone.Date]string{}

	if s.deps.Holidays != nil {
		holidays, err := s.deps.Holidays.HolidaysInRange(ctx, from, to)
		if err != nil {
			// A missing holiday label costs a line on the sheet, not its
			// correctness — the plan itself is unaffected.
			s.getLogger().Warn("plan export: holiday lookup failed", "error", err.Error())
		}
		for _, holiday := range holidays {
			labels[holiday.Date] = "Feiertag: " + holiday.Name
		}
	}

	if s.deps.ClosingDays != nil {
		ranges, err := s.deps.ClosingDays.ClosingDaysInRange(ctx, from, to)
		if err != nil {
			s.getLogger().Warn("plan export: closing day lookup failed", "error", err.Error())
		}
		for _, closing := range ranges {
			if closing == nil {
				continue
			}
			for day := closing.StartDate; !day.After(closing.EndDate); day = day.AddDays(1) {
				if day.Before(from) || day.After(to) {
					continue
				}
				labels[day] = "Schließtag: " + closing.Reason
			}
		}
	}

	return labels
}

// document assembles the finished document from per-week row builders.
// buildWeek returns the rows of one sheet; an empty result becomes an
// explicit "nothing planned" row, because a silently omitted week would be
// indistinguishable from a broken export.
func (s *service) document(
	title, rowLabel string,
	params Params,
	weeks []week,
	buildWeek func(w week) []listexport.Row,
) listexport.Document {
	rows := make([]listexport.Row, 0, len(weeks)*8)
	multiWeek := len(weeks) > 1
	for _, w := range weeks {
		if multiWeek {
			rows = append(rows, listexport.Row{GroupTitle: w.label()})
		}
		weekRows := buildWeek(w)
		if len(weekRows) == 0 {
			weekRows = []listexport.Row{emptyWeekRow()}
		}
		rows = append(rows, weekRows...)
	}

	return listexport.Document{
		Title:       title,
		Subtitle:    rangeSubtitle(weeks),
		GeneratedAt: time.Now(),
		Filters:     documentFilters(params),
		Columns:     columnsFor(rowLabel, weeks),
		Rows:        rows,
		Footer:      confidentialityNote,
	}
}

// emptyWeekRow states that the week is empty instead of leaving the sheet
// blank.
func emptyWeekRow() listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		listexport.ColumnPlanRowLabel: "Keine Einträge in dieser Woche",
	}}
}

// rangeSubtitle names the printed window: one week by its label, several by
// their span.
func rangeSubtitle(weeks []week) string {
	if len(weeks) == 0 {
		return ""
	}
	if len(weeks) == 1 {
		return weeks[0].label()
	}
	return fmt.Sprintf("%s – %s (%d Wochen)",
		weeks[0].days[0].Format("02.01.2006"),
		weeks[len(weeks)-1].days[4].Format("02.01.2006"),
		len(weeks),
	)
}

// documentFilters are the header pills. The variant is stated on the sheet
// itself so nobody has to guess whether a printout in their hand is the
// public or the internal one.
func documentFilters(params Params) []string {
	switch params.Variant {
	case VariantInternal:
		return []string{"Interne Fassung (mit Gründen und Lücken)"}
	default:
		return []string{"Aushang"}
	}
}

// filename keeps the German label but leaves the slug to listexport, which
// already folds umlauts and punctuation.
func filename(plan string, weeks []week) string {
	if len(weeks) == 0 {
		return plan
	}
	if len(weeks) == 1 {
		return fmt.Sprintf("%s %s", plan, weeks[0].monday.String())
	}
	return fmt.Sprintf("%s %s bis %s", plan, weeks[0].monday.String(), weeks[len(weeks)-1].days[4].String())
}
