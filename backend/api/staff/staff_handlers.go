package staff

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// parseAndGetStaff parses staff ID from URL and returns the staff if it exists.
// Returns nil and false if parsing fails or staff doesn't exist (error already rendered).
func (rs *Resource) parseAndGetStaff(w http.ResponseWriter, r *http.Request) (*users.Staff, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return nil, false
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return nil, false
	}

	return staff, true
}

// listStaff handles listing all staff members with optional filtering
// Optimized to avoid N+1 queries by batch-loading Person and Teacher data
func (rs *Resource) listStaff(w http.ResponseWriter, r *http.Request) {
	filters := parseListStaffFilters(r)
	ctx := r.Context()

	// Get all staff members with person data in a single query (avoids N+1)
	staffMembers, err := rs.PersonService.ListStaffWithPerson(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Collect all staff IDs for batch teacher lookup
	staffIDs := make([]int64, len(staffMembers))
	for i, s := range staffMembers {
		staffIDs[i] = s.ID
	}

	// Batch-load all teachers in a single query (avoids N+1)
	teacherMap, err := rs.PersonService.GetTeachersByStaffIDs(ctx, staffIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Batch-load staff who had supervision activity today (for "Anwesend" status)
	presentStaffIDs, err := rs.WorkSessionService.GetStaffIDsWithSupervisionToday(ctx)
	if err != nil {
		// Log warning but continue - presence status is non-critical
		rs.getLogger().Warn("failed to fetch present staff IDs", slog.String("error", err.Error()))
		presentStaffIDs = []int64{}
	}

	// Build presentMap for O(1) lookup
	presentMap := make(map[int64]bool, len(presentStaffIDs))
	for _, id := range presentStaffIDs {
		presentMap[id] = true
	}

	// Batch-load work session and absence status (non-critical)
	workStatusMap := rs.loadWorkStatusMap(ctx)
	absenceMap := rs.loadAbsenceMap(ctx)

	// Batch-load account roles, emails, and avatars for all staff members (non-critical)
	accountRoleMap := rs.loadAccountMap(ctx, staffMembers, "role", func(ctx context.Context, ids []int64) (map[int64]string, error) {
		return rs.AuthService.GetAccountRoleNames(ctx, ids)
	})
	accountEmailMap := rs.loadAccountMap(ctx, staffMembers, "email", func(ctx context.Context, ids []int64) (map[int64]string, error) {
		return rs.AuthService.GetAccountEmailsByIDs(ctx, ids)
	})
	accountAvatarMap := rs.loadAccountMap(ctx, staffMembers, "avatar", func(ctx context.Context, ids []int64) (map[int64]string, error) {
		return rs.AuthService.GetAccountAvatarsByIDs(ctx, ids)
	})

	// Build response objects using pre-loaded data
	responses := make([]interface{}, 0, len(staffMembers))
	for _, staff := range staffMembers {
		if response, include := rs.processStaffForListOptimized(ctx, staff, teacherMap, presentMap, workStatusMap, absenceMap, accountRoleMap, accountEmailMap, accountAvatarMap, filters); include {
			responses = append(responses, response)
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff members retrieved successfully")
}

// getStaff handles getting a staff member by ID
func (rs *Resource) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Get staff member with person data using FindWithPerson method
	staff, err := rs.PersonService.GetStaffWithPerson(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	rs.ensureStaffPerson(r.Context(), staff)
	accountRole, accountEmail, accountAvatar := rs.loadStaffAccountInfo(r.Context(), staff)
	wasPresentToday, workStatus, absenceType := rs.resolveStaffPresence(r.Context(), staff)

	// Check if this staff member is also a teacher
	teacher, err := rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)
	if err == nil && teacher != nil {
		response := newTeacherResponse(staff, teacher, wasPresentToday, workStatus, absenceType, accountRole, accountEmail, accountAvatar)
		common.Respond(w, r, http.StatusOK, response, "Teacher retrieved successfully")
		return
	}

	response := newStaffResponse(staff, false, wasPresentToday, workStatus, absenceType, accountRole, accountEmail, accountAvatar)
	common.Respond(w, r, http.StatusOK, response, "Staff member retrieved successfully")
}

// getFinancialProfile returns only the identity needed to operate the
// staff:financial Stammdaten section. It deliberately excludes the generic
// profile's notes, RFID, account, presence, and absence data.
func (rs *Resource) getFinancialProfile(w http.ResponseWriter, r *http.Request) {
	rs.getMinimalStaffProfile(w, r, "Financial staff profile retrieved successfully")
}

// getDocumentProfile returns only the staff identity needed to operate the
// documents tab. Dedicated health-document users must not need unrelated
// directory permissions merely to identify the profile they may access.
func (rs *Resource) getDocumentProfile(w http.ResponseWriter, r *http.Request) {
	rs.getMinimalStaffProfile(w, r, "Document staff profile retrieved successfully")
}

func (rs *Resource) getMinimalStaffProfile(w http.ResponseWriter, r *http.Request, message string) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	staff, err := rs.PersonService.GetStaffWithPerson(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}
	rs.ensureStaffPerson(r.Context(), staff)
	if staff.Person == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"id":        staff.ID,
		"name":      staff.Person.FirstName + " " + staff.Person.LastName,
		"firstName": staff.Person.FirstName,
		"lastName":  staff.Person.LastName,
	}, message)
}

