package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	enrollmentCapability "github.com/moto-ogs/project-phoenix/modules/enrollment"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
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

func (rs *StudentResource) getStudentEnrollmentExtraFields(w http.ResponseWriter, r *http.Request) {
	studentID, ok, canRead := rs.StudentReadAccess(w, r)
	if !ok {
		return
	}
	if !canRead {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("read access required to view enrollment extra fields")))
		return
	}
	if rs.EnrollmentDecision == nil || rs.EnrollmentFormSchema == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment services not configured")))
		return
	}

	summaries, err := rs.EnrollmentDecision.StudentExtraFieldRequests(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load enrollment extra fields", err))
		return
	}

	out, err := rs.toStudentEnrollmentExtraFieldGroups(r, studentID, summaries)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load enrollment extra fields", err))
		return
	}
	common.Respond(w, r, http.StatusOK, out, "Student enrollment extra fields retrieved")
}

func (rs *StudentResource) toStudentEnrollmentExtraFieldGroups(r *http.Request, studentID int64, summaries []*enrollmentCapability.DecisionSummary) ([]StudentEnrollmentExtraFieldGroup, error) {
	prepared, ids := prepareStudentExtraFields(summaries, studentID)
	out := make([]StudentEnrollmentExtraFieldGroup, 0, len(prepared))
	var versions map[int64]enrollmentCapability.SchemaVersionResult
	var readErr error
	for _, item := range prepared {
		if len(item.customData) == 0 {
			continue
		}
		if versions == nil {
			versions, readErr = rs.loadStudentExtraFieldSchemas(r.Context(), ids)
		}
		summary := item.summary
		fields, err := rs.studentEnrollmentExtraFieldsForChild(*summary.Request.SchemaID, item.customData, versions[*summary.Request.SchemaID], readErr)
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

func (rs *StudentResource) loadStudentExtraFieldSchemas(ctx context.Context, ids []int64) (map[int64]enrollmentCapability.SchemaVersionResult, error) {
	results, err := rs.EnrollmentFormSchema.StudentExtraFieldSchemas(ctx, ids)
	versions := make(map[int64]enrollmentCapability.SchemaVersionResult, len(results))
	for _, result := range results {
		versions[result.ID] = result
	}
	return versions, err
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

func (rs *StudentResource) studentEnrollmentExtraFieldsForChild(schemaID int64, customData map[string]any, result enrollmentCapability.SchemaVersionResult, readErr error) ([]StudentEnrollmentExtraField, error) {
	err := result.Err
	if err == nil {
		err = readErr
	}
	if err != nil {
		rs.logEnrollmentSchemaLoadFailure(schemaID, err)
		return nil, fmt.Errorf("load enrollment schema %d for student extra fields: %w", schemaID, err)
	}
	if result.Schema == nil {
		err := fmt.Errorf("enrollment schema %d not found", schemaID)
		rs.logEnrollmentSchemaLoadFailure(schemaID, err)
		return nil, err
	}
	return enrollmentExtraFieldsFromSchema(result.Schema.Fields, customData), nil
}

func (rs *StudentResource) logEnrollmentSchemaLoadFailure(schemaID int64, err error) {
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

// StudentResource owns the linked enrollment answers served under /students.
type StudentResource struct {
	EnrollmentDecision   enrollmentCapability.Decisions
	EnrollmentFormSchema enrollmentCapability.FormSchemaAdministration
	StudentReadAccess    func(http.ResponseWriter, *http.Request) (studentID int64, found, canRead bool)
	Logger               *slog.Logger
}

func (rs *StudentResource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/enrollment-extra-fields", rs.getStudentEnrollmentExtraFields)
}
