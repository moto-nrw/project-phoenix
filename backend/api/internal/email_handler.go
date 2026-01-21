package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/logging"
)

// SendEmailRequest represents a request to send an email.
type SendEmailRequest struct {
	// Template is the name of the email template to use (without .html extension)
	Template string `json:"template"`
	// To is the recipient email address
	To string `json:"to"`
	// Subject is the email subject (optional, will use template default if not provided)
	Subject string `json:"subject,omitempty"`
	// Data contains template variables
	Data map[string]any `json:"data"`
}

// SendEmailResponse represents the response from sending an email.
type SendEmailResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// validTemplates defines the allowed email templates for internal API.
// This prevents arbitrary template injection.
var validTemplates = []string{
	// Organization lifecycle emails
	"org-pending",
	"org-approved",
	"org-rejected",
	// Member lifecycle emails
	"member-pending",
	"member-approved",
	"member-rejected",
	// Organization invitations
	"org-invitation",
}

// templateSubjects provides default subjects for each template.
var templateSubjects = map[string]string{
	"org-pending":     "Deine Organisation wird geprüft",
	"org-approved":    "Willkommen bei moto - Deine Organisation ist freigeschaltet!",
	"org-rejected":    "Deine Organisationsanfrage wurde abgelehnt",
	"member-pending":  "Deine Mitgliedschaftsanfrage wird geprüft",
	"member-approved": "Willkommen bei deiner Organisation!",
	"member-rejected": "Deine Mitgliedschaftsanfrage wurde abgelehnt",
	"org-invitation":  "Du wurdest zu einer Organisation eingeladen",
}

// sendEmail handles POST /api/internal/email
// This endpoint is internal-only and should not be exposed to the public internet.
func (rs *Resource) sendEmail(w http.ResponseWriter, r *http.Request) {
	var req SendEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"status":"error","message":"Invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Template == "" {
		http.Error(w, `{"status":"error","message":"Template is required"}`, http.StatusBadRequest)
		return
	}
	if req.To == "" {
		http.Error(w, `{"status":"error","message":"To (recipient email) is required"}`, http.StatusBadRequest)
		return
	}

	// Validate template is in allowed list
	if !slices.Contains(validTemplates, req.Template) {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"template":  req.Template,
				"recipient": req.To,
			}).Warn("Rejected unknown email template")
		}
		http.Error(w, `{"status":"error","message":"Unknown template"}`, http.StatusBadRequest)
		return
	}

	// Use default subject if not provided
	subject := req.Subject
	if subject == "" {
		subject = templateSubjects[req.Template]
	}
	if subject == "" {
		subject = "Nachricht von moto"
	}

	// Build email message
	// Template name should include .html extension for the mailer
	templateName := req.Template + ".html"

	msg := email.Message{
		From:     rs.fromEmail,
		To:       email.NewEmail("", req.To),
		Subject:  subject,
		Template: templateName,
		Content:  req.Data,
	}

	// Log the request (without sensitive data)
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"template":  req.Template,
			"recipient": req.To,
			"subject":   subject,
		}).Info("Internal email API: queuing email")
	}

	// Dispatch email asynchronously
	rs.dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
		Message: msg,
		Metadata: email.DeliveryMetadata{
			Type:      "internal_api_" + req.Template,
			Recipient: req.To,
		},
	})

	// Return success (email is queued, not necessarily sent yet)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	resp := SendEmailResponse{
		Status:  "queued",
		Message: "Email has been queued for delivery",
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Log but don't fail - the email is already queued
		if logging.Logger != nil {
			logging.Logger.WithError(err).Error("Failed to encode response")
		}
	}
}
