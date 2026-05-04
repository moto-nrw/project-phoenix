package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MeProfileResponse is the autofill payload for the parent enrollment
// form. It carries the guardian's account info + their linked students
// so the form can prefill guardian fields and offer one-click child
// reuse. Returns 200 with empty children when the guardian has none
// yet; 401 when no session is in scope.
type MeProfileResponse struct {
	Guardian MeGuardian `json:"guardian"`
	Children []MeChild  `json:"children"`
}

type MeGuardian struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
}

// MeChild only carries fields the form needs to prefill a child slot.
// Date-of-birth is intentionally absent — users.students has no DOB
// column today; parent fills it manually for the enrollment row. The
// PR 8 admin-side decision service will reconcile against the existing
// student record by id when promoting to approved.
type MeChild struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GradeLevel  *int   `json:"grade_level,omitempty"`
}

// getMyProfile assembles the autofill payload for the JWT-bearing
// guardian. Slug-gated only by the tenant tx — RLS narrows reads to
// the resolved tenant. We deliberately don't 401 hard when the
// session lacks a guardian profile (e.g. teacher session on the
// public form): we return guardian fields filled from the auth
// account but children=[] so the form still works.
func (rs *Resource) getMyProfile(w http.ResponseWriter, r *http.Request) {
	if rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("me/profile not wired")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("not authenticated")))
		return
	}
	accountID := int64(claims.ID)
	tenantID := tenant.FromContext(r.Context())
	if tenantID == 0 {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("tenant context missing")))
		return
	}

	resp := MeProfileResponse{
		Guardian: MeGuardian{
			FirstName: claims.FirstName,
			LastName:  claims.LastName,
			Email:     claims.Sub, // JWT subject carries the email
		},
		Children: []MeChild{},
	}

	err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		// Look up the guardian profile linked to this account. May not
		// exist (teacher session, freshly-invited guardian without a
		// profile yet) — that's fine; we just skip children and return
		// the auth-derived guardian fields.
		var profile usersModels.GuardianProfile
		schoolErr := tx.NewSelect().
			Model(&profile).
			ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
			Where(`"guardian_profile".account_id = ?`, accountID).
			Limit(1).
			Scan(txCtx)
		if schoolErr != nil {
			// no profile linked → return early with just the auth claims
			return nil //nolint:nilerr // deliberate fall-through
		}

		// Override the auth-derived names with the profile's curated
		// values when available — admin may have cleaned them up.
		if profile.FirstName != "" {
			resp.Guardian.FirstName = profile.FirstName
		}
		if profile.LastName != "" {
			resp.Guardian.LastName = profile.LastName
		}
		if profile.Email != nil && *profile.Email != "" {
			resp.Guardian.Email = *profile.Email
		}

		// Primary phone from guardian_phone_numbers. Best-effort: if
		// the join fails or there's no row, leave phone nil.
		var primaryPhone usersModels.GuardianPhoneNumber
		if err := tx.NewSelect().
			Model(&primaryPhone).
			ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
			Where(`"guardian_phone_number".guardian_profile_id = ?`, profile.ID).
			OrderExpr(`"guardian_phone_number".is_primary DESC, "guardian_phone_number".id ASC`).
			Limit(1).
			Scan(txCtx); err == nil && primaryPhone.PhoneNumber != "" {
			phone := primaryPhone.PhoneNumber
			resp.Guardian.Phone = &phone
		}

		// Children: students_guardians → students → persons (for names).
		type studentRow struct {
			StudentID   int64  `bun:"student_id"`
			FirstName   string `bun:"first_name"`
			LastName    string `bun:"last_name"`
			SchoolClass string `bun:"school_class"`
		}
		var rows []studentRow
		if err := tx.NewSelect().
			TableExpr(`users.students_guardians AS "sg"`).
			ColumnExpr(`"s".id AS student_id`).
			ColumnExpr(`"p".first_name`).
			ColumnExpr(`"p".last_name`).
			ColumnExpr(`"s".school_class`).
			Join(`INNER JOIN users.students AS "s" ON "s".id = "sg".student_id`).
			Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
			Where(`"sg".guardian_profile_id = ?`, profile.ID).
			Where(`"s".status <> ?`, "alumnus").
			OrderExpr(`"p".last_name ASC, "p".first_name ASC`).
			Scan(txCtx, &rows); err == nil {
			for _, row := range rows {
				child := MeChild{
					ID:          strconv.FormatInt(row.StudentID, 10),
					FirstName:   row.FirstName,
					LastName:    row.LastName,
					SchoolClass: row.SchoolClass,
				}
				if grade := parseGradeLevel(row.SchoolClass); grade > 0 {
					child.GradeLevel = &grade
				}
				resp.Children = append(resp.Children, child)
			}
		}
		return nil
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Profile retrieved")
}

// parseGradeLevel pulls the leading integer out of strings like "1a",
// "2b", "10" → 1, 2, 10. Returns 0 when nothing parses (e.g.
// "Vorschule"); the form treats that as "no prefill".
func parseGradeLevel(schoolClass string) int {
	digits := ""
	for _, r := range schoolClass {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		break
	}
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}
