package timetablehttp

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// templateRefusalRules classify the refusals every Regeltermin write shares
// (create, update, split, conversion) with their own code (#2516). The
// statuses stay as they were; the field marks the editor input involved.
var templateRefusalRules = []common.ErrorRule{
	{Target: timetableModule.ErrCategoryNotAssignable, Render: func(error) render.Renderer {
		return invalidOnField(common.CodeTimetableTemplateCategoryUnavailable, "category_id", "category is archived or unavailable")
	}},
	{Match: isPlanningTrackUnavailable, Render: func(error) render.Renderer {
		return invalidOnField(common.CodeTimetableTemplatePlanningTrackUnavailable, "planning_track_id", "planning track is archived or unavailable")
	}},
	{Target: timetableModule.ErrTemplateWeekendWeekday, Render: func(error) render.Renderer {
		return invalidOnField(common.CodeTimetableTemplateWeekend, "weekdays", timetableModule.ErrTemplateWeekendWeekday.Error())
	}},
	{Target: timetableModule.ErrOfferingSourceInvalid, Render: codedOr(common.CodeTimetableOfferingSourceInvalid)},
	{Match: isTemplateEducationGroupError, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(err, common.CodeTimetableTemplateEducationGroupInvalid, "education_group_id")
	}},
	{Target: timetableModule.ErrTemplateTargetGradeExceedsLimit, Render: codedOr(common.CodeTimetableTemplateGradeAboveMax)},
}

func isPlanningTrackUnavailable(err error) bool {
	return errors.Is(err, timetableModule.ErrPlanningTrackNotFound) ||
		errors.Is(err, timetableModule.ErrPlanningTrackArchived)
}

func isTemplateEducationGroupError(err error) bool {
	var educationGroupErr *timetableModule.TemplateEducationGroupError
	return errors.As(err, &educationGroupErr)
}

// templateRefusalRenderer answers one of the shared refusals, or nil.
func templateRefusalRenderer(err error) render.Renderer {
	return common.RenderWithRules(err, templateRefusalRules, func(error) render.Renderer { return nil })
}

// templateSplitInvalidRenderer answers a refused split or end date: a coded
// refusal keeps its code, any other invalid input is template_split_invalid.
func templateSplitInvalidRenderer(err error) render.Renderer {
	return codedOr(common.CodeTimetableTemplateSplitInvalid)(err)
}
