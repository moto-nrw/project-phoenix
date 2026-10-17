package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/classday"
)

func (s *service) BuildList(ctx context.Context, params classday.Params) (*classday.Result, error) {
	if err := s.requireTimetableEnabled(ctx); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	date, err := parseDate(params.Date)
	if err != nil {
		return nil, err
	}
	return s.buildList(ctx, listRequest{Params: params, date: date})
}

func (s *service) buildList(ctx context.Context, params listRequest) (*classday.Result, error) {
	if err := s.validateListRequest(params); err != nil {
		return nil, err
	}
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve the pickup-cohort cutoffs ONCE per build and reuse the snapshot for
	// both the list label/provenance and the row collection. Resolving them twice
	// (label, then collectPickupEntries) would let an administrator's mid-build
	// cutoff change land between the two reads under READ COMMITTED, so the header
	// could name a different threshold than the rows were bucketed by (#1565 review
	// pass 2). Only pickup lists need the cutoffs; a slot list neither reads them
	// nor should fail on an invalid pickup config.
	var pickupCfg pickupBucketConfig
	if params.Target == classday.TargetPickupCohort {
		pickupCfg, err = s.pickupBuckets(ctx)
		if err != nil {
			return nil, err
		}
	}

	listLabel := s.listLabel(params, pickupCfg)

	result := &classday.Result{
		Date:         params.Date.String(),
		Target:       params.Target,
		PickupCohort: params.PickupCohort,
		ListKind:     params.ListKind,
		ListLabel:    listLabel,
		Source:       params.Source,
		GroupBy:      params.GroupBy,
		Provenance:   s.provenance(params, listLabel),
		Slots:        []classday.Slot{},
		Rows:         []classday.Row{},
	}

	var entries []mergedEntry
	if params.Target == classday.TargetPickupCohort {
		entries, err = s.collectPickupEntries(ctx, params, pickupCfg, result)
	} else {
		entries, err = s.collectSlotEntries(ctx, params, result)
	}
	if err != nil {
		return nil, err
	}

	// Full (unfiltered) rows for the requested source — used both to derive the
	// available group/class options and as the input to the row filters.
	allRows, err := s.enrichEntries(ctx, entries, params.Source, access, params.date)
	if err != nil {
		return nil, err
	}
	completeListResult(params, result, allRows)
	return result, nil
}

// validateListRequest keeps date restrictions before any access or data lookup.
func (s *service) validateListRequest(params listRequest) error {
	if !params.Target.Valid() {
		return fmt.Errorf("unknown list target %q", params.Target)
	}
	if !params.Source.Valid() {
		return fmt.Errorf("unknown source %q", params.Source)
	}
	if params.Target == classday.TargetPickupCohort && !params.PickupCohort.Valid() {
		return fmt.Errorf("unknown pickup cohort %q", params.PickupCohort)
	}
	if !params.ListKind.Valid() {
		return fmt.Errorf("unknown list kind %q", params.ListKind)
	}
	if params.Target != classday.TargetSlots && params.ListKind != classday.ListKindNone {
		return fmt.Errorf("list kind %q is not valid for target %q", params.ListKind, params.Target)
	}
	if !params.GroupBy.ValidFor(params.Target) {
		return fmt.Errorf("grouping %q is not valid for target %q", params.GroupBy, params.Target)
	}
	// A Ganztag pickup plan cannot be reconstructed for a past date (the pickup
	// schedule has no history) — refuse rather than project the current schedule
	// backwards. Slot-based lists stay available for any date (#1565 review).
	if params.Target == classday.TargetPickupCohort && params.date.Before(s.todayDate()) {
		return classday.ErrPickupCohortPastDate
	}
	// A reconciliation on a strictly-future date has no presence evidence to merge
	// against, so every planned child would be labelled "Fehlt". Refuse it rather
	// than emit a list that reports the whole group missing before the day starts
	// (#1565 review). Today is fine — evidence accrues through the day; Plan/Ist
	// stay available for any date. Slots that have not started yet *on today* are
	// handled per-slot in collectSlotEntries (a not-yet-begun occurrence is
	// excluded from the merge, not reported as a group of missing children).
	if params.Source == classday.SourceReconciliation && params.date.After(s.todayDate()) {
		return classday.ErrReconciliationFutureDate
	}
	return nil
}