// ensureStaffPerson lazily loads the linked person when GetStaffWithPerson did
// not populate it. Best-effort: a failure is logged, not fatal.
func (rs *Resource) ensureStaffPerson(ctx context.Context, staff *users.Staff) {
	if staff.Person != nil || staff.PersonID <= 0 {
		return
	}
	person, err := rs.PersonService.Get(ctx, staff.PersonID)
	if err != nil {
		rs.getLogger().Warn("failed to get person data for staff member",
			slog.Int64("staff_id", staff.ID),
			slog.String("error", err.Error()))
		return
	}
	staff.Person = person
}

// loadStaffAccountInfo resolves the tenant-scoped account role, email, and
// avatar for a staff member. All three are non-critical: a lookup failure
// yields an empty string rather than an error.
func (rs *Resource) loadStaffAccountInfo(ctx context.Context, staff *users.Staff) (role, email, avatar string) {
	if staff.Person == nil || staff.Person.AccountID == nil || rs.AuthService == nil {
		return "", "", ""
	}
	accountID := *staff.Person.AccountID
	if roleMap, err := rs.AuthService.GetAccountRoleNames(ctx, []int64{accountID}); err == nil {
		role = roleMap[accountID]
	}
	if emailMap, err := rs.AuthService.GetAccountEmailsByIDs(ctx, []int64{accountID}); err == nil {
		email = emailMap[accountID]
	}
	if avatarMap, err := rs.AuthService.GetAccountAvatarsByIDs(ctx, []int64{accountID}); err == nil {
		avatar = avatarMap[accountID]
	}
	return role, email, avatar
}

// resolveStaffPresence resolves the presence/work-status/absence for a single
// staff member, keeping the detail view consistent with the list view. The
// maps cover all staff today; we look up our single ID.
func (rs *Resource) resolveStaffPresence(ctx context.Context, staff *users.Staff) (present bool, workStatus, absence string) {
	if presentIDs, err := rs.WorkSessionService.GetStaffIDsWithSupervisionToday(ctx); err == nil {
		present = slices.Contains(presentIDs, staff.ID)
	} else {
		rs.getLogger().Warn("failed to fetch present staff IDs",
			slog.Int64("staff_id", staff.ID),
			slog.String("error", err.Error()))
	}
	workStatus = rs.loadWorkStatusMap(ctx)[staff.ID]
	absence = rs.loadAbsenceMap(ctx)[staff.ID]
	return present, workStatus, absence
}

// grantDefaultPermissions grants default permissions to a newly created account
func (rs *Resource) grantDefaultPermissions(ctx context.Context, accountID int64, role string) {
	if rs.AuthService == nil {
		return
	}

	// Get the groups:read permission
	perm, err := rs.AuthService.GetPermissionByName(ctx, permissions.GroupsRead)
	if err == nil && perm != nil {
		// Grant the permission to the account
		if err := rs.AuthService.GrantPermissionToAccount(ctx, int(accountID), int(perm.ID)); err != nil {
			rs.getLogger().Error("failed to grant groups:read permission",
				slog.String("role", role),
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()))
		}
	}
}

