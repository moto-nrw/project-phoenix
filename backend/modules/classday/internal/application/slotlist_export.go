package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
)

func (s *service) RenderList(ctx context.Context, requested classday.Params, format classday.Format) (classday.File, error) {
	result, err := s.BuildList(ctx, requested)
	if err != nil {
		return classday.File{}, err
	}
	// BuildList already refused a malformed day, so this parse cannot fail.
	date, err := parseDate(requested.Date)
	if err != nil {
		return classday.File{}, err
	}
	params := listRequest{Params: requested, date: date}

	// Atomic drift guard: the client echoes the content signature of the preview
	// it verified. If this fresh build no longer matches, live data changed
	// between the client's verification and this render — refuse rather than hand
	// out a file that differs from the approved preview (#1565 review pass 2). An
	// empty ExpectedSignature (older client or a direct render) skips the guard.
	if params.ExpectedSignature != "" && result.Signature != params.ExpectedSignature {
		return classday.File{}, classday.ErrListDrifted
	}

	pickupBased := params.Target == classday.TargetPickupCohort
	// Only a planned list is ticked off on paper; an actual or reconciliation
	// list already records who came (#2077).
	checklist := params.Source == classday.SourcePlanned
	columns := slotListExportColumns(pickupBased, checklist)
	docRows := slotListExportRows(result.Rows, pickupBased, checklist)

	doc := classday.ExportDocument{
		Title:       documentTitle(result),
		Subtitle:    subtitle(result),
		GeneratedAt: time.Now(),
		Filters:     exportFilters(params, result),
		Columns:     columns,
		Rows:        docRows,
		Footer:      confidentialityNote,
	}

	filename := exportFilename(params, result)
	return s.listExport.Render(doc, format, filename)
}

func slotListExportColumns(pickupBased, checklist bool) []classday.ExportColumn {
	columns := []classday.ExportColumn{
		{ID: classday.ExportColumnName, Label: "Name"},
		{ID: classday.ExportColumnSchoolClass, Label: "Klasse"},
		{ID: classday.ExportColumnGroup, Label: "Gruppe"},
		{ID: classday.ExportColumnSlot, Label: "Angebot / Zeitraum"},
	}
	if pickupBased {
		columns = append(columns, classday.ExportColumn{ID: classday.ExportColumnPlannedPickup, Label: "Geplante Abholung"})
	}
	columns = append(columns, classday.ExportColumn{ID: classday.ExportColumnPresenceStatus, Label: "Status"})
	if checklist {
		columns = append(columns, classday.ExportColumn{ID: classday.ExportColumnChecklist, Label: "Abhaken"})
	}

	return columns
}

// The PDF renderer treats a Row with GroupTitle as a pure section marker
// and drops its Values (pdf_design.go paginate). Emit one marker row per
// section followed by plain value rows — the same shape the grouped
// student exports produce. Rows arrive sorted by GroupTitle (sortRows),
// so sections are contiguous.
func slotListExportRows(rows []classday.Row, pickupBased, checklist bool) []classday.ExportRow {
	docRows := make([]classday.ExportRow, 0, len(rows))
	currentGroup := ""
	for _, row := range rows {
		if row.GroupTitle != "" && row.GroupTitle != currentGroup {
			currentGroup = row.GroupTitle
			docRows = append(docRows, classday.ExportRow{GroupTitle: row.GroupTitle})
		}
		values := map[classday.ExportColumnID]string{
			classday.ExportColumnName:           row.Name,
			classday.ExportColumnSchoolClass:    row.SchoolClass,
			classday.ExportColumnGroup:          row.GroupName,
			classday.ExportColumnSlot:           row.Slot,
			classday.ExportColumnPresenceStatus: row.StatusLabel,
		}
		if pickupBased {
			values[classday.ExportColumnPlannedPickup] = row.PickupTime
		}
		if checklist {
			values[classday.ExportColumnChecklist] = ""
		}
		docRows = append(docRows, classday.ExportRow{Values: values})
	}

	return docRows
}

func documentTitle(result *classday.Result) string {
	return fmt.Sprintf("Tagesliste %s – %s", result.Source.Label(), result.ListLabel)
}

