package students

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/api/common"
)

type StudentEnrollmentExtraFieldGroup struct {
	RequestID   string                        `json:"request_id"`
	PhaseName   string                        `json:"phase_name"`
	SubmittedAt time.Time                     `json:"submitted_at"`
	Fields      []StudentEnrollmentExtraField `json:"fields"`
}

type StudentEnrollmentExtraField struct {
	Key     string                              `json:"key"`
	Label   string                              `json:"label"`
	Type    string                              `json:"type"`
	Target  string                              `json:"target,omitempty"`
	Options []StudentEnrollmentExtraFieldOption `json:"options,omitempty"`
	Value   any                                 `json:"value"`
}

type StudentEnrollmentExtraFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func (rs *Resource) getStudentEnrollmentExtraFields(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("read access required to view enrollment extra fields")))
		return
	}
	if rs.EnrollmentDecision == nil || rs.EnrollmentFormSchema == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("enrollment services not configured")))
		return
	}

	summaries, err := rs.EnrollmentDecision.StudentDecisionRequests(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load enrollment extra fields", err))
		return
	}

	out, err := rs.toStudentEnrollmentExtraFieldGroups(r, student.ID, summaries)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load enrollment extra fields", err))
		return
	}
	common.Respond(w, r, http.StatusOK, out, "Student enrollment extra fields retrieved")
}

func (rs *Resource) toStudentEnrollmentExtraFieldGroups(r *http.Request, studentID int64, summaries []*enrollmentCapability.DecisionSummary) ([]StudentEnrollmentExtraFieldGroup, error) {
	out := make([]StudentEnrollmentExtraFieldGroup, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.Request == nil || summary.Request.SchemaID == nil {
			continue
		}
		child := linkedSummaryChild(summary, studentID)
		if child == nil {
			continue
		}
		customData, err := decodeEnrollmentCustomData(child.CustomData)
		if err != nil {
			return nil, err
		}
		fields, err := rs.studentEnrollmentExtraFieldsForChild(r, *summary.Request.SchemaID, customData)
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			continue
		}
		group := StudentEnrollmentExtraFieldGroup{
			RequestID:   strconv.FormatInt(summary.Request.ID, 10),
			SubmittedAt: summary.Request.SubmittedAt,
			Fields:      fields,
		}
		if summary.Phase != nil {
			group.PhaseName = summary.Phase.Name
		}
		out = append(out, group)
	}
	return out, nil
}

func linkedSummaryChild(summary *enrollmentCapability.DecisionSummary, studentID int64) *enrollmentCapability.RequestChild {
	for _, child := range summary.Children {
		if child == nil || child.CreatedStudentID == nil {
			continue
		}
		if *child.CreatedStudentID == studentID {
			return child
		}
	}
	return nil
}

// decodeEnrollmentCustomData decodes the answers a family gave for a child,
// which the Enrollment owner hands over as raw JSON.
func decodeEnrollmentCustomData(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var customData map[string]any
	if err := json.Unmarshal(raw, &customData); err != nil {
		return nil, fmt.Errorf("decode enrollment child custom data: %w", err)
	}
	return customData, nil
}

func (rs *Resource) studentEnrollmentExtraFieldsForChild(r *http.Request, schemaID int64, customData map[string]any) ([]StudentEnrollmentExtraField, error) {
	if len(customData) == 0 {
		return nil, nil
	}
	schema, err := rs.EnrollmentFormSchema.SchemaVersion(r.Context(), schemaID)
	if err != nil {
		rs.logEnrollmentSchemaLoadFailure(schemaID, err)
		return nil, fmt.Errorf("load enrollment schema %d for student extra fields: %w", schemaID, err)
	}
	if schema == nil {
		err := fmt.Errorf("enrollment schema %d not found", schemaID)
		rs.logEnrollmentSchemaLoadFailure(schemaID, err)
		return nil, err
	}
	return enrollmentExtraFieldsFromSchema(schema.Fields, customData), nil
}

func (rs *Resource) logEnrollmentSchemaLoadFailure(schemaID int64, err error) {
	if rs.Logger == nil {
		return
	}
	rs.Logger.Warn("failed to load enrollment schema for student extra fields",
		"schema_id", schemaID,
		"error", err.Error(),
	)
}

// enrollmentExtraFieldsFromSchema maps the child-applicable, populated fields of
// a form schema to their response shape. Info blocks and fields not applicable
// to children are skipped, as are keys absent from (or nil in) customData.
func enrollmentExtraFieldsFromSchema(fields []enrollmentCapability.FormField, customData map[string]any) []StudentEnrollmentExtraField {
	out := make([]StudentEnrollmentExtraField, 0, len(fields))
	for _, field := range fields {
		if !field.AppliesToCh || field.Type == enrollmentCapability.FormFieldInfo {
			continue
		}
		value, ok := customData[field.Key]
		if !ok || value == nil {
			continue
		}
		out = append(out, StudentEnrollmentExtraField{
			Key:     field.Key,
			Label:   field.Label,
			Type:    string(field.Type),
			Target:  field.Target,
			Options: enrollmentExtraFieldOptions(field.Options),
			Value:   value,
		})
	}
	return out
}

func enrollmentExtraFieldOptions(fieldOptions []enrollmentCapability.FormFieldOption) []StudentEnrollmentExtraFieldOption {
	options := make([]StudentEnrollmentExtraFieldOption, 0, len(fieldOptions))
	for _, option := range fieldOptions {
		options = append(options, StudentEnrollmentExtraFieldOption{
			Label: option.Label,
			Value: option.Value,
		})
	}
	return options
}
