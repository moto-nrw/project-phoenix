package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func (r operatorAccountDirectory) ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]OrgAccountInfo, error) {
	schools, err := r.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("load organization schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, nonDeletedSchoolIDs(schools))
}

func (r operatorAccountDirectory) ListAllAccounts(ctx context.Context) ([]OrgAccountInfo, error) {
	schools, err := r.schools.ListNonDeletedSchools(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, nonDeletedSchoolIDs(schools))
}

func (r operatorAccountDirectory) listAccounts(ctx context.Context, schoolIDs []int64) ([]OrgAccountInfo, error) {
	rows, err := r.accountsBySchoolIDs(ctx, schoolIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	schools, err := schoolsByID(ctx, r.schools, schoolIDs)
	if err != nil {
		return nil, fmt.Errorf("load school names for accounts: %w", err)
	}
	for index := range rows {
		school, found := schools[rows[index].SchoolID]
		if !found {
			return nil, fmt.Errorf("school %d missing for account row", rows[index].SchoolID)
		}
		rows[index].SchoolName = school.Name
	}
	slices.SortStableFunc(rows, func(left, right OrgAccountInfo) int {
		if order := compareStrings(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		if order := compareStrings(left.LastName, right.LastName); order != 0 {
			return order
		}
		return compareStrings(left.FirstName, right.FirstName)
	})
	return rows, nil
}

func nonDeletedSchoolIDs(schools []organizationtenancy.School) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		if school.IsDeleted() {
			continue
		}
		ids = append(ids, school.ID)
	}
	return ids
}

func schoolsByID(ctx context.Context, query organizationtenancy.Query, ids []int64) (map[int64]organizationtenancy.School, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	values, err := query.ListSchoolsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]organizationtenancy.School, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
