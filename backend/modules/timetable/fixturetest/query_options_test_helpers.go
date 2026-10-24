//go:build testhooks

package fixturetest

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// narrowListOptions reads the retained listing options into a typed filter:
// the page through page, each sort field through sort, and each plain
// condition through apply. A nil sort refuses any order; compound filters are
// refused, never dropped.
func narrowListOptions[F any](options *QueryOptions, subject string, page func(*F, int, int), sort func(*F, ListSortField), apply func(*F, ListCondition) error) (F, error) {
	var result, none F
	if options == nil {
		return result, nil
	}
	if options.Pagination != nil {
		page(&result, options.Pagination.PageSize, options.Pagination.Offset())
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		if sort == nil {
			return none, fmt.Errorf("%s sorting is unsupported", subject)
		}
		for _, field := range options.Sorting.Fields {
			sort(&result, field)
		}
	}
	if options.Filter == nil {
		return result, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
		return none, fmt.Errorf("compound %s filters are unsupported", subject)
	}
	for _, condition := range options.Filter.Conditions() {
		if err := apply(&result, condition); err != nil {
			return none, err
		}
	}
	return result, nil
}

func ActivityInstanceListOptions(options *QueryOptions) (ports.LegacyInstanceFilter, error) {
	return narrowListOptions(options, "activity instance", func(f *ports.LegacyInstanceFilter, limit, offset int) { f.Limit, f.Offset = limit, offset }, nil, applyActivityInstanceCondition)
}

func applyActivityInstanceCondition(result *ports.LegacyInstanceFilter, condition ListCondition) error {
	switch condition.Operator {
	case ListOpEqual:
		return applyActivityInstanceEqual(result, condition.Field, condition.Value)
	case ListOpIn:
		return applyActivityInstanceIn(result, condition.Field, condition.Value)
	default:
		return errors.New("activity instance filter is unsupported")
	}
}

func applyActivityInstanceEqual(result *ports.LegacyInstanceFilter, field string, value any) error {
	switch field {
	case "id":
		return assignActivityInstanceID(&result.IDs, value)
	case "activity_group_id":
		return assignActivityInstanceID(&result.ActivityGroupIDs, value)
	case "active_group_id":
		return assignActivityInstanceID(&result.ActiveGroupIDs, value)
	case "date":
		date, ok := value.(calendar.Date)
		if ok {
			text := date.String()
			result.Date = &text
			return nil
		}
	case "status":
		status, ok := value.(string)
		if ok {
			result.Status = status
			return nil
		}
	case "is_spontaneous":
		spontaneous, ok := value.(bool)
		if ok {
			result.IsSpontaneous = &spontaneous
			return nil
		}
	case "idempotency_key":
		key, ok := value.(string)
		if ok {
			result.IdempotencyKey = key
			return nil
		}
	}
	return errors.New("activity instance filter is unsupported")
}

func assignActivityInstanceID(target *[]int64, value any) error {
	id, ok := value.(int64)
	if !ok {
		return errors.New("activity instance id filter is unsupported")
	}
	*target = append(*target, id)
	return nil
}

func applyActivityInstanceIn(result *ports.LegacyInstanceFilter, field string, value any) error {
	values, ok := value.([]interface{})
	if !ok {
		return errors.New("activity instance IN filter is unsupported")
	}
	switch field {
	case "id":
		return appendActivityInstanceIDs(&result.IDs, values)
	case "activity_group_id":
		return appendActivityInstanceIDs(&result.ActivityGroupIDs, values)
	case "active_group_id":
		return appendActivityInstanceIDs(&result.ActiveGroupIDs, values)
	case "date":
		for _, value := range values {
			date, dateOK := value.(calendar.Date)
			if !dateOK {
				return errors.New("activity instance date filter is unsupported")
			}
			result.Dates = append(result.Dates, date.String())
		}
		return nil
	default:
		return errors.New("activity instance IN filter is unsupported")
	}
}

func appendActivityInstanceIDs(target *[]int64, values []interface{}) error {
	for _, value := range values {
		if err := assignActivityInstanceID(target, value); err != nil {
			return err
		}
	}
	return nil
}

func InstanceStudentListOptions(options *QueryOptions) (ports.ParticipantListingFilter, error) {
	return narrowListOptions(options, "instance student", func(f *ports.ParticipantListingFilter, limit, offset int) { f.Limit, f.Offset = limit, offset }, nil, applyInstanceStudentCondition)
}

func applyInstanceStudentCondition(result *ports.ParticipantListingFilter, condition ListCondition) error {
	if condition.Operator == ListOpEqual && condition.Field == "status" {
		value, ok := condition.Value.(string)
		if !ok {
			return errors.New("instance student status filter is unsupported")
		}
		result.Status = &value
		return nil
	}
	if condition.Operator == ListOpEqual {
		id, ok := condition.Value.(int64)
		if !ok {
			return errors.New("instance student filter is unsupported")
		}
		return assignInstanceStudentID(result, condition.Field, id)
	}
	if condition.Operator != ListOpIn || condition.Field == "status" {
		return errors.New("instance student filter is unsupported")
	}
	values, ok := condition.Value.([]interface{})
	if !ok {
		return errors.New("instance student IN filter is unsupported")
	}
	for _, value := range values {
		id, idOK := value.(int64)
		if !idOK {
			return errors.New("instance student IN filter is unsupported")
		}
		if err := assignInstanceStudentID(result, condition.Field, id); err != nil {
			return err
		}
	}
	return nil
}

func assignInstanceStudentID(result *ports.ParticipantListingFilter, field string, id int64) error {
	switch field {
	case "id":
		result.IDs = append(result.IDs, id)
	case "instance_id":
		result.InstanceIDs = append(result.InstanceIDs, id)
	case "student_id":
		result.StudentIDs = append(result.StudentIDs, id)
	default:
		return errors.New("instance student filter is unsupported")
	}
	return nil
}

// InstanceStudentDay is the Berlin calendar day of an instant as the legacy
// adapters compare it with a planned participant's date.