// completeListResult derives filter options, ordered rows, counters and hashes
// from the same unfiltered source rows.
func completeListResult(params listRequest, result *classday.Result, allRows []classday.Row) {
	result.Groups, result.Classes = availableOptions(allRows)

	rows := filterRows(allRows, params)
	applyGrouping(rows, params.GroupBy)
	sort.SliceStable(rows, func(i, j int) bool {
		return listRowLess(&rows[i], &rows[j])
	})
	result.Rows = rows
	result.Counters = countRows(rows, params.Source)
	// Fingerprint the exact rendered header (title + every export filter line)
	// so the signature reflects the "Enthalten" slot summary and the other
	// metadata lines the export prints, not just rows and counters (#1565 review
	// pass 7). Computed here because the header needs Params.
	result.ExportHeaderSignature = strings.Join(
		append([]string{documentTitle(result)}, exportFilters(params, result)...),
		"\x1f",
	)
	result.Signature = listSignature(result)
}

func listRowLess(left, right *classday.Row) bool {
	// Keep grouped rows contiguous: section heading first, then the same
	// slot/name order within each section.
	if left.GroupTitle != right.GroupTitle {
		return left.GroupTitle < right.GroupTitle
	}
	if left.Slot != right.Slot {
		return left.Slot < right.Slot
	}
	if left.Name != right.Name {
		return left.Name < right.Name
	}
	// Stable ID tie-breakers: two rows can share group title, slot and full
	// name — e.g. two identically named unplanned attendees in one slot. Their
	// input order comes from map iteration / an incompletely ordered query,
	// while listSignature hashes InstanceID and StudentID positionally, so
	// without a deterministic order unchanged data could produce different
	// preview/export signatures and trigger false drift refusals (#1565 review
	// pass 9).
	if left.InstanceID != right.InstanceID {
		return left.InstanceID < right.InstanceID
	}
	return left.StudentID < right.StudentID
}

