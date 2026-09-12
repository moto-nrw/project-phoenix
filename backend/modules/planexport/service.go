package planexport

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// confidentialityNote matches the wording of every other printed export.
const confidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

// Service renders the printable weekly plans.
type Service interface {
	ExportDienstplan(ctx context.Context, params Params) (listexport.File, error)
	ExportBetreuungsplan(ctx context.Context, params Params) (listexport.File, error)
}

type service struct {
	deps   Dependencies
	logger *slog.Logger
}

// NewService creates the plan export service over the bound ports.
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
func (s *service) nonWorkingDays(ctx context.Context, from, to timezone.Date) map[Date]string {
	labels := map[Date]string{}

	if s.deps.Holidays != nil {
		holidays, err := s.deps.Holidays.HolidaysInRange(ctx, dayKey(from), dayKey(to))
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
		ranges, err := s.deps.ClosingDays.ClosingDaysInRange(ctx, dayKey(from), dayKey(to))
		if err != nil {
			s.getLogger().Warn("plan export: closing day lookup failed", "error", err.Error())
		}
		for _, closing := range ranges {
			if closing == nil {
				continue
			}
			start, err := timezone.ParseDate(string(closing.StartDate))
			if err != nil {
				s.getLogger().Warn("plan export: closing day has no valid start", "error", err.Error())
				continue
			}
			end, err := timezone.ParseDate(string(closing.EndDate))
			if err != nil {
				s.getLogger().Warn("plan export: closing day has no valid end", "error", err.Error())
				continue
			}
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			for day := start; !day.After(end); day = day.AddDays(1) {
				labels[dayKey(day)] = "Schließtag: " + closing.Reason
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
	title, rowLabel, legend string,
	params Params,
	weeks []week,
	buildWeek func(w week) []listexport.Row,
	emptyWeekRows func(w week) []listexport.Row,
) listexport.Document {
	rows := make([]listexport.Row, 0, len(weeks)*8)
	multiWeek := len(weeks) > 1
	for _, w := range weeks {
		if multiWeek {
			rows = append(rows, listexport.Row{GroupTitle: w.label()})
		}
		weekRows := buildWeek(w)
		if len(weekRows) == 0 {
			weekRows = emptyWeekRows(w)
		}
		rows = append(rows, weekRows...)
	}

	return listexport.Document{
		Title:       title,
		Subtitle:    rangeSubtitle(weeks),
		GeneratedAt: time.Now(),
		Filters:     documentFilters(params, legend),
		Columns:     columnsFor(rowLabel, weeks),
		Rows:        rows,
		Footer:      confidentialityNote,
	}
}

// emptyWeekRow states that the week is empty instead of leaving the sheet
// blank.
func emptyWeekRows(w week, closedDays map[Date]string) []listexport.Row {
	cells := newDayCells("Keine Einträge in dieser Woche", w)
	for i, day := range w.days {
		if label, ok := closedDays[dayKey(day)]; ok {
			cells.days[i] = []listexport.Line{strong(label)}
		}
	}
	return []listexport.Row{cells.toRow()}
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
		weeks[len(weeks)-1].last().Format("02.01.2006"),
		len(weeks),
	)
}

// documentFilters are the header pills: the variant, so nobody has to guess
// whether the printout in their hand is the public or the internal one, and
// a legend for the cell ranks. The legend is not decoration — without it a
// reader has to infer that bold means presence and regular means task, and
// inferring is exactly what a sheet on a wall must not require.
func documentFilters(params Params, legend string) []string {
	variant := "Aushang"
	if params.Variant == VariantInternal {
		variant = "Interne Fassung (mit Gründen und Lücken)"
	}
	return []string{variant, legend}
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
	return fmt.Sprintf("%s %s bis %s", plan, weeks[0].monday.String(), weeks[len(weeks)-1].last().String())
}