func exportFilename(params listRequest, result *classday.Result) string {
	return fmt.Sprintf("Tagesliste %s %s %s", params.Source.Label(), result.ListLabel, params.Date.String())
}

// exportFilters builds the header lines stamped on the printed export.
func exportFilters(params listRequest, result *classday.Result) []string {
	filters := []string{
		"Datum: " + params.date.Format("02.01.2006"),
		"Datenbasis: " + params.Source.Label() + " – " + result.Provenance,
		"Enthalten: " + includedSummary(params, result),
	}
	if names := selectedGroupNames(params, result); len(names) > 0 {
		filters = append(filters, "Gruppen: "+strings.Join(names, ", "))
	}
	if len(params.Classes) > 0 {
		filters = append(filters, "Klassen: "+strings.Join(params.Classes, ", "))
	}
	if params.GroupBy != classday.GroupByNone {
		filters = append(filters, "Gruppiert nach: "+params.GroupBy.Label())
	}
	return filters
}

// selectedGroupNames maps the selected education-group IDs to their display
// names via the options the build already resolved, so a filtered export
// discloses the restriction instead of looking like the full cohort.
// result.Groups is derived from the day's rows (availableOptions(allRows)), so a
// saved selection that references a group with no rows for this date/source is
// absent from it. Such an ID is disclosed via an explicit "Gruppe #<id>" marker
// rather than dropped — otherwise an empty or partial confidential export would
// print as if it were unfiltered (#1565 review pass 2).
func selectedGroupNames(params listRequest, result *classday.Result) []string {
	if len(params.GroupIDs) == 0 {
		return nil
	}
	byID := make(map[int64]string, len(result.Groups))
	for _, group := range result.Groups {
		byID[group.ID] = group.Name
	}
	names := make([]string, 0, len(params.GroupIDs))
	for _, id := range params.GroupIDs {
		if name := byID[id]; name != "" {
			names = append(names, name)
			continue
		}
		names = append(names, fmt.Sprintf("Gruppe #%d", id))
	}
	sort.Strings(names)
	return names
}

func includedSummary(params listRequest, result *classday.Result) string {
	if params.Target == classday.TargetPickupCohort {
		return result.ListLabel
	}
	count := summarySlotCount(params, result)
	if params.ListKind != classday.ListKindNone {
		return fmt.Sprintf("%s (%d %s)", result.ListLabel, count, plural(count, "Termin", "Termine"))
	}
	return fmt.Sprintf("%d %s", count, plural(count, "Angebot", "Angebote"))
}

// summarySlotCount is the number of slots the export actually contains. When an
// explicit instance selection is present (InstanceIDsSet, even for a classified
// list_kind) the count is restricted to the selected slots — so a filtered
// export reports its real subset instead of claiming the whole cohort (#1565
// review). result.Slots is always the full set of matching slots, so the
// selection has to be re-applied here. Cancelled slots are excluded upstream and
// ignored defensively here; deferred slots (a not-yet-started reconciliation
// occurrence excluded from the merge in collectSlotEntries) are dropped in both
// branches when they yield no export rows, so the header cannot claim content the
// export does not contain (#1565).
func summarySlotCount(params listRequest, result *classday.Result) int {
	all := !params.InstanceIDsSet && len(params.InstanceIDs) == 0
	selected := int64Set(params.InstanceIDs)
	count := 0
	for _, slot := range result.Slots {
		if slot.Status == string(ports.InstanceStatusCancelled) {
			continue
		}
		if _, deferred := result.DeferredSlots[slot.InstanceID]; deferred {
			continue
		}
		if all || selected[slot.InstanceID] {
			count++
		}
	}
	return count
}

func subtitle(result *classday.Result) string {
	switch result.Source {
	case classday.SourcePlanned:
		return fmt.Sprintf("%d geplante Kinder", result.Counters.Planned)
	case classday.SourceActual:
		return fmt.Sprintf("%d anwesende Kinder", result.Counters.Present)
	default:
		return fmt.Sprintf(
			"%d geplant, %d anwesend, %d fehlend, %d abgemeldet, %d ungeplant",
			result.Counters.Planned,
			result.Counters.Present,
			result.Counters.Missing,
			result.Counters.Excused,
			result.Counters.Unplanned,
		)
	}
}

func plural(count int, singular string, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}
