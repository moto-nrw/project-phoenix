package organizationtenancy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
)

// Test Träger (#4658, spec #4611 user story 125): the moto team sets up a
// test system in the operator portal, so a new Träger can try everything
// before real children are entered. A test Träger is marked at creation and
// keeps the mark; its test schools are hidden from the public school list.
// The sample children come from Data Import, which asks for the test schools
// here. A test Träger is erased like any other Träger (#4657).

// Bounds of the number of test schools one test Träger gets.
const (
	MinTestSchools     = 1
	MaxTestSchools     = 5
	DefaultTestSchools = 3
)

// testSchoolSubdomainSuffix is the longest suffix a test school's subdomain
// gets after the Träger slug, "-test-5".
const testSchoolSubdomainSuffix = len("-test-") + 1

// ErrNotTestTraeger reports a sample data request for a Träger without the
// test mark: invented children never enter a real Träger's schools.
var ErrNotTestTraeger = errors.New("organization is not a test Träger")

// CreateTestTraeger is the operator's order for a test system: the name and
// slug of the test Träger, the number of test schools and the contact address
// the test schools need like every school.
type CreateTestTraeger struct {
	Name         string
	Slug         string
	SchoolCount  int
	ContactEmail string
}

// TestTraeger is the created test Träger with its test schools.
type TestTraeger struct {
	Organization Organization `json:"organization"`
	Schools      []School     `json:"schools"`
}

// TestTraegerProvisioning creates test Träger and names their schools for the
// sample data. Both run in their own administrative unit and write the
// operator audit.
type TestTraegerProvisioning interface {
	CreateTestTraeger(ctx context.Context, input CreateTestTraeger, operatorID int64, clientIP net.IP) (*TestTraeger, error)
	// RecordSampleChildrenRequest records the operator's request for sample
	// children in the operator audit and returns the undeleted schools of the
	// active test Träger. A Träger without the test mark is
	// ErrNotTestTraeger, a deactivated or deleted one ErrOrganizationInactive.
	RecordSampleChildrenRequest(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) ([]int64, error)
}

// NormalizeCreateTestTraeger trims the order, applies the default school
// count and checks the fields only a test Träger has. The Träger name and
// slug are checked when the organization is created.
func NormalizeCreateTestTraeger(input *CreateTestTraeger) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	if input.SchoolCount == 0 {
		input.SchoolCount = DefaultTestSchools
	}
	if input.SchoolCount < MinTestSchools || input.SchoolCount > MaxTestSchools {
		return fmt.Errorf("%w: school count must be between %d and %d", ErrInvalidOrganization, MinTestSchools, MaxTestSchools)
	}
	if limit := MaxDNSLabelLength - testSchoolSubdomainSuffix; len(input.Slug) > limit {
		return fmt.Errorf("%w: slug of a test Träger must not exceed %d characters", ErrInvalidOrganization, limit)
	}
	if address, err := mail.ParseAddress(input.ContactEmail); err != nil || address.Address != input.ContactEmail {
		return fmt.Errorf("%w: contact email must be a valid address", ErrInvalidOrganization)
	}
	return nil
}

// TestSchools are the schools of a new test Träger: "Test-Schule 1" to n,
// hidden from the public school list, with subdomains derived from the
// Träger slug.
func TestSchools(organization Organization, input CreateTestTraeger) []CreateSchool {
	schools := make([]CreateSchool, 0, input.SchoolCount)
	for number := 1; number <= input.SchoolCount; number++ {
		schools = append(schools, CreateSchool{
			OrganizationID: organization.ID,
			Name:           fmt.Sprintf("Test-Schule %d", number),
			Slug:           fmt.Sprintf("test-schule-%d", number),
			Subdomain:      fmt.Sprintf("%s-test-%d", organization.Slug, number),
			Active:         true,
			Hidden:         true,
			Email:          input.ContactEmail,
		})
	}
	return schools
}
