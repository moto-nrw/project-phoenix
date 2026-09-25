package application

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A change request stores the request as it was (base) and as the family
// wants it (proposed), both in the submission's JSON shape. The approval
// compares the base to the current state and applies the proposal.

// currentSnapshot is the request as stored now, with the bookings in force
// today rather than at the service start: an approved dated change must not
// be written back over by a stale selection.
func (s *ChangeRequests) currentSnapshot(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) (map[string]any, error) {
	guardians, err := s.deps.Guardians.RequestGuardians(ctx, []int64{req.ID})
	if err != nil {
		return nil, fmt.Errorf("change request: list guardians: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	phase, err := s.intake.intakePhase(ctx, req.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("change request: load phase for current snapshot: %w", err)
	}
	if phase == nil {
		return nil, enrollment.ErrEnrollmentDisabled
	}
	links, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, s.deps.Children, childIDs, s.intake.currentOfferingSelectionDate(phase))
	if err != nil {
		return nil, fmt.Errorf("change request: list child offerings: %w", err)
	}
	linksByChild := make(map[int64][]*enrollment.RequestChildOfferingRecord, len(children))
	for _, link := range links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}
	return persistedSnapshot(req, children, guardians, linksByChild), nil
}

func submitSnapshot(req SubmitRequest) map[string]any {
	children := make([]any, 0, len(req.Children))
	for _, child := range req.Children {
		children = append(children, submitChildSnapshot(child))
	}
	guardians := make([]any, 0, len(req.AdditionalGuardians))
	for _, guardian := range req.AdditionalGuardians {
		guardians = append(guardians, map[string]any{
			"first_name": guardian.FirstName,
			"last_name":  guardian.LastName,
			"email":      ptrStringValue(guardian.Email),
			"phone":      ptrStringValue(guardian.Phone),
		})
	}
	return map[string]any{
		"phase_id":             strconv.FormatInt(req.PhaseID, 10),
		"guardian_first_name":  req.GuardianFirstName,
		"guardian_last_name":   req.GuardianLastName,
		"guardian_email":       req.GuardianEmail,
		"guardian_phone":       ptrStringValue(req.GuardianPhone),
		"consent_flags":        req.ConsentFlags,
		"custom_data":          req.CustomData,
		"additional_guardians": guardians,
		"children":             children,
	}
}

func submitChildSnapshot(child SubmitChild) map[string]any {
	offeringIDs := make([]any, 0, len(child.OfferingIDs))
	for _, id := range child.OfferingIDs {
		offeringIDs = append(offeringIDs, strconv.FormatInt(id, 10))
	}
	offeringDays := make([]any, 0, len(child.OfferingDays))
	for _, row := range child.OfferingDays {
		offeringDays = append(offeringDays, map[string]any{
			"offering_id":   strconv.FormatInt(row.OfferingID, 10),
			"selected_days": copyDays(row.SelectedDays),
		})
	}
	row := map[string]any{
		"first_name":         child.FirstName,
		"last_name":          child.LastName,
		"date_of_birth":      child.DateOfBirth.String(),
		"target_grade_level": child.TargetGradeLevel,
		"custom_data":        child.CustomData,
		"offering_ids":       offeringIDs,
		"offering_days":      offeringDays,
	}
	// Snapshots stored before #1833 have no target_school_class key, so an
	// unset class stays absent too; a recomputed "null" would otherwise read
	// as a conflict against every older pending change request.
	if class := trimmedOptionalString(child.TargetSchoolClass); class != "" {
		row["target_school_class"] = class
	}
	if child.ID > 0 {
		row["id"] = strconv.FormatInt(child.ID, 10)
	}
	return row
}

