package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ExportPhase loads the export payload (exportData) and records the
// GDPR access-log row before returning, so the two are inseparable: a
// caller cannot obtain the PII without the audit, and a failed audit
// write fails the whole call. Both halves run on the caller's tenant tx.
func (d *Decisions) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollment.PhaseExport, error) {
	data, err := d.exportData(ctx, phaseID, childStatusFilter)
	if err != nil {
		return nil, err
	}
	requestCount, childCount := data.Counts()
	if err := d.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, data.Phase, format, childStatusFilter, requestCount, childCount); err != nil {
		return nil, err
	}
	return data, nil
}

// ExportStudent exports the requests of one student's kartei tab and
// records the GDPR access-log row.
func (d *Decisions) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*enrollment.StudentEnrollmentExport, error) {
	data, err := d.exportStudentData(ctx, studentID)
	if err != nil {
		return nil, err
	}
	requestCount, childCount := data.Counts()
	if err := d.recordStudentExportAudit(ctx, actorAccountID, actorRole, data, format, requestCount, childCount); err != nil {
		return nil, err
	}
	return data, nil
}

// exportRequests lists and decodes the requests an export covers, bounding
// the in-memory payload: the renderers assemble the whole document at once,
// so a pathologically large set is rejected up front rather than allocating
// an unbounded file. The cap is well above any real OGS phase.
func (d *Decisions) exportRequests(ctx context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, []*enrollmentModels.Request, error) {
	requests, err := d.deps.Requests.AdminRequests(ctx, filters)
	if err != nil {
		return nil, nil, err
	}
	decoded, err := requestValues(requests)
	if err != nil {
		return nil, nil, err
	}
	return requests, decoded, nil
}

// exportChildren lists and decodes the children of the exported requests.
func (d *Decisions) exportChildren(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestChild, error) {
	children, err := d.deps.Children.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	if _, err := childValues(children); err != nil {
		return nil, err
	}
	return children, nil
}

// exportData assembles the whole phase in a fixed number of queries:
// the phase's requests, the phase, their children, the children's offering
// links, the phase's care-offering catalog, the co-guardians and each
// distinct form-schema version. Everything else is in-memory grouping — no
// query runs inside a loop. Unexported: the only way to obtain export data is
// via ExportPhase, which always records the GDPR audit row.
func (d *Decisions) exportData(ctx context.Context, phaseID int64, childStatusFilter string) (*enrollment.PhaseExport, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("decision: export: phase_id required")
	}
	requests, decoded, err := d.exportRequests(ctx, enrollment.RequestListFilters{PhaseID: phaseID})
	if err != nil {
		return nil, fmt.Errorf("decision: export list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("decision: export phase %d has %d requests (max %d): %w",
			phaseID, len(requests), maxExportRequests, enrollment.ErrExportTooLarge)
	}
	phase, err := d.deps.Phases.Phase(ctx, phaseID)
	if err != nil {
		// Map a missing/unreachable phase to the not-found sentinel so the
		// handler can answer 404 rather than 500, like the phase lookup does.
		return nil, fmt.Errorf("decision: export load phase %d: %w", phaseID, enrollment.ErrPhaseNotFound)
	}
	reqIDs := requestIDsOf(requests)
	children, err := d.exportChildren(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export load children: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, c := range children {
		childIDs = append(childIDs, c.ID)
	}
	offeringsByChild, err := d.phaseExportOfferings(ctx, phase, childIDs)
	if err != nil {
		return nil, err
	}
	guardiansByRequest, err := d.exportGuardians(ctx, reqIDs)
	if err != nil {
		return nil, err
	}
	// Fail closed: missing schema labels would silently omit custom answers
	// from a GDPR export while still recording a complete disclosure.
	schemas, err := loadReportSchemas(ctx, d.deps.Schemas, decoded)
	if err != nil {
		d.logger().Error("decision: export schema lookup failed, aborting export", slog.String("error", err.Error()))
		return nil, fmt.Errorf("decision: export load schemas: %w", err)
	}
	rows := phaseExportRows(requests, groupChildrenByRequest(children, len(reqIDs)), offeringsByChild, guardiansByRequest, childStatusFilter)
	return &enrollment.PhaseExport{Phase: phase, Schemas: schemas, Rows: rows}, nil
}

// phaseExportOfferings resolves the children's selections effective on the
// report date against the phase's offering catalog.
func (d *Decisions) phaseExportOfferings(ctx context.Context, phase *enrollment.Phase, childIDs []int64) (map[int64][]enrollment.ChildOfferingRow, error) {
	links, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, d.deps.Children, childIDs, enrollment.ReportOfferingDate(d.todayDate(), phase))
	if err != nil {
		return nil, fmt.Errorf("decision: export load offerings: %w", err)
	}
	offerings, err := d.deps.Offerings.ListByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: export load care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, off := range offerings {
		offeringByID[off.ID] = off
	}
	return groupOfferingsByChild(links, offeringByID, len(childIDs)), nil
}