// createStaff handles creating a new staff member
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("person not found")))
		return
	}

	// Create staff record (and optionally teacher) in tenant transaction
	staff, teacher, teacherCreationFailed, err := rs.PersonService.CreateStaffWithTeacher(r.Context(), usersSvc.CreateStaffInput{
		PersonID:       req.PersonID,
		StaffNotes:     req.StaffNotes,
		IsTeacher:      req.IsTeacher,
		Specialization: req.Specialization,
		Role:           req.Role,
		Qualifications: req.Qualifications,
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	isTeacher := teacher != nil

	// Grant groups:read permission if they have an account
	if person.AccountID != nil {
		if isTeacher {
			rs.grantDefaultPermissions(r.Context(), *person.AccountID, "teacher")
		} else if !teacherCreationFailed {
			rs.grantDefaultPermissions(r.Context(), *person.AccountID, "staff")
		}
	}

	// Set person data for response
	staff.Person = person

	if teacherCreationFailed {
		response := newStaffResponse(staff, false, false, "", "", "", "", "")
		common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully, but failed to create teacher record")
		return
	}

	if isTeacher {
		// Return teacher response
		response := newTeacherResponse(staff, teacher, false, "", "", "", "", "")
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher, false, "", "", "", "", "")
	common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully")
}

// updateStaff handles updating a staff member
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	// Update basic fields
	staff.StaffNotes = req.StaffNotes

	// Handle person ID change
	if staff.PersonID != req.PersonID {
		if rs.updateStaffPerson(r.Context(), staff, req.PersonID) != nil {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("person not found")))
			return
		}
	}

	teacher, action, err := rs.PersonService.UpdateStaffWithTeacher(r.Context(), staff, req.IsTeacher, req.Specialization, req.Role, req.Qualifications)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response, message := updateStaffResponseFor(staff, teacher, action)
	common.Respond(w, r, http.StatusOK, response, message)
}

// updateStaffPerson validates and updates the person ID for a staff member
func (rs *Resource) updateStaffPerson(ctx context.Context, staff *users.Staff, personID int64) error {
	person, err := rs.PersonService.Get(ctx, personID)
	if err != nil {
		return err
	}
	staff.PersonID = personID
	staff.Person = person
	return nil
}

// deleteStaff handles offboarding a staff member: soft-deletes the staff and
// teacher rows and revokes the linked account's access for this tenant.
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	deletedByStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.StaffOffboardingService.OffboardStaff(ctx, id, deletedByStaffID, claims.Username); err != nil {
			return err
		}
		documents, err := rs.StaffDocumentService.ListStaffDocumentsPendingFileCleanup(ctx, id)
		if err != nil {
			return err
		}
		for _, document := range documents {
			docID, storedName := document.ID, document.FilenameStored
			tenant.RegisterAfterCommit(ctx, func() {
				rs.cleanupStaffDocumentFile(tenantID, id, docID, storedName, "offboarding")
			})
		}
		return nil
	}); err != nil {
		if errors.Is(err, usersSvc.ErrStaffInUse) {
			common.RenderError(w, r, common.ErrorConflictMessage(usersSvc.ErrStaffInUse.Error()))
			return
		}
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Personal kann nicht gelöscht werden: Mitarbeiter/in wird noch in anderen Bereichen referenziert"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// serveStaffAvatar serves the avatar image for a staff member.
// Requires users:read permission, any authenticated user in the same tenant can view staff avatars.
func (rs *Resource) serveStaffAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	staff, err := rs.PersonService.GetStaffWithPerson(r.Context(), id)
	if err != nil || staff == nil {
		http.NotFound(w, r)
		return
	}

	if staff.Person == nil || staff.Person.AccountID == nil {
		http.NotFound(w, r)
		return
	}

	accountID := *staff.Person.AccountID
	avatarMap, err := rs.AuthService.GetAccountAvatarsByIDs(r.Context(), []int64{accountID})
	if err != nil || avatarMap[accountID] == "" {
		http.NotFound(w, r)
		return
	}

	avatarPath := avatarMap[accountID]
	filePath, err := common.ResolveStoredPath("public", avatarPath, "/uploads/avatars/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	common.ServeImage(w, r, filepath.Dir(filePath), filepath.Base(filePath), "private, max-age=86400")
}
