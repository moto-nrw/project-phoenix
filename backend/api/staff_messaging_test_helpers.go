//go:build testhooks

package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/communication/http/messageattachments"
	staffmessagingHTTP "github.com/moto-ogs/project-phoenix/modules/communication/http/staffmessages"

	"github.com/moto-ogs/project-phoenix/realtime"

	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	"github.com/moto-ogs/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-ogs/project-phoenix/modules/communication/composition"
	deliveryPolicy "github.com/moto-ogs/project-phoenix/modules/delivery/devices"
	settingsValues "github.com/moto-ogs/project-phoenix/modules/settings"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// StaffMessagingTestModule is the Communication owner's staff messaging.
type StaffMessagingTestModule struct {
	StaffMessaging communication.StaffMessagingRuntime
	// Attachments are the photos and files of the team chat (#6685).
	Attachments communication.MessageAttachmentCapability
}

// TenantStaffMessagingRouterWithAttachmentsForTests mounts the team chat with
// its attachment routes over the production tenant units and an in-memory
// object store, the way the server mounts them.
func (m StaffMessagingTestModule) TenantStaffMessagingRouterWithAttachmentsForTests() chi.Router {
	attachments := NewMessageAttachmentRoutesForTests(m.Attachments)
	return staffmessagingHTTP.NewResource(m.StaffMessaging).Router(ProtectedTenantRoutesWith(attachments.Register))
}

// NewMessageAttachmentRoutesForTests builds the attachment routes of a chat
// family over the production tenant units and an in-memory object store.
func NewMessageAttachmentRoutesForTests(attachments communication.MessageAttachmentCapability, guard ...func(http.Handler) http.Handler) *messageattachments.Resource {
	return messageattachments.NewResource(messageAttachmentUnits{attachments: attachments}, NewMemoryMessageAttachmentStorageForTests(), guard...)
}

// NewMemoryMessageAttachmentStorageForTests is an in-memory object store for
// the attachment routes.
func NewMemoryMessageAttachmentStorageForTests() common.MessageAttachmentFileStorage {
	files := &memoryAttachmentFiles{objects: map[string][]byte{}}
	return func() (common.MessageAttachmentFiles, error) { return files, nil }
}

// memoryAttachmentFiles is the object store of the attachment routes in tests.
type memoryAttachmentFiles struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memoryAttachmentFiles) Save(_ context.Context, tenantID int64, storedName string, source io.Reader) (int64, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[fmt.Sprintf("%d/%s", tenantID, storedName)] = data
	return int64(len(data)), nil
}

func (m *memoryAttachmentFiles) serve(w http.ResponseWriter, tenantID int64, storedName, contentType, disposition string) bool {
	m.mu.Lock()
	data, ok := m.objects[fmt.Sprintf("%d/%s", tenantID, storedName)]
	m.mu.Unlock()
	if !ok {
		return false
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", disposition)
	_, _ = w.Write(data)
	return true
}

func (m *memoryAttachmentFiles) Serve(w http.ResponseWriter, _ *http.Request, tenantID int64, storedName, _, contentType string) bool {
	return m.serve(w, tenantID, storedName, contentType, "attachment")
}

func (m *memoryAttachmentFiles) ServeInline(w http.ResponseWriter, _ *http.Request, tenantID int64, storedName, _, contentType string) bool {
	return m.serve(w, tenantID, storedName, contentType, "inline")
}

// SchoolStaffMessagingRouterForTests mounts the actual school-scope resource
// with the production problem response middleware in the typed test root.
func (m StaffMessagingTestModule) SchoolStaffMessagingRouterForTests() chi.Router {
	router := chi.NewRouter()
	router.Use(common.ProblemResponseMiddleware)
	router.Mount("/school/staff-messages", staffmessagingHTTP.NewResource(m.StaffMessaging).SchoolRouter(ProtectedSchoolRoutes))
	return router
}

// NewStaffMessagingTestModule composes the staff messaging over db and
// returns it with the tenant settings it reads.
func NewStaffMessagingTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (StaffMessagingTestModule, owners.SettingsTestModule, error) {
	repos, err := NewStaffMessagingTestRepositories(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	people, err := NewRFIDTestPeople(db)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	messaging, attachments := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		DB:         db.Pool(),
		ThreadRepo: repos.Thread, MessageRepo: repos.Message, ReadRepo: repos.Read,
		Persons: owners.BindStaffSenderNames(people), Session: CommunicationSession{}, Settings: settingsValues.NewStaffMessagingValues(settings.Settings), Broadcaster: realtime.NewHub(slog.Default()),
		Logger: slog.Default(), Notifier: owners.BindCommunicationNotificationSender(delivery.Notifications),
		Preferences: delivery.NotificationPreferences, Observe: func(communicationCompose.Observation) {},
	}, owners.CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	return StaffMessagingTestModule{StaffMessaging: messaging, Attachments: attachments}, settings, nil
}
