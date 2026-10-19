package parentpostgres

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func TestNativeAnnouncementKindUsesCurrentFields(t *testing.T) {
	t.Parallel()
	announcement := &communication.PortalParentAnnouncement{ResponseType: "single_choice", Poll: false, DeclarationMode: true}
	stampAnnouncementKind(announcement)
	if !announcement.Poll || announcement.DeclarationMode {
		t.Fatalf("poll kind not derived: %#v", announcement)
	}
	announcement.ResponseType = "none"
	announcement.DeliveryMode = "declaration"
	stampAnnouncementKind(announcement)
	if announcement.Poll || !announcement.DeclarationMode {
		t.Fatalf("changed declaration kind not derived: %#v", announcement)
	}
	stampAnnouncementKind(nil)
}

func TestNativeMessageCreateRejectsNil(t *testing.T) {
	t.Parallel()
	err := (&messageRepository{}).ParentMessageCreate(context.Background(), nil)
	if err == nil || err.Error() != "create parent message: message is required" {
		t.Fatalf("nil write error = %v", err)
	}
}
