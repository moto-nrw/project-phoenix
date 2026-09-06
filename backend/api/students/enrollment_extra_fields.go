package students

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
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

	summaries, err := rs.EnrollmentDecision.ListByStudent(r.Context(), student.ID)
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

func (rs *Resource) toStudentEnrollmentExtraFieldGroups(r *http.Request, studentID int64, summaries []*enrollmentService.RequestSummary) ([]StudentEnrollmentExtraFieldGroup, error) {
	out := make([]StudentEnrollmentExtraFieldGroup, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.Request == nil || summary.Request.SchemaID == nil {
			continue
		}
		child := linkedSummaryChild(summary, studentID)
		if child == nil {
			continue
		}
		fields, err := rs.studentEnrollmentExtraFieldsForChild(r, *summary.Request.SchemaID, child.CustomData)
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

func linkedSummaryChild(summary *enrollmentService.RequestSummary, studentID int64) *enrollmentModels.RequestChild {
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

func (rs *Resource) studentEnrollmentExtraFieldsForChild(r *http.Request, schemaID int64, customData map[string]any) ([]StudentEnrollmentExtraField, error) {
	if len(customData) == 0 {
		return nil, nil
	}
	schema, err := rs.EnrollmentFormSchema.GetByID(r.Context(), schemaID)
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
