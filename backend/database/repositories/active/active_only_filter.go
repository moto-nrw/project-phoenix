package active

import modelBase "github.com/moto-nrw/project-phoenix/models/base"

// rewriteActiveOnlyFilter turns the API-only active_only flag into ordinary
// filter conditions understood by the generic repository list pipeline.
func rewriteActiveOnlyFilter(filter *modelBase.Filter, startField, endField string, activeAfter any) {
	activeOnly, ok := filter.Get("active_only")
	if !ok {
		return
	}

	filter.Remove("active_only")
	isActive, ok := activeOnly.(bool)
	if !ok {
		return
	}

	if isActive {
		if startField != "" {
			filter.LessThanOrEqual(startField, activeAfter)
		}
		active := modelBase.NewFilter().IsNull(endField)
		active.Or(*modelBase.NewFilter().GreaterThan(endField, activeAfter))
		filter.And(*active)
		return
	}
	filter.IsNotNull(endField).LessThanOrEqual(endField, activeAfter)
}
