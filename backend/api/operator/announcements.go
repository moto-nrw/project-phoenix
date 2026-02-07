package operator

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// AnnouncementsResource handles operator announcements endpoints
type AnnouncementsResource struct {
	announcementService platformSvc.AnnouncementService
}

// NewAnnouncementsResource creates a new announcements resource
func NewAnnouncementsResource(announcementService platformSvc.AnnouncementService) *AnnouncementsResource {
	return &AnnouncementsResource{
		announcementService: announcementService,
	}
}

// AnnouncementResponse represents an announcement in the response
type AnnouncementResponse struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Version     *string  `json:"version,omitempty"`
	Active      bool     `json:"active"`
	PublishedAt *string  `json:"published_at,omitempty"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
	TargetRoles []string `json:"target_roles"`
	CreatedBy   int64    `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Status      string   `json:"status"` // draft, published, expired
}

// CreateAnnouncementRequest represents the create announcement request body
type CreateAnnouncementRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Version     *string  `json:"version,omitempty"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
	TargetRoles []string `json:"target_roles,omitempty"`
}

// Bind validates the create announcement request
func (req *CreateAnnouncementRequest) Bind(r *http.Request) error {
	return nil
}

// UpdateAnnouncementRequest represents the update announcement request body
type UpdateAnnouncementRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Version     *string  `json:"version,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
	TargetRoles []string `json:"target_roles,omitempty"`
}

// Bind validates the update announcement request
func (req *UpdateAnnouncementRequest) Bind(r *http.Request) error {
	return nil
}

// ListAnnouncements handles listing all announcements for operators
func (rs *AnnouncementsResource) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	announcements, err := rs.announcementService.ListAnnouncements(r.Context(), includeInactive)
	if err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	responses := make([]AnnouncementResponse, 0, len(announcements))
	for _, announcement := range announcements {
		responses = append(responses, newAnnouncementResponse(announcement))
	}

	common.Respond(w, r, http.StatusOK, responses, "Announcements retrieved successfully")
}

// GetAnnouncement handles getting a single announcement
func (rs *AnnouncementsResource) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	announcement, err := rs.announcementService.GetAnnouncement(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newAnnouncementResponse(announcement), "Announcement retrieved successfully")
}

// CreateAnnouncement handles creating a new announcement
func (rs *AnnouncementsResource) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &CreateAnnouncementRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.Title == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("title is required")))
		return
	}
	if req.Content == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("content is required")))
		return
	}

	announcement := &platform.Announcement{
		Title:       req.Title,
		Content:     req.Content,
		Type:        req.Type,
		Severity:    req.Severity,
		Version:     req.Version,
		TargetRoles: req.TargetRoles,
		Active:      true,
	}

	// Set defaults if not provided
	if announcement.Type == "" {
		announcement.Type = platform.TypeAnnouncement
	}
	if announcement.Severity == "" {
		announcement.Severity = platform.SeverityInfo
	}

	// Parse expires_at if provided
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid expires_at format, expected RFC3339")))
			return
		}
		announcement.ExpiresAt = &expiresAt
	}

	clientIP := getClientIP(r)

	if err := rs.announcementService.CreateAnnouncement(r.Context(), announcement, operatorID, clientIP); err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newAnnouncementResponse(announcement), "Announcement created successfully")
}

// UpdateAnnouncement handles updating an announcement
func (rs *AnnouncementsResource) UpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	req := &UpdateAnnouncementRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	// Get existing announcement
	existing, err := rs.announcementService.GetAnnouncement(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	// Update fields
	existing.Title = req.Title
	existing.Content = req.Content
	existing.Type = req.Type
	existing.Severity = req.Severity
	existing.Version = req.Version
	existing.TargetRoles = req.TargetRoles
	if req.Active != nil {
		existing.Active = *req.Active
	}

	// Parse expires_at if provided
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			existing.ExpiresAt = nil
		} else {
			expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid expires_at format, expected RFC3339")))
				return
			}
			existing.ExpiresAt = &expiresAt
		}
	}

	clientIP := getClientIP(r)

	if err := rs.announcementService.UpdateAnnouncement(r.Context(), existing, operatorID, clientIP); err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newAnnouncementResponse(existing), "Announcement updated successfully")
}

// DeleteAnnouncement handles deleting an announcement
func (rs *AnnouncementsResource) DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	clientIP := getClientIP(r)

	if err := rs.announcementService.DeleteAnnouncement(r.Context(), id, operatorID, clientIP); err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement deleted successfully")
}

// PublishAnnouncement handles publishing an announcement
func (rs *AnnouncementsResource) PublishAnnouncement(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	clientIP := getClientIP(r)

	if err := rs.announcementService.PublishAnnouncement(r.Context(), id, operatorID, clientIP); err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement published successfully")
}

// GetStats handles getting view statistics for an announcement
func (rs *AnnouncementsResource) GetStats(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	stats, err := rs.announcementService.GetStats(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, stats, "Stats retrieved successfully")
}

// AnnouncementViewDetailResponse represents a view detail in the response
type AnnouncementViewDetailResponse struct {
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	SeenAt    string `json:"seen_at"`
	Dismissed bool   `json:"dismissed"`
}

// GetViewDetails handles getting detailed view information for an announcement
func (rs *AnnouncementsResource) GetViewDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	details, err := rs.announcementService.GetViewDetails(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, AnnouncementErrorRenderer(err))
		return
	}

	responses := make([]AnnouncementViewDetailResponse, 0, len(details))
	for _, detail := range details {
		responses = append(responses, AnnouncementViewDetailResponse{
			UserID:    detail.UserID,
			UserName:  detail.UserName,
			SeenAt:    detail.SeenAt.Format(time.RFC3339),
			Dismissed: detail.Dismissed,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "View details retrieved successfully")
}

// newAnnouncementResponse creates an announcement response from an announcement model
func newAnnouncementResponse(a *platform.Announcement) AnnouncementResponse {
	targetRoles := a.TargetRoles
	if targetRoles == nil {
		targetRoles = []string{}
	}

	response := AnnouncementResponse{
		ID:          a.ID,
		Title:       a.Title,
		Content:     a.Content,
		Type:        a.Type,
		Severity:    a.Severity,
		Version:     a.Version,
		Active:      a.Active,
		TargetRoles: targetRoles,
		CreatedBy:   a.CreatedBy,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
	}

	if a.PublishedAt != nil {
		formatted := a.PublishedAt.Format(time.RFC3339)
		response.PublishedAt = &formatted
	}

	if a.ExpiresAt != nil {
		formatted := a.ExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &formatted
	}

	// Determine status
	if a.IsDraft() {
		response.Status = "draft"
	} else if a.IsExpired() {
		response.Status = "expired"
	} else {
		response.Status = "published"
	}

	return response
}
