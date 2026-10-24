//go:build testhooks

package api

import userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

// ParentRequestReviewStudent returns a detached student row that belongs to
// groupID (nil: no group), for the review policy suites that decide over the
// row alone and never touch the database. Those suites may not import the
// retained models (#3966).
func ParentRequestReviewStudent(groupID *int64) *userModels.StudentState {
	return &userModels.StudentState{GroupID: groupID}
}