func persistedSnapshot(req *enrollmentModels.Request, children []*RequestChild, guardians []*enrollment.RequestGuardian, linksByChild map[int64][]*enrollment.RequestChildOfferingRecord) map[string]any {
	submit := SubmitRequest{
		TenantID: req.TenantID, PhaseID: req.PhaseID, GuardianFirstName: req.GuardianFirstName,
		GuardianLastName: req.GuardianLastName, GuardianEmail: req.GuardianEmail, GuardianPhone: req.GuardianPhone,
		ConsentFlags: req.ConsentFlags, CustomData: req.CustomData,
	}
	for _, guardian := range guardians {
		submit.AdditionalGuardians = append(submit.AdditionalGuardians, SubmitGuardian{
			FirstName: guardian.FirstName, LastName: guardian.LastName, Email: guardian.Email, Phone: guardian.Phone,
		})
	}
	for _, child := range children {
		next := SubmitChild{
			FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
			TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass, CustomData: child.CustomData,
		}
		for _, link := range linksByChild[child.ID] {
			next.OfferingIDs = append(next.OfferingIDs, link.CareOfferingID)
			if len(link.SelectedDays) > 0 {
				next.OfferingDays = append(next.OfferingDays, SubmitOfferingDays{OfferingID: link.CareOfferingID, SelectedDays: copyDays(link.SelectedDays)})
			}
		}
		submit.Children = append(submit.Children, next)
	}
	snapshot := submitSnapshot(submit)
	rows := snapshot["children"].([]any)
	for i, child := range children {
		if i >= len(rows) {
			continue
		}
		if row, ok := rows[i].(map[string]any); ok {
			row["id"] = strconv.FormatInt(child.ID, 10)
			row["status"] = child.Status
		}
	}
	return snapshot
}

// ensureTakenOverChildrenUnchanged refuses a proposal that would alter a
// child already taken over into care (ADR 0003). Both snapshots are aligned
// with children by index; id and status are bookkeeping the proposal lacks.
func ensureTakenOverChildrenUnchanged(children []*RequestChild, base, proposed map[string]any) error {
	baseRows := sliceFromAny(base["children"])
	proposedRows := sliceFromAny(proposed["children"])
	for i, child := range children {
		if !childTakenOver(child) {
			continue
		}
		if i >= len(baseRows) || i >= len(proposedRows) {
			return enrollment.ErrChangeRequestChildLocked
		}
		baseRow := mapFromAny(baseRows[i])
		proposedRow := mapFromAny(proposedRows[i])
		// A proposal naming another child at this position is refused rather
		// than compared, or the wrong two rows would clear the lock.
		if id := stringFromAny(proposedRow["id"]); id != "" && id != stringFromAny(baseRow["id"]) {
			return enrollment.ErrChangeRequestChildLocked
		}
		if int16PtrFromAny(proposedRow["target_grade_level"]) == nil {
			proposedRow["target_grade_level"] = baseRow["target_grade_level"]
		}
		if !jsonEqual(comparableChildSnapshot(baseRow), comparableChildSnapshot(proposedRow)) {
			return enrollment.ErrChangeRequestChildLocked
		}
	}
	return nil
}

func comparableChildSnapshot(row map[string]any) map[string]any {
	row = maps.Clone(row)
	delete(row, "id")
	delete(row, "status")
	if len(mapFromAny(row["custom_data"])) == 0 {
		row["custom_data"] = map[string]any{}
	}
	values := sliceFromAny(row["offering_ids"])
	sort.Slice(values, func(i, j int) bool { return stringFromAny(values[i]) < stringFromAny(values[j]) })
	row["offering_ids"] = values
	for _, raw := range sliceFromAny(row["offering_days"]) {
		day := mapFromAny(raw)
		days := sliceFromAny(day["selected_days"])
		sort.Slice(days, func(i, j int) bool { return stringFromAny(days[i]) < stringFromAny(days[j]) })
		day["selected_days"] = days
	}
	return row
}