// exportGuardians loads and groups the additional guardians (co-guardians)
// so the export carries every submitted contact, matching the admin detail
// and the public status page. Defensive against an unwired port.
func (d *Decisions) exportGuardians(ctx context.Context, requestIDs []int64) (map[int64][]*enrollment.RequestGuardian, error) {
	guardiansByRequest := make(map[int64][]*enrollment.RequestGuardian)
	if d.deps.Guardians == nil {
		return guardiansByRequest, nil
	}
	guardians, err := d.deps.Guardians.RequestGuardians(ctx, requestIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export load co-guardians: %w", err)
	}
	for _, g := range guardians {
		guardiansByRequest[g.RequestID] = append(guardiansByRequest[g.RequestID], g)
	}
	return guardiansByRequest, nil
}

// phaseExportRows builds the rows in request order. A per-child status
// filter mirrors the admin list's status dropdown (exact match on the
// child's own status); under an active filter, a request with no matching
// child is omitted entirely — the list shows children, not registrations.
func phaseExportRows(
	requests []*enrollment.Request,
	childrenByRequest map[int64][]*enrollment.RequestChild,
	offeringsByChild map[int64][]enrollment.ChildOfferingRow,
	guardiansByRequest map[int64][]*enrollment.RequestGuardian,
	childStatusFilter string,
) []enrollment.ExportRequestRow {
	rows := make([]enrollment.ExportRequestRow, 0, len(requests))
	for _, req := range requests {
		kids := childrenByRequest[req.ID]
		childRows := make([]enrollment.ExportChildRow, 0, len(kids))
		for _, c := range kids {
			if childStatusFilter != "" && c.Status != childStatusFilter {
				continue
			}
			childRows = append(childRows, enrollment.ExportChildRow{Child: c, Offerings: offeringsByChild[c.ID]})
		}
		if childStatusFilter != "" && len(childRows) == 0 {
			continue
		}
		rows = append(rows, enrollment.ExportRequestRow{Request: req, Children: childRows, Guardians: guardiansByRequest[req.ID]})
	}
	return rows
}

// exportStudentData assembles the requests whose children created the
// student, keeping only those children, with the offerings effective on each
// phase's report date.
func (d *Decisions) exportStudentData(ctx context.Context, studentID int64) (*enrollment.StudentEnrollmentExport, error) {
	if err := d.checkExportStudent(ctx, studentID); err != nil {
		return nil, err
	}
	requests, decoded, err := d.exportRequests(ctx, enrollment.RequestListFilters{CreatedStudentID: studentID})
	if err != nil {
		return nil, fmt.Errorf("decision: export student list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("decision: export student %d has %d requests (max %d): %w",
			studentID, len(requests), maxExportRequests, enrollment.ErrExportTooLarge)
	}
	reqIDs := requestIDsOf(requests)
	children, err := d.exportChildren(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export student load children: %w", err)
	}
	studentChildren := childrenOfStudent(children, studentID)
	links, offeringByID, err := d.studentExportLinks(ctx, studentChildren)
	if err != nil {
		return nil, err
	}
	phases, err := d.exportStudentPhases(ctx, requests)
	if err != nil {
		return nil, err
	}
	links = filterOfferingsAtPhaseDate(d.todayDate(), links, childrenByIDOf(studentChildren), requestsByIDOf(requests), phases)
	offeringsByChild := groupOfferingsByChild(links, offeringByID, len(studentChildren))
	schemas, err := loadReportSchemas(ctx, d.deps.Schemas, decoded)
	if err != nil {
		return nil, fmt.Errorf("decision: export student load schemas: %w", err)
	}
	return &enrollment.StudentEnrollmentExport{
		StudentID: studentID,
		Schemas:   schemas,
		Phases:    phases,
		Rows:      studentExportRows(requests, groupChildrenByRequest(studentChildren, len(reqIDs)), offeringsByChild),
	}, nil
}

func (d *Decisions) checkExportStudent(ctx context.Context, studentID int64) error {
	if studentID <= 0 {
		return fmt.Errorf("decision: export student: student_id required")
	}
	if d.deps.People.Students == nil {
		return fmt.Errorf("decision: export student: student repo not configured")
	}
	if err := d.deps.People.Students.FindStudent(ctx, studentID); err != nil {
		if d.deps.Runtime.NotFound(err) {
			return fmt.Errorf("decision: export student load student %d: %w", studentID, enrollment.ErrDecisionStudentNotFound)
		}
		return fmt.Errorf("decision: export student load student %d: %w", studentID, err)
	}
	return nil
}

