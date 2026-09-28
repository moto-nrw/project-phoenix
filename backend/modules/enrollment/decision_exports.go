package enrollment

// PhaseExport is the fully-assembled payload for the compact phase
// export. Rows preserve the admin list order (newest submission first).
// Schemas is keyed by schema_id so a renderer can resolve a custom
// field's German label + select-option labels for any request,
// regardless of which form-schema version it was pinned to.
type PhaseExport struct {
	Phase   *Phase
	Schemas map[int64]*FormSchema
	Rows    []ExportRequestRow
}

// Counts returns the number of requests (rows) and the total number of
// children across them. Pure derivation from Rows — the single source
// of truth for both the audit row's metadata (ExportPhase) and the
// rendered document subtitle (the export handler).
func (e *PhaseExport) Counts() (requests, children int) {
	return exportRowCounts(e.Rows)
}

// StudentEnrollmentExport is the fully-assembled payload for exports from
// one student's kartei tab. Rows contain only the matching request_child row,
// even when the original parent submission included siblings.
type StudentEnrollmentExport struct {
	StudentID int64
	Schemas   map[int64]*FormSchema
	Phases    map[int64]*Phase
	Rows      []ExportRequestRow
}

// Counts returns the number of requests (rows) and the total number of
// children across them.
func (e *StudentEnrollmentExport) Counts() (requests, children int) {
	return exportRowCounts(e.Rows)
}

func exportRowCounts(rows []ExportRequestRow) (requests, children int) {
	for _, row := range rows {
		children += len(row.Children)
	}
	return len(rows), children
}

// ExportRequestRow is one parent submission with its resolved children
// and the additional guardians (co-guardians) the parent submitted
// alongside the primary contact. Guardians is nil when the submission had
// none.
type ExportRequestRow struct {
	Request   *Request
	Children  []ExportChildRow
	Guardians []*RequestGuardian
}

// ExportChildRow is one child plus its care-offering selections,
// resolved to offering names/days via the phase's offering catalog.
type ExportChildRow struct {
	Child     *RequestChild
	Offerings []ChildOfferingRow
}