func snapshotDiff(base, proposed map[string]any) map[string]any {
	changed := make([]string, 0)
	keys := make(map[string]bool)
	for key := range base {
		keys[key] = true
	}
	for key := range proposed {
		keys[key] = true
	}
	for key := range keys {
		if !jsonEqual(base[key], proposed[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return map[string]any{"changed": changed}
}

// childSnapshotChanged reports whether a child's own data changed between
// the snapshots, status aside.
func childSnapshotChanged(base, proposed map[string]any, childID int64) bool {
	baseChild := snapshotChildByID(base, childID)
	proposedChild := snapshotChildByID(proposed, childID)
	if baseChild == nil || proposedChild == nil {
		return false
	}
	return !jsonEqual(snapshotWithoutStatus(baseChild), snapshotWithoutStatus(proposedChild))
}

func snapshotWithoutStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		if key != "status" {
			out[key] = value
		}
	}
	return out
}

func snapshotToSubmitRequest(snapshot map[string]any) (SubmitRequest, error) {
	out := SubmitRequest{
		PhaseID:           int64FromAny(snapshot["phase_id"]),
		GuardianFirstName: stringFromAny(snapshot["guardian_first_name"]),
		GuardianLastName:  stringFromAny(snapshot["guardian_last_name"]),
		GuardianEmail:     stringFromAny(snapshot["guardian_email"]),
		GuardianPhone:     optionalStringFromAny(snapshot["guardian_phone"]),
		ConsentFlags:      mapFromAny(snapshot["consent_flags"]),
		CustomData:        mapFromAny(snapshot["custom_data"]),
	}
	for _, raw := range sliceFromAny(snapshot["additional_guardians"]) {
		row := mapFromAny(raw)
		out.AdditionalGuardians = append(out.AdditionalGuardians, SubmitGuardian{
			FirstName: stringFromAny(row["first_name"]), LastName: stringFromAny(row["last_name"]),
			Email: optionalStringFromAny(row["email"]), Phone: optionalStringFromAny(row["phone"]),
		})
	}
	for i, raw := range sliceFromAny(snapshot["children"]) {
		child, err := snapshotChild(i, mapFromAny(raw))
		if err != nil {
			return out, err
		}
		out.Children = append(out.Children, child)
	}
	return out, nil
}

func snapshotChild(i int, row map[string]any) (SubmitChild, error) {
	dob, err := calendar.ParseDate(stringFromAny(row["date_of_birth"]))
	if err != nil {
		return SubmitChild{}, fmt.Errorf("%w: child %d date_of_birth", enrollment.ErrChangeRequestInvalidData, i)
	}
	child := SubmitChild{
		ID: int64FromAny(row["id"]), FirstName: stringFromAny(row["first_name"]), LastName: stringFromAny(row["last_name"]),
		DateOfBirth: dob, TargetGradeLevel: int16PtrFromAny(row["target_grade_level"]),
		TargetSchoolClass: optionalStringFromAny(row["target_school_class"]), CustomData: mapFromAny(row["custom_data"]),
	}
	for _, rawID := range sliceFromAny(row["offering_ids"]) {
		if id := int64FromAny(rawID); id > 0 {
			child.OfferingIDs = append(child.OfferingIDs, id)
		}
	}
	for _, rawDay := range sliceFromAny(row["offering_days"]) {
		day := mapFromAny(rawDay)
		if id := int64FromAny(day["offering_id"]); id > 0 {
			child.OfferingDays = append(child.OfferingDays, SubmitOfferingDays{OfferingID: id, SelectedDays: stringSliceFromAny(day["selected_days"])})
		}
	}
	return child, nil
}

func ptrStringValue(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func optionalStringFromAny(v any) *string {
	s := strings.TrimSpace(stringFromAny(v))
	if s == "" {
		return nil
	}
	return &s
}

func int16PtrFromAny(v any) *int16 {
	if value, ok := v.(*int16); ok {
		return value
	}
	n := int64FromAny(v)
	if n == 0 {
		return nil
	}
	out := int16(n)
	return &out
}

func stringSliceFromAny(v any) []string {
	out := make([]string, 0)
	for _, raw := range sliceFromAny(v) {
		if s := stringFromAny(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func trimmedOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
