package students

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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
		renderError(w, r, ErrorForbidden(errors.New("read access required to view enrollment extra fields")))
		return
	}
	if rs.EnrollmentDecision == nil || rs.EnrollmentFormSchema == nil {
		renderError(w, r, ErrorInternalServer(errors.New("enrollment services not configured")))
		return
	}

	summaries, err := rs.EnrollmentDecision.ListByStudent(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	out := rs.toStudentEnrollmentExtraFieldGroups(r, student.ID, summaries)
	common.Respond(w, r, http.StatusOK, out, "Student enrollment extra fields retrieved")
}

func (rs *Resource) toStudentEnrollmentExtraFieldGroups(r *http.Request, studentID int64, summaries []*enrollmentService.RequestSummary) []StudentEnrollmentExtraFieldGroup {
	out := make([]StudentEnrollmentExtraFieldGroup, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.Request == nil || summary.Request.SchemaID == nil {
			continue
		}
		child := linkedSummaryChild(summary, studentID)
		if child == nil {
			continue
		}
		fields := rs.studentEnrollmentExtraFieldsForChild(r, *summary.Request.SchemaID, child.CustomData)
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
	return out
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

func (rs *Resource) studentEnrollmentExtraFieldsForChild(r *http.Request, schemaID int64, customData map[string]any) []StudentEnrollmentExtraField {
	if len(customData) == 0 {
		return nil
	}
	schema, err := rs.EnrollmentFormSchema.GetByID(r.Context(), schemaID)
	if err != nil || schema == nil {
		if err != nil && rs.Logger != nil {
			rs.Logger.Warn("failed to load enrollment schema for student extra fields",
				"schema_id", schemaID,
				"error", err.Error(),
			)
		}
		return nil
	}

	out := make([]StudentEnrollmentExtraField, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if !field.AppliesToCh || field.Type == enrollmentModels.FormFieldInfo {
			continue
		}
		value, ok := customData[field.Key]
		if !ok || value == nil {
			continue
		}
		options := make([]StudentEnrollmentExtraFieldOption, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, StudentEnrollmentExtraFieldOption{
				Label: option.Label,
				Value: option.Value,
			})
		}
		out = append(out, StudentEnrollmentExtraField{
			Key:     field.Key,
			Label:   field.Label,
			Type:    string(field.Type),
			Target:  field.Target,
			Options: options,
			Value:   value,
		})
	}
	return out
}

// GetStudentEnrollmentExtraFieldsHandler returns the handler for getting
// per-child enrollment extra fields linked to a student.
func (rs *Resource) GetStudentEnrollmentExtraFieldsHandler() http.HandlerFunc {
	return rs.getStudentEnrollmentExtraFields
}