func childrenOfStudent(children []*enrollment.RequestChild, studentID int64) []*enrollment.RequestChild {
	filtered := make([]*enrollment.RequestChild, 0, len(children))
	for _, child := range children {
		if child.CreatedStudentID != nil && *child.CreatedStudentID == studentID {
			filtered = append(filtered, child)
		}
	}
	return filtered
}

// exportStudentPhases loads the phases of the exported requests; a missing
// phase fails the export.
func (d *Decisions) exportStudentPhases(ctx context.Context, requests []*enrollment.Request) (map[int64]*enrollment.Phase, error) {
	phaseIDs := make(map[int64]struct{}, len(requests))
	for _, req := range requests {
		phaseIDs[req.PhaseID] = struct{}{}
	}
	phaseRows, err := d.deps.Phases.PhasesByID(ctx, int64SetKeys(phaseIDs))
	if err != nil {
		return nil, fmt.Errorf("decision: export student load phases: %w", err)
	}
	phases := make(map[int64]*enrollment.Phase, len(phaseRows))
	for _, row := range phaseRows {
		if row != nil {
			phases[row.ID] = row
		}
	}
	for phaseID := range phaseIDs {
		if phases[phaseID] == nil {
			return nil, fmt.Errorf("decision: export student load phase %d: missing", phaseID)
		}
	}
	return phases, nil
}

// studentExportLinks reads the children's offering history and the catalog
// entries behind it.
func (d *Decisions) studentExportLinks(ctx context.Context, children []*enrollment.RequestChild) ([]*enrollment.RequestChildOfferingRecord, map[int64]*enrollmentModels.CareOffering, error) {
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	links, err := enrollment.OfferingHistoryRecordsForChildren(ctx, d.deps.Children, childIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("decision: export student load offerings: %w", err)
	}
	offerings, err := d.deps.Offerings.ListByIDs(ctx, uniqueCareOfferingIDs(links))
	if err != nil {
		return nil, nil, fmt.Errorf("decision: export student load care offerings: %w", err)
	}
	return links, careOfferingsByID(offerings), nil
}

func childrenByIDOf(children []*enrollment.RequestChild) map[int64]*enrollment.RequestChild {
	byID := make(map[int64]*enrollment.RequestChild, len(children))
	for _, child := range children {
		byID[child.ID] = child
	}
	return byID
}

func requestsByIDOf(requests []*enrollment.Request) map[int64]*enrollment.Request {
	byID := make(map[int64]*enrollment.Request, len(requests))
	for _, request := range requests {
		byID[request.ID] = request
	}
	return byID
}

func studentExportRows(
	requests []*enrollment.Request,
	childrenByRequest map[int64][]*enrollment.RequestChild,
	offeringsByChild map[int64][]enrollment.ChildOfferingRow,
) []enrollment.ExportRequestRow {
	rows := make([]enrollment.ExportRequestRow, 0, len(requests))
	for _, req := range requests {
		childRows := make([]enrollment.ExportChildRow, 0, len(childrenByRequest[req.ID]))
		for _, child := range childrenByRequest[req.ID] {
			childRows = append(childRows, enrollment.ExportChildRow{Child: child, Offerings: offeringsByChild[child.ID]})
		}
		if len(childRows) == 0 {
			continue
		}
		rows = append(rows, enrollment.ExportRequestRow{Request: req, Children: childRows})
	}
	return rows
}

func filterOfferingsAtPhaseDate(
	today calendar.Date,
	links []*enrollment.RequestChildOfferingRecord,
	childrenByID map[int64]*enrollment.RequestChild,
	requestsByID map[int64]*enrollment.Request,
	phases map[int64]*enrollment.Phase,
) []*enrollment.RequestChildOfferingRecord {
	filtered := make([]*enrollment.RequestChildOfferingRecord, 0, len(links))
	for _, link := range links {
		phase := linkPhase(link, childrenByID, requestsByID, phases)
		if phase == nil {
			continue
		}
		onDate := enrollment.ReportOfferingDate(today, phase)
		if (link.ValidFrom == nil || !link.ValidFrom.After(onDate)) &&
			(link.ValidUntil == nil || link.ValidUntil.After(onDate)) {
			filtered = append(filtered, link)
		}
	}
	return filtered
}

// linkPhase resolves the phase of an offering link through its child and
// request, or nil.
func linkPhase(
	link *enrollment.RequestChildOfferingRecord,
	childrenByID map[int64]*enrollment.RequestChild,
	requestsByID map[int64]*enrollment.Request,
	phases map[int64]*enrollment.Phase,
) *enrollment.Phase {
	if link == nil {
		return nil
	}
	child := childrenByID[link.RequestChildID]
	if child == nil {
		return nil
	}
	request := requestsByID[child.RequestID]
	if request == nil {
		return nil
	}
	return phases[request.PhaseID]
}