// listSignature is a stable content hash of the rendered list: its label,
// provenance, the precomputed export header (title + filter lines, which carry
// the "Enthalten" slot summary — see Result.exportHeaderSig), counters and every
// row's identity, placement and status. Rows are already sorted deterministically
// by BuildList, so a positional walk is stable across two builds of the same
// unchanged data. Unit/record separators (0x1f / 0x1e) delimit fields so no value
// can be confused with a boundary. See the Result.Signature doc for how the
// export drift guard uses it (#1565 review pass 2 + pass 7).
func listSignature(r *classday.Result) string {
	var b strings.Builder
	b.WriteString(r.ListLabel)
	b.WriteByte('\x1e')
	b.WriteString(r.Provenance)
	b.WriteByte('\x1e')
	// The rendered document header (title + export filter lines), precomputed in
	// BuildList. Binds the signature to the "Enthalten" slot summary and the
	// date/group/class/grouping metadata the export prints — none of which the
	// row/counter hash below covers (#1565 review pass 7).
	b.WriteString(r.ExportHeaderSignature)
	b.WriteByte('\x1e')
	fmt.Fprintf(&b, "%d\x1f%d\x1f%d\x1f%d\x1f%d\x1e",
		r.Counters.Planned, r.Counters.Present, r.Counters.Missing,
		r.Counters.Excused, r.Counters.Unplanned)
	for _, row := range r.Rows {
		fmt.Fprintf(&b,
			"%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%t\x1f%t\x1f%t\x1f%t\x1f%s\x1e",
			row.StudentID, row.Name, row.SchoolClass, row.GroupName,
			groupIDSignatureField(row.GroupID), row.InstanceID, row.Slot,
			row.RoomName, row.PickupTime, row.StatusLabel, row.Planned,
			row.Present, row.Unplanned, row.Excused, row.GroupTitle)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func groupIDSignatureField(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

// applyGrouping stamps each row's GroupTitle from the chosen dimension. With
// GroupByNone every title stays empty (a flat list). Rows whose group value is
// empty fall into a generic "Ohne …" bucket so they never silently vanish.
func applyGrouping(rows []classday.Row, groupBy classday.GroupBy) {
	if groupBy == classday.GroupByNone {
		return
	}
	// GroupBySlot keys on the Slot label, but that label is NOT unique across
	// activity instances: two concrete instances can share a title and time range
	// (parallel offerings in different rooms, or two templates). Since both the
	// export's repeated-marker suppression and the frontend's group_title
	// bucketing key sections off the GroupTitle string, an undisambiguated label
	// would fold distinct instances into one section. Precompute a per-instance
	// heading suffix that only fires when a label actually collides (#1565 review
	// pass 2).
	slotSuffix := slotHeadingDisambiguation(rows, groupBy)
	for i := range rows {
		var value string
		switch groupBy {
		case classday.GroupBySlot:
			value = rows[i].Slot
			value += slotSuffix[rows[i].InstanceID]
		case classday.GroupByRoom:
			value = rows[i].RoomName
		case classday.GroupByClass:
			value = rows[i].SchoolClass
		case classday.GroupByPickupTime:
			value = rows[i].PickupTime
		}
		if value == "" {
			value = "Ohne " + groupBy.Label()
		}
		rows[i].GroupTitle = groupBy.Label() + ": " + value
	}
}

// slotHeadingDisambiguation returns a per-instance heading suffix for GroupBySlot
// so distinct activity instances that share one Slot label never merge into a
// single section. Instances whose label is already unique get no suffix (the
// map returns "" for them), so the common case renders exactly as before. When a
// label is shared by several instances, each colliding instance is tagged with
// its room name where that uniquely identifies it, otherwise a running ordinal —
// guaranteeing a distinct, deterministic heading without leaking raw IDs. Returns
// nil for any non-slot grouping (no suffix ever applied).
func slotHeadingDisambiguation(rows []classday.Row, groupBy classday.GroupBy) map[int64]string {
	if groupBy != classday.GroupBySlot {
		return nil
	}
	byLabel := slotLabelInfos(rows)
	suffix := map[int64]string{}
	for _, li := range byLabel {
		if len(li.ids) < 2 {
			continue // unique label — no disambiguation, heading stays clean
		}
		sort.Slice(li.ids, func(i, j int) bool { return li.ids[i] < li.ids[j] })
		roomCount := map[string]int{}
		for _, id := range li.ids {
			roomCount[li.room[id]]++
		}
		// Two passes with a shared `used` set so the room and ordinal suffixes can
		// never render the same string. A numeric room name like "1" produces the
		// suffix " (1)", which is textually identical to the first ordinal " (1)";
		// emitting both would hand two distinct instances the same GroupTitle and
		// re-merge the sections this function exists to split. First assign every
		// uniquely-identifying room its " (room)" suffix and record it; then fill the
		// remaining instances with running ordinals, SKIPPING any ordinal string a
		// room suffix already claimed. This keeps both forms clean (no verbose prefix,
		// no leaked IDs) while guaranteeing distinct, deterministic headings
		// (#1565 review pass 2 follow-up).
		used := uniqueRoomSuffixes(suffix, li.ids, li.room, roomCount)
		addOrdinalSuffixes(suffix, li.ids, used)
	}
	return suffix
}

// slotLabelInfo holds each label's distinct instance IDs and first room names.
type slotLabelInfo struct {
	ids   []int64
	room  map[int64]string
	known map[int64]struct{}
}

func slotLabelInfos(rows []classday.Row) map[string]*slotLabelInfo {
	byLabel := map[string]*slotLabelInfo{}
	for _, r := range rows {
		li := byLabel[r.Slot]
		if li == nil {
			li = &slotLabelInfo{room: map[int64]string{}, known: map[int64]struct{}{}}
			byLabel[r.Slot] = li
		}
		if _, ok := li.known[r.InstanceID]; ok {
			continue
		}
		li.known[r.InstanceID] = struct{}{}
		li.ids = append(li.ids, r.InstanceID)
		li.room[r.InstanceID] = r.RoomName
	}
	return byLabel
}

func uniqueRoomSuffixes(suffix map[int64]string, ids []int64, rooms map[int64]string, roomCount map[string]int) map[string]struct{} {
	used := map[string]struct{}{}
	for _, id := range ids {
		if room := rooms[id]; room != "" && roomCount[room] == 1 {
			s := " (" + room + ")"
			suffix[id] = s
			used[s] = struct{}{}
		}
	}
	return used
}

func addOrdinalSuffixes(suffix map[int64]string, ids []int64, used map[string]struct{}) {
	ordinal := 0
	for _, id := range ids {
		if _, done := suffix[id]; done {
			continue
		}
		var s string
		for {
			ordinal++
			s = fmt.Sprintf(" (%d)", ordinal)
			if _, taken := used[s]; !taken {
				break
			}
		}
		suffix[id] = s
		used[s] = struct{}{}
	}
}

// availableOptions derives the distinct education groups and school classes
// present in the unfiltered rows, sorted for stable UI dropdowns.
func availableOptions(rows []classday.Row) ([]classday.GroupOption, []string) {
	groupByID := map[int64]string{}
	classSet := map[string]struct{}{}
	for _, row := range rows {
		if row.GroupID != nil && row.GroupName != "" {
			groupByID[*row.GroupID] = row.GroupName
		}
		if row.SchoolClass != "" {
			classSet[row.SchoolClass] = struct{}{}
		}
	}
	groups := make([]classday.GroupOption, 0, len(groupByID))
	for id, name := range groupByID {
		groups = append(groups, classday.GroupOption{ID: id, Name: name})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	classes := make([]string, 0, len(classSet))
	for class := range classSet {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	return groups, classes
}

// filterRows applies the offering/group/class filters (AND). An empty filter
// slice means "no restriction" on that dimension.
func filterRows(rows []classday.Row, params listRequest) []classday.Row {
	if params.Target == classday.TargetSlots && params.InstanceIDsSet && len(params.InstanceIDs) == 0 {
		return []classday.Row{}
	}
	instanceSet := int64Set(params.InstanceIDs)
	groupSet := int64Set(params.GroupIDs)
	classSet := make(map[string]struct{}, len(params.Classes))
	for _, c := range params.Classes {
		classSet[c] = struct{}{}
	}

	out := make([]classday.Row, 0, len(rows))
	for _, row := range rows {
		if !rowMatchesFilters(&row, params.Target, instanceSet, groupSet, classSet) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// rowMatchesFilters applies the three independent list dimensions to one row.
func rowMatchesFilters(row *classday.Row, target classday.Target, instanceSet, groupSet map[int64]bool, classSet map[string]struct{}) bool {
	// InstanceIDs are a slot-list filter only; they are ignored for pickup
	// cohorts (whose rows carry InstanceID 0), where a non-empty selection
	// would otherwise drop every row (#1565 review).
	if target == classday.TargetSlots && len(instanceSet) > 0 && !instanceSet[row.InstanceID] {
		return false
	}
	if len(groupSet) > 0 && (row.GroupID == nil || !groupSet[*row.GroupID]) {
		return false
	}
	if len(classSet) > 0 {
		if _, ok := classSet[row.SchoolClass]; !ok {
			return false
		}
	}
	return true
}

func int64StructSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func int64Set(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// mergedEntry is one (slot, student) pair before person/group enrichment.
type mergedEntry struct {
	StudentID        int64
	InstanceID       int64 // 0 for pickup-based cohorts
	SlotLabel        string
	RoomName         string // planned room of the slot; "" for pickup cohorts
	PickupTime       string
	Planned          bool
	Present          bool
	PlannedStatus    string  // schedule.instance_students.status, "" for pickup cohorts
	PlannedSubstatus *string // schedule.instance_students.substatus, nil for pickup cohorts
}
