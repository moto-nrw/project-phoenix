package activities

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
)

// categoryIDParam is the URL parameter of the category Stammdaten routes. It
// is deliberately not "id": the activities router already binds "{id}" to an
// activity group, and reusing the name across sibling patterns makes the
// chi-resolved value ambiguous to read.
const categoryIDParam = "categoryId"

const msgInvalidCategoryID = "Ungültige Kategorie-ID"

// maxCategoryNameLength keeps a Kategorie label short enough to render in the
// planner dropdowns and Termin badges. The column itself is unbounded TEXT.
const maxCategoryNameLength = 60

const maxCategoryDescriptionLength = 255

// CategoryRequest is the create/update payload for an activity category.
// is_system is deliberately absent: schools must not be able to mint system
// categories, and the existing ones stay read-only (#2131).
type CategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// Bind validates and normalizes the category payload.
func (req *CategoryRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Color = strings.TrimSpace(req.Color)

	return validation.ValidateStruct(req,
		validation.Field(&req.Name,
			validation.Required.Error("Name ist erforderlich"),
			validation.Length(1, maxCategoryNameLength).Error("Name darf höchstens 60 Zeichen lang sein"),
		),
		validation.Field(&req.Description,
			validation.Length(0, maxCategoryDescriptionLength).Error("Beschreibung darf höchstens 255 Zeichen lang sein"),
		),
	)
}

func (req *CategoryRequest) toInput() activitiesSvc.CategoryInput {
	return activitiesSvc.CategoryInput{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
	}
}

// listCategories handles listing the tenant's activity categories.
//
// Three filters, all opt-in so existing consumers (planner dropdowns, activity
// forms) keep seeing exactly the pickable set at the price of one query:
//   - include_system=true keeps the auto-provisioned Schulhof/WC categories
//     (issue #923; the IoT provisioning flows need them, staff pickers do not)
//   - include_archived=true keeps retired ones, for the Stammdaten screen that
//     offers restoring them (#2131)
//   - with_usage=true reports usage_count, which costs an extra tenant-wide
//     aggregate over activities.groups. Only the Stammdaten screen renders it;
//     the pickers would pay for a field they never read.
func (rs *Resource) listCategories(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	includeSystem := query.Get("include_system") == "true"
	includeArchived := query.Get("include_archived") == "true"
	withUsage := query.Get("with_usage") == "true"

	categories, err := rs.ActivityService.ListCategories(r.Context())
	if err != nil {
		common.RenderError(w, r, categoryErrorRenderer(err))
		return
	}

	var usage map[int64]int
	if withUsage {
		if usage, err = rs.ActivityService.CategoryUsageCounts(r.Context()); err != nil {
			common.RenderError(w, r, categoryErrorRenderer(err))
			return
		}
	}

	responses := make([]CategoryResponse, 0, len(categories))
	for _, category := range categories {
		if category.IsSystem && !includeSystem {
			continue
		}
		if category.IsArchived() && !includeArchived {
			continue
		}
		response := newCategoryResponse(category)
		if withUsage {
			count := usage[category.ID]
			response.UsageCount = &count
		}
		responses = append(responses, response)
	}

	common.Respond(w, r, http.StatusOK, responses, "Categories retrieved successfully")
}

// createCategory creates a new school-owned activity category.
func (rs *Resource) createCategory(ctx context.Context, req *CategoryRequest) (CategoryResponse, error) {
	category := &activityModels.Category{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
	}

	created, err := rs.ActivityService.CreateCategory(ctx, category)
	if err != nil {
		return CategoryResponse{}, err
	}

	return newCategoryResponse(created), nil
}

// updateCategory renames a category and updates its description/color.
func (rs *Resource) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseIDParam(r, categoryIDParam)
	if err != nil {
		//nolint:staticcheck // ST1005: user-facing German message rendered verbatim
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(msgInvalidCategoryID)))
		return
	}

	req := &CategoryRequest{}
	if bindErr := render.Bind(r, req); bindErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(bindErr))
		return
	}

	updated, updateErr := rs.ActivityService.UpdateCategory(r.Context(), id, req.toInput())
	if updateErr != nil {
		common.RenderError(w, r, categoryErrorRenderer(updateErr))
		return
	}

	common.Respond(w, r, http.StatusOK, newCategoryResponse(updated), "Category updated successfully")
}

// archiveCategory retires a category. Nothing is deleted — existing Termine
// and Aktivitäten keep their category and stay valid.
func (rs *Resource) archiveCategory(ctx context.Context, id int64) (CategoryResponse, error) {
	archived, err := rs.ActivityService.ArchiveCategory(ctx, id)
	if err != nil {
		return CategoryResponse{}, err
	}
	return newCategoryResponse(archived), nil
}

// restoreCategory brings an archived category back into the pickers.
func (rs *Resource) restoreCategory(ctx context.Context, id int64) (CategoryResponse, error) {
	restored, err := rs.ActivityService.RestoreCategory(ctx, id)
	if err != nil {
		return CategoryResponse{}, err
	}
	return newCategoryResponse(restored), nil
}