// groupOfferingsByChild resolves each child->offering link against the
// offering catalog and groups the rows per request child. Shared by the
// phase export and the per-student export.
func groupOfferingsByChild(links []*enrollment.RequestChildOfferingRecord, offeringByID map[int64]*enrollmentModels.CareOffering, childCount int) map[int64][]enrollment.ChildOfferingRow {
	offeringsByChild := make(map[int64][]enrollment.ChildOfferingRow, childCount)
	for _, link := range links {
		row := enrollment.ChildOfferingRow{
			OfferingID:            link.CareOfferingID,
			SelectedDays:          link.SelectedDays,
			ManualSelectedDays:    link.ManualSelectedDays,
			AutomaticSelectedDays: link.AutomaticSelectedDays,
		}
		if off := offeringByID[link.CareOfferingID]; off != nil {
			row.OfferingName = off.Name
			row.DaysOfWeekMode = off.DaysOfWeekMode
			row.AvailableDays = off.AvailableDays
		}
		offeringsByChild[link.RequestChildID] = append(offeringsByChild[link.RequestChildID], row)
	}
	return offeringsByChild
}

// groupChildrenByRequest groups request children per request id.
func groupChildrenByRequest(children []*enrollment.RequestChild, requestCount int) map[int64][]*enrollment.RequestChild {
	childrenByRequest := make(map[int64][]*enrollment.RequestChild, requestCount)
	for _, c := range children {
		childrenByRequest[c.RequestID] = append(childrenByRequest[c.RequestID], c)
	}
	return childrenByRequest
}

func requestIDsOf(requests []*enrollment.Request) []int64 {
	ids := make([]int64, 0, len(requests))
	for _, req := range requests {
		ids = append(ids, req.ID)
	}
	return ids
}

func int64SetKeys(values map[int64]struct{}) []int64 {
	ids := make([]int64, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	return ids
}

// RecordPhaseExportAudit writes the GDPR access-log row for a phase
// export. Synchronous and blocking: the caller refuses to serve the
// file when this errors. range_start/range_end carry the phase's service
// window — the temporal span of the disclosed data — and metadata carries
// phase_id, format, status_filter and the row counts.
func (d *Decisions) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollment.Phase, format, statusFilter string, requestCount, childCount int) error {
	const errPrefix = "decision: export audit"
	if d.deps.AccessLog == nil {
		return fmt.Errorf("%s: data access log repo not configured", errPrefix)
	}
	if phase == nil {
		return fmt.Errorf("%s: phase required", errPrefix)
	}
	// An empty filter means the export covered every child — record it as
	// "all" so the audit trail is explicit about the disclosed scope.
	statusFilterLabel := statusFilter
	if statusFilterLabel == "" {
		statusFilterLabel = "all"
	}
	entry, err := newExportAccess(errPrefix, actorAccountID, actorRole,
		calendar.Date(phase.ServiceStartDate).BerlinMidnight(), calendar.Date(phase.ServiceEndDate).EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.Metadata["phase_id"] = phase.ID
	entry.Metadata["format"] = format
	entry.Metadata["status_filter"] = statusFilterLabel
	entry.Metadata["request_count"] = requestCount
	entry.Metadata["child_count"] = childCount
	if err := d.deps.AccessLog.RecordPhaseExport(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

func (d *Decisions) recordStudentExportAudit(ctx context.Context, actorAccountID int64, actorRole string, data *enrollment.StudentEnrollmentExport, format string, requestCount, childCount int) error {
	const errPrefix = "decision: student export audit"
	if d.deps.AccessLog == nil {
		return fmt.Errorf("%s: data access log repo not configured", errPrefix)
	}
	if data == nil || data.StudentID <= 0 {
		return fmt.Errorf("%s: student required", errPrefix)
	}
	now := time.Now()
	rangeStart, rangeEnd := studentExportRange(data.Phases, now)
	entry, err := newExportAccess(errPrefix, actorAccountID, actorRole, rangeStart, rangeEnd, now)
	if err != nil {
		return err
	}
	entry.Metadata["format"] = format
	entry.Metadata["request_count"] = requestCount
	entry.Metadata["child_count"] = childCount
	if err := d.deps.AccessLog.RecordStudentExport(ctx, data.StudentID, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

// studentExportRange spans the service windows of the exported phases, or
// the export instant when there are none.
func studentExportRange(phases map[int64]*enrollment.Phase, now time.Time) (time.Time, time.Time) {
	rangeStart, rangeEnd := now, now
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		start := calendar.Date(phase.ServiceStartDate).BerlinMidnight()
		end := calendar.Date(phase.ServiceEndDate).EndOfDay()
		if rangeStart.Equal(now) || start.Before(rangeStart) {
			rangeStart = start
		}
		if rangeEnd.Equal(now) || end.After(rangeEnd) {
			rangeEnd = end
		}
	}
	return rangeStart, rangeEnd
}
