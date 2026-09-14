package services

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAbsenceEmailDeliveryPreservesMetadataAndContent(t *testing.T) {
	t.Parallel()
	resolver := &testpkg.ReplyToResolverStub{Name: "School", Address: "school@example.test"}
	dispatcher := absenceEmailDispatcher{identity: resolver}
	dispatcher.from.Name, dispatcher.from.Address = "moto", "no-reply@example.test"
	message := absenceEmailMessage{
		Type: "absence_request_received", ReferenceID: 42, TenantID: 7, Recipient: "recipient@example.test",
		Template: "absence-request-received.html", Subject: "Request",
	}
	message.To.Name, message.To.Address = "Recipient", "recipient@example.test"
	message.Content.FirstName, message.Content.LastName = "First", "Last"
	message.Content.RequesterName, message.Content.AbsenceTypeLabel = "Requester", "Leave"
	message.Content.DateRange, message.Content.Note = "01.01.2026", "Note"
	message.Content.PreviousQuestion = "Question"
	message.Content.LinkURL, message.Content.LogoURL = "https://school.example.test/staff", "https://example.test/logo.png"
	request := dispatcher.request(context.Background(), message)
	require.Equal(t, message.TenantID, resolver.TenantID)
	require.Equal(t, message.Type, request.Metadata.Type)
	require.Equal(t, message.ReferenceID, request.Metadata.ReferenceID)
	require.Equal(t, message.Recipient, request.Metadata.Recipient)
	require.Equal(t, dispatcher.from, request.Message.From)
	require.Equal(t, message.To.Name, request.Message.To.Name)
	require.Equal(t, message.To.Address, request.Message.To.Address)
	require.Equal(t, "School", request.Message.ReplyTo.Name)
	require.Equal(t, "school@example.test", request.Message.ReplyTo.Address)
	require.Equal(t, message.Template, request.Message.Template)
	require.Equal(t, message.Subject, request.Message.Subject)
	require.Equal(t, map[string]any{
		"FirstName": "First", "LastName": "Last", "RequesterName": "Requester", "AbsenceTypeLabel": "Leave",
		"DateRange": "01.01.2026", "Note": "Note", "PreviousQuestion": "Question",
		"LinkURL": message.Content.LinkURL, "LogoURL": message.Content.LogoURL,
	}, request.Message.Content)
	resolver.Err = errors.New("reply address unavailable")
	message.Template = "absence-request-approved.html"
	message.Content.DecisionNote = "Approved"
	request = dispatcher.request(context.Background(), message)
	require.Empty(t, request.Message.ReplyTo.Address)
	content := request.Message.Content.(map[string]any)
	require.Equal(t, "Approved", content["DecisionNote"])
	require.NotContains(t, content, "PreviousQuestion")
	require.NotContains(t, content, "RequesterName")
	require.NotContains(t, content, "Note")
	resolver.Err = nil
	resolver.TenantID = 0
	message.TenantID = 0
	request = dispatcher.request(context.Background(), message)
	require.Zero(t, resolver.TenantID, "tenantless sends must not resolve a reply address")
	require.Empty(t, request.Message.ReplyTo.Address)
}
