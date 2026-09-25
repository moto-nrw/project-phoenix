package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// maxReportRows caps the students and children one report covers;
// maxExportRequests caps the requests.
const (
	maxReportRows     = 10000
	maxExportRequests = 5000
)

// The reports read the parent's answers, so they decode the stored request
// and child answers the same way the retained decision flow does until that
// flow moves into the owner (#3564, #3565).

// reportChild is a request child with its decoded answers.
type reportChild struct {
	ID                int64
	RequestID         int64
	FirstName         string
	LastName          string
	DateOfBirth       calendar.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	CustomData        map[string]any
	Status            string
	CreatedStudentID  *int64
	MatchedStudentID  *int64
}

func listReportRequests(ctx context.Context, owner ReportRequests, filters enrollment.RequestListFilters) ([]*enrollmentModels.Request, error) {
	values, err := owner.AdminRequests(ctx, filters)
	if err != nil {
		return nil, err
	}
	var result []*enrollmentModels.Request
	for _, value := range values {
		converted, err := reportRequestValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func reportRequestValue(r *enrollment.Request) (*enrollmentModels.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollmentModels.Request{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		SchemaID: r.SchemaID, PhaseID: r.PhaseID,
		GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName,
		GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID,
		SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires,
		SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode,
	}
	for _, field := range []struct {
		raw  json.RawMessage
		into any
	}{
		{r.ConsentFlags, &result.ConsentFlags},
		{r.LegalBlocksSnapshot, &result.LegalBlocksSnapshot},
		{r.CustomData, &result.CustomData},
		{r.SourceMetadata, &result.SourceMetadata},
	} {
		if len(field.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(field.raw, field.into); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func listReportChildren(ctx context.Context, owner ReportChildren, requestIDs []int64) ([]*reportChild, error) {
	values, err := owner.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	var result []*reportChild
	for _, value := range values {
		converted, err := reportChildValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func reportChildValue(r *enrollment.RequestChild) (*reportChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &reportChild{
		ID: r.ID, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName,
		TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, Status: r.Status,
		CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID,
	}
	dob, err := calendar.ParseDate(string(r.DateOfBirth))
	if err != nil {
		return nil, err
	}
	result.DateOfBirth = dob
	if r.ActivateOn != nil {
		if _, err := calendar.ParseDate(string(*r.ActivateOn)); err != nil {
			return nil, err
		}
	}
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// loadReportSchemas loads the form schema of every request that has one.
func loadReportSchemas(ctx context.Context, owner ReportSchemas, requests []*enrollmentModels.Request) (map[int64]*enrollment.FormSchema, error) {
	ids := make([]int64, 0, len(requests))
	seen := make(map[int64]bool)
	for _, request := range requests {
		if request != nil && request.SchemaID != nil && !seen[*request.SchemaID] {
			seen[*request.SchemaID] = true
			ids = append(ids, *request.SchemaID)
		}
	}
	result := make(map[int64]*enrollment.FormSchema, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if owner == nil {
		return nil, fmt.Errorf("form schema repo not configured")
	}
	rows, err := owner.Schemas(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, schema := range rows {
		result[schema.ID] = enrollment.CopyFormSchema(schema)
	}
	for _, id := range ids {
		if result[id] == nil {
			return nil, fmt.Errorf("form schema %d not found", id)
		}
	}
	return result, nil
}

// reportFieldValue pulls the submission value for a field. Guardian-level
// fields live on the request's answers, per-child fields on the child's.
func reportFieldValue(req *enrollmentModels.Request, child *reportChild, field enrollment.FormField) any {
	if field.AppliesToCh {
		if child == nil || child.CustomData == nil {
			return nil
		}
		return child.CustomData[field.Key]
	}
	if req == nil || req.CustomData == nil {
		return nil
	}
	return req.CustomData[field.Key]
}

// careUsagePickupByDay reads the child's pickup weekday schedule answer.
func careUsagePickupByDay(req *enrollmentModels.Request, child *reportChild, schemas map[int64]*enrollment.FormSchema) (map[string]string, error) {
	return careUsageScheduleByTarget(req, child, schemas, enrollment.TargetSchedulePickup)
}

func careUsageScheduleByTarget(req *enrollmentModels.Request, child *reportChild, schemas map[int64]*enrollment.FormSchema, target string) (map[string]string, error) {
	out := map[string]string{}
	if req == nil || req.SchemaID == nil || child == nil {
		return out, nil
	}
	schema := schemas[*req.SchemaID]
	if schema == nil {
		return out, nil
	}
	for _, field := range careUsageScheduleFields(schema, target) {
		raw := reportFieldValue(req, child, field)
		if raw == nil {
			continue
		}
		schedule, err := decodeCareUsageWeekdaySchedule(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.Key, err)
		}
		for day, pickupTime := range schedule {
			pickupTime = strings.TrimSpace(pickupTime)
			if pickupTime == "" {
				continue
			}
			out[day] = pickupTime
		}
	}
	return out, nil
}

func careUsageScheduleFields(schema *enrollment.FormSchema, target string) []enrollment.FormField {
	if schema == nil {
		return nil
	}
	fields := make([]enrollment.FormField, 0)
	for _, field := range schema.Fields {
		if field.Target != target || field.Type != enrollment.FormFieldWeekdaySchedule {
			continue
		}
		fields = append(fields, field)
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].SortOrder < fields[j].SortOrder
	})
	return fields
}

func decodeCareUsageWeekdaySchedule(raw any) (enrollment.WeekdaySchedule, error) {
	var schedule enrollment.WeekdaySchedule
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &schedule); err != nil {
		return nil, err
	}
	if schedule == nil {
		schedule = enrollment.WeekdaySchedule{}
	}
	if err := schedule.Validate(); err != nil {
		return nil, err
	}
	return schedule, nil
}

// decodeStructured marshals raw → JSON → out so the stored answer reads into
// the typed answer shapes of the form schema.
func decodeStructured(raw any, out any) error {
	bs, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, out)
}

func decodeDepartureDays(raw any) (departure.DepartureDays, error) {
	var modes enrollment.WeekdayMode
	if err := decodeStructured(raw, &modes); err != nil {
		return nil, fmt.Errorf("decode weekday_mode: %w", err)
	}
	if err := modes.Validate(); err != nil {
		return nil, err
	}
	out := departure.DepartureDays{}
	for day, mode := range modes {
		switch mode {
		case enrollment.WeekdayModeBus:
			out[day] = departure.DepartureBus
		case enrollment.WeekdayModePickup:
			out[day] = departure.DeparturePickup
		case enrollment.WeekdayModeAccompanied:
			out[day] = departure.DepartureAccompanied
		}
	}
	return out.Normalize(), nil
}

// weekdayDepartureModes maps the form's weekday mode values onto the
// departure contract.
var weekdayDepartureModes = map[string]departure.DepartureMode{
	enrollment.WeekdayModeAlone:       departure.DepartureAlone,
	enrollment.WeekdayModeBus:         departure.DepartureBus,
	enrollment.WeekdayModePickup:      departure.DeparturePickup,
	enrollment.WeekdayModeAccompanied: departure.DepartureAccompanied,
}

func decodeAllowedDepartureModes(raw any) (departure.AllowedDepartureModes, error) {
	var modes enrollment.WeekdayMultiMode
	if err := decodeStructured(raw, &modes); err != nil {
		return nil, fmt.Errorf("decode weekday_multi_mode: %w", err)
	}
	if err := modes.Validate(); err != nil {
		return nil, err
	}
	out := departure.AllowedDepartureModes{}
	for day, rawModes := range modes {
		for _, mode := range rawModes {
			if mapped, ok := weekdayDepartureModes[mode]; ok {
				out[day] = append(out[day], mapped)
			}
		}
	}
	return out.Normalize(), nil
}

func decodeBusDays(raw any) (departure.BusDays, error) {
	if enabled, ok := raw.(bool); ok {
		return departure.BusDaysFromLegacyFlag(enabled), nil
	}
	return decodeWeekdayBooleanDays[departure.BusDays](raw, departure.BusDayOrder)
}

// decodeWeekdayBooleanDays decodes a weekday_boolean map and projects it
// onto the given day order.
func decodeWeekdayBooleanDays[M ~map[string]bool](raw any, order []string) (M, error) {
	var days enrollment.WeekdayBoolean
	if err := decodeStructured(raw, &days); err != nil {
		return nil, fmt.Errorf("decode weekday_boolean: %w", err)
	}
	if err := days.Validate(); err != nil {
		return nil, err
	}
	out := M{}
	for _, day := range order {
		if days[day] {
			out[day] = true
		}
	}
	return out, nil
}

// decodePickupDays reads a pickup answer. A pending pre-migration submission
// stored the select option value ("picked_up" / "alone"), not the German
// label, so "picked_up" maps to all five weekdays explicitly; anything else
// is read by the owner's legacy status helper.
func decodePickupDays(raw any) (departure.PickupDays, error) {
	if str, ok := raw.(string); ok {
		if strings.TrimSpace(str) == "picked_up" {
			return departure.PickupDaysFromLegacyStatus(departure.PickupStatusPickedUp), nil
		}
		return departure.PickupDaysFromLegacyStatus(str), nil
	}
	return decodeWeekdayBooleanDays[departure.PickupDays](raw, departure.PickupDayOrder)
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
