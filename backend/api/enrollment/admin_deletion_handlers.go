package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type AdminEnrollmentDeletionCounts struct {
	Requests                  int `json:"requests"`
	RequestChildren           int `json:"request_children"`
	RequestChildOfferings     int `json:"request_child_offerings"`
	RequestGuardians          int `json:"request_guardians"`
	ChangeRequests            int `json:"change_requests"`
	ChangeRequestMessages     int `json:"change_request_messages"`
	LateInvites               int `json:"late_invites"`
	OfferingAdjustments       int `json:"offering_adjustments"`
	EmailOutbox               int `json:"email_outbox"`
	RolloverLinksCleared      int `json:"rollover_links_cleared"`
	StudentSourceLinksCleared int `json:"student_source_links_cleared"`
	Total                     int `json:"total"`
}

type AdminEnrollmentDeletionImpact struct {
	RequestID                     string                        `json:"request_id"`
	ChildID                       string                        `json:"child_id,omitempty"`
	DeletesRequest                bool                          `json:"deletes_request"`
	CanDelete                     bool                          `json:"can_delete"`
	Counts                        AdminEnrollmentDeletionCounts `json:"counts"`
	BlockingStudentIDs            []string                      `json:"blocking_student_ids"`
	PreservedGuardianProfiles     int                           `json:"preserved_guardian_profiles"`
	PreservedParentAccounts       int                           `json:"preserved_parent_accounts"`
	UnlinkedGuardianProfiles      int                           `json:"unlinked_guardian_profiles"`
	ParentAccountsWithoutStudents int                           `json:"parent_accounts_without_students"`
}

type AdminEnrollmentDeleteRequest struct {
	Reason string `json:"reason"`
}

func (req *AdminEnrollmentDeleteRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) getAdminRequestDeleteImpact(w http.ResponseWriter, r *http.Request) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	rs.previewEnrollmentDeletion(w, r, requestID, 0)
}

func (rs *Resource) getAdminChildDeleteImpact(w http.ResponseWriter, r *http.Request) {
	requestID, childID, ok := parseEnrollmentDeletionIDs(w, r)
	if !ok {
		return
	}
	rs.previewEnrollmentDeletion(w, r, requestID, childID)
}

func (rs *Resource) previewEnrollmentDeletion(w http.ResponseWriter, r *http.Request, requestID, childID int64) {
	if rs.DeletionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment deletion service not configured")))
		return
	}
	var impact *enrollmentModels.DeletionImpact
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		var previewErr error
		if childID > 0 {
			impact, previewErr = rs.DeletionService.PreviewChild(ctx, requestID, childID)
		} else {
			impact, previewErr = rs.DeletionService.PreviewRequest(ctx, requestID)
		}
		return previewErr
	})
	if err != nil {
		renderEnrollmentDeletionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAdminEnrollmentDeletionImpact(impact), "Enrollment deletion impact retrieved")
}

func (rs *Resource) deleteAdminRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	rs.executeEnrollmentDeletion(w, r, requestID, 0)
}

func (rs *Resource) deleteAdminChild(w http.ResponseWriter, r *http.Request) {
	requestID, childID, ok := parseEnrollmentDeletionIDs(w, r)
	if !ok {
		return
	}
	rs.executeEnrollmentDeletion(w, r, requestID, childID)
}

func (rs *Resource) executeEnrollmentDeletion(w http.ResponseWriter, r *http.Request, requestID, childID int64) {
	if rs.DeletionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment deletion service not configured")))
		return
	}
	body := new(AdminEnrollmentDeleteRequest)
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	actorAccountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	var impact *enrollmentModels.DeletionImpact
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		var deleteErr error
		if childID > 0 {
			impact, deleteErr = rs.DeletionService.DeleteChild(ctx, requestID, childID, actorAccountID, body.Reason)
		} else {
			impact, deleteErr = rs.DeletionService.DeleteRequest(ctx, requestID, actorAccountID, body.Reason)
		}
		return deleteErr
	})
	if err != nil {
		renderEnrollmentDeletionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAdminEnrollmentDeletionImpact(impact), "Enrollment deleted")
}

func parseEnrollmentDeletionIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return 0, 0, false
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	return requestID, childID, ok
}

func renderEnrollmentDeletionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrEnrollmentDeletionNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrEnrollmentDeletionInvalidReason):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrEnrollmentDeletionStudentExists):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.student_exists"))
	case errors.Is(err, enrollmentService.ErrEnrollmentDeletionNotAllowed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.child_deletion_not_allowed"))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func toAdminEnrollmentDeletionImpact(impact *enrollmentModels.DeletionImpact) AdminEnrollmentDeletionImpact {
	if impact == nil {
		return AdminEnrollmentDeletionImpact{}
	}
	blockingIDs := make([]string, 0, len(impact.BlockingStudentIDs))
	for _, id := range impact.BlockingStudentIDs {
		blockingIDs = append(blockingIDs, strconv.FormatInt(id, 10))
	}
	return AdminEnrollmentDeletionImpact{
		RequestID:                     strconv.FormatInt(impact.RequestID, 10),
		ChildID:                       optionalInt64String(impact.ChildID),
		DeletesRequest:                impact.DeletesRequest,
		CanDelete:                     len(impact.BlockingStudentIDs) == 0,
		Counts:                        toAdminEnrollmentDeletionCounts(impact.Counts),
		BlockingStudentIDs:            blockingIDs,
		PreservedGuardianProfiles:     impact.PreservedGuardianProfiles,
		PreservedParentAccounts:       impact.PreservedParentAccounts,
		UnlinkedGuardianProfiles:      impact.UnlinkedGuardianProfiles,
		ParentAccountsWithoutStudents: impact.ParentAccountsWithoutStudents,
	}
}

func toAdminEnrollmentDeletionCounts(counts enrollmentModels.DeletionCounts) AdminEnrollmentDeletionCounts {
	return AdminEnrollmentDeletionCounts{
		Requests:                  counts.Requests,
		RequestChildren:           counts.RequestChildren,
		RequestChildOfferings:     counts.RequestChildOfferings,
		RequestGuardians:          counts.RequestGuardians,
		ChangeRequests:            counts.ChangeRequests,
		ChangeRequestMessages:     counts.ChangeRequestMessages,
		LateInvites:               counts.LateInvites,
		OfferingAdjustments:       counts.OfferingAdjustments,
		EmailOutbox:               counts.EmailOutbox,
		RolloverLinksCleared:      counts.RolloverLinksCleared,
		StudentSourceLinksCleared: counts.StudentSourceLinksCleared,
		Total:                     counts.Total(),
	}
}
