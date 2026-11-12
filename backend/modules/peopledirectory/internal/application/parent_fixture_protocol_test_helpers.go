//go:build testhooks

package application

import (
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/parentfixture"
)

// ParentFixtureProtocol binds the model constants the compatibility contract
// tests compare, without copying their values into the test or public DTO.
func (*Service) ParentFixtureProtocol() parentfixture.ParentFixtureProtocol {
	return parentfixture.ParentFixtureProtocol{
		ParentRequestTypeExcusedAbsence:    peopledirectory.ParentRequestTypeExcusedAbsence,
		ParentRequestEventSubmitted:        peopledirectory.ParentRequestEventSubmitted,
		ParentRequestEventGuardianEdit:     peopledirectory.ParentRequestEventGuardianEdit,
		ParentRequestEventDecided:          peopledirectory.ParentRequestEventDecided,
		ParentRequestEventMarkedDone:       peopledirectory.ParentRequestEventMarkedDone,
		ParentRequestEventCorrected:        peopledirectory.ParentRequestEventCorrected,
		ParentMessageEventRequestCreated:   peopledirectory.ParentMessageEventRequestCreated,
		ParentMessageEventRequestStatus:    peopledirectory.ParentMessageEventRequestStatus,
		ParentMessageSenderGuardian:        peopledirectory.ParentMessageSenderGuardian,
		ParentMessageSenderStaff:           peopledirectory.ParentMessageSenderStaff,
		ParentMessageRequestStatusOpen:     peopledirectory.ParentMessageRequestStatusOpen,
		ParentMessageRequestStatusDone:     peopledirectory.ParentMessageRequestStatusDone,
		ParentMessageRequestStatusRejected: peopledirectory.ParentMessageRequestStatusRejected,
		ParentMessageRequestExcusedAbsence: peopledirectory.ParentMessageRequestExcusedAbsence,
		ParentMessageRequestSickAbsence:    peopledirectory.ParentMessageRequestSickAbsence,
	}
}
