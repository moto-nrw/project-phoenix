package common

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianProfileResponse is the autofill payload for the parent enrollment
// form: the guardian's account info plus their linked students, so the form
// can prefill guardian fields and offer one-click child reuse. Shared by the
// public tenant route (api/enrollment) and the parents-portal route
// (api/parent), which must emit the identical wire shape for the same
// EnrollmentForm component.
type GuardianProfileResponse struct {
	Guardian GuardianProfileGuardian `json:"guardian"`
	Children []GuardianProfileChild  `json:"children"`
}

type GuardianProfileGuardian struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
}

// GuardianProfileChild only carries fields the form needs to prefill a child
// slot. Date-of-birth is intentionally absent — users.students has no DOB
// column today; the parent fills it manually for the enrollment row.
type GuardianProfileChild struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GradeLevel  *int   `json:"grade_level,omitempty"`
	// EnrollmentSubmit reports whether this guardian relationship grants
	// parent_portal.enrollment.submit for this child. The form offers reuse
	// only for children where this is true, so a permission-revoked child is
	// never presented as reusable (would 403 at submit) (#1663).
	EnrollmentSubmit bool `json:"enrollment_submit"`
	// Status is the child's lifecycle status ("active" | "pending" |
	// "inactive"). Only active/pending students count as enrolled, so the
	// reuse picker drops the rest instead of offering a candidate the
	// existing_students submit gate would reject (#1663).
	Status string `json:"status"`
}

// BuildGuardianProfileResponse merges claims-derived defaults with the
// (possibly nil) loaded profile. Nil loaded → defaults only with an empty
// children list.
func BuildGuardianProfileResponse(claims jwt.AppClaims, loaded *usersModels.GuardianProfileWithChildren) GuardianProfileResponse {
	resp := GuardianProfileResponse{
		Guardian: GuardianProfileGuardian{
			FirstName: claims.FirstName,
			LastName:  claims.LastName,
			Email:     claims.Sub,
		},
		Children: []GuardianProfileChild{},
	}
	if loaded == nil || loaded.Profile == nil {
		return resp
	}
	if loaded.Profile.FirstName != "" {
		resp.Guardian.FirstName = loaded.Profile.FirstName
	}
	if loaded.Profile.LastName != "" {
		resp.Guardian.LastName = loaded.Profile.LastName
	}
	if loaded.Profile.Email != nil && *loaded.Profile.Email != "" {
		resp.Guardian.Email = *loaded.Profile.Email
	}
	if loaded.PrimaryPhone != "" {
		phone := loaded.PrimaryPhone
		resp.Guardian.Phone = &phone
	}
	for _, child := range loaded.Children {
		entry := GuardianProfileChild{
			ID:               strconv.FormatInt(child.StudentID, 10),
			FirstName:        child.FirstName,
			LastName:         child.LastName,
			SchoolClass:      child.SchoolClass,
			Status:           child.Status,
			EnrollmentSubmit: child.EnrollmentSubmit,
		}
		if grade := ParseLeadingGradeLevel(child.SchoolClass); grade > 0 {
			entry.GradeLevel = &grade
		}
		resp.Children = append(resp.Children, entry)
	}
	return resp
}

// ParseLeadingGradeLevel derives the grade using the shared class-label
// grammar ("2b" -> 2, "Klasse 3a" -> 3). Returns 0 when the label has no
// grade number or the extracted value cannot be represented as an int; the
// form treats that as "no prefill".
func ParseLeadingGradeLevel(schoolClass string) int {
	digits := schoolclass.GradePrefix(schoolClass)
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}
