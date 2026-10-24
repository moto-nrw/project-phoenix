package compose

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/uptrace/bun"
)

// SurveyStaff is the school's staff a staff survey asks (#4781): the
// colleague relation of the team chat.
type SurveyStaff interface{ deliveryPorts.SurveyStaff }

// SurveyWiring is what the surveys need beyond the announcement service.
type SurveyWiring struct {
	DB    *bun.DB
	Staff SurveyStaff
}

// announcementsWithSurveys is the parent announcement capability with the
// surveys of the same owner. Its retention stays reachable the way the
// Worker asks for it.
type announcementsWithSurveys struct {
	communication.ParentAnnouncementCapability
	communication.ParentAnnouncementRetention
	communication.SurveyCapability
}

// withSurveys adds the surveys (#4781). A parent survey is carried by a
// system announcement, so the surveys get an announcement service of their
// own over the same dependencies.
func withSurveys(announcements communication.ParentAnnouncementCapability, cfg deliveryPorts.ServiceConfig, wiring SurveyWiring, runtime deliveryPorts.Runtime, policy deliveryPorts.ParentDeliveryPolicy) (communication.ParentAnnouncementCapability, error) {
	if wiring.DB == nil || wiring.Staff == nil {
		return nil, errors.New("surveys compose: database and staff relation are required")
	}
	retention, ok := announcements.(communication.ParentAnnouncementRetention)
	if !ok {
		return nil, fmt.Errorf("surveys compose: parent announcements %T lack their retention", announcements)
	}
	surveys := deliveryPorts.NewSurveyService(deliveryPorts.SurveyConfig{
		Announcements: cfg,
		Store:         parentpostgres.NewSurveyStore(parentDatabase(wiring.DB)),
		Staff:         wiring.Staff,
	}, runtime, policy)
	return announcementsWithSurveys{ParentAnnouncementCapability: announcements, ParentAnnouncementRetention: retention, SurveyCapability: surveys}, nil
}

// SurveysOf returns the surveys composed with the parent announcements.
func SurveysOf(announcements communication.ParentAnnouncementCapability) (communication.SurveyCapability, error) {
	surveys, ok := announcements.(communication.SurveyCapability)
	if !ok {
		return nil, fmt.Errorf("surveys compose: parent announcements %T were composed without surveys", announcements)
	}
	return surveys, nil
}
