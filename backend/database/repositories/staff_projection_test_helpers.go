package repositories

import (
	"time"

	"github.com/uptrace/bun"
)

// NewFactoryWithSchoolMembership builds the repository factory with the
// School Membership projections already bound, so repository tests read the
// same staff-enriched rows the service graph does. The owner itself is
// composed by NewSchoolMembership (school_membership_test_helpers.go).
func NewFactoryWithSchoolMembership(db *bun.DB, clocks ...func() time.Time) (*Factory, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	factory := NewFactory(db, clocks...)
	factory.bindStaffProjections(membership)
	return factory, nil
}
