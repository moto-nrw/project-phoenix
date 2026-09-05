package activities

import (
	"context"
	"errors"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type CategoryQueryOptions = modelBase.QueryOptions

func ValidateCategoryListOptions(options *CategoryQueryOptions) error {
	if options != nil && (options.Filter != nil || options.Pagination != nil || options.Sorting != nil) {
		return errors.New("category list options are unsupported")
	}
	return nil
}

// WrapDatabaseError preserves the legacy repository error contract for
// composition-layer adapters that cannot import legacy model infrastructure.
func WrapDatabaseError(operation string, err error) error {
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

// StudentEnrollmentListOptions translates the only filtered legacy list use
// into the timetable owner's explicit query contract.
func StudentEnrollmentListOptions(options *activitiesModels.StudentEnrollmentQueryOptions) ([]int64, int, int, error) {
	if options == nil {
		return nil, 0, 0, nil
	}
	limit, offset := 0, 0
	if options.Pagination != nil {
		limit, offset = options.Pagination.PageSize, options.Pagination.Offset()
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		return nil, 0, 0, errors.New("student enrollment sorting is unsupported")
	}
	if options.Filter == nil {
		return nil, limit, offset, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
		return nil, 0, 0, errors.New("compound student enrollment filters are unsupported")
	}
	var studentIDs []int64
	for _, condition := range options.Filter.Conditions() {
		if condition.Field != "student_id" || condition.Operator != modelBase.OpIn {
			return nil, 0, 0, fmt.Errorf("student enrollment filter %s %s is unsupported", condition.Field, condition.Operator)
		}
		ids, err := studentEnrollmentFilterIDs(condition.Value)
		if err != nil {
			return nil, 0, 0, err
		}
		studentIDs = ids
	}
	return studentIDs, limit, offset, nil
}

func studentEnrollmentFilterIDs(value any) ([]int64, error) {
	values, ok := value.([]interface{})
	if !ok {
		return nil, errors.New("student enrollment IDs filter has invalid values")
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		id, ok := value.(int64)
		if !ok || id <= 0 {
			return nil, errors.New("student enrollment IDs must be positive int64 values")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// WrapNotFoundDatabaseError preserves the typed legacy not-found contract.
func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, modelBase.ErrNotFound)
}

// ContextTenantMatches guards compatibility methods whose old signature
// redundantly carries the tenant beside the authenticated context.
func ContextTenantMatches(ctx context.Context, tenantID int64) bool {
	contextTenant, err := tenant.TenantFromContext(ctx)
	return err == nil && contextTenant.Int64() == tenantID
}
