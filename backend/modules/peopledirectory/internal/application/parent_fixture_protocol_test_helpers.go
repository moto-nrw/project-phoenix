//go:build testhooks

package application

import (
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// ParentFixtureProtocol binds the model constants the compatibility contract
// tests compare, without copying their values into the test or public DTO.
func (Engine) ParentFixtureProtocol() peopledirectory.ParentFixtureProtocol {
	return peopledirectory.ParentFixtureProtocol{
		ParentRequestTypeExcusedAbsence:    usersModels.ParentRequestTypeExcusedAbsence,
		ParentRequestEventSubmitted:        usersModels.ParentRequestEventSubmitted,
		ParentRequestEventGuardianEdit:     usersModels.ParentRequestEventGuardianEdit,
		ParentRequestEventDecided:          usersModels.ParentRequestEventDecided,
		ParentRequestEventMarkedDone:       usersModels.ParentRequestEventMarkedDone,
		ParentRequestEventCorrected:        usersModels.ParentRequestEventCorrected,
		ParentMessageEventRequestCreated:   usersModels.ParentMessageEventRequestCreated,
		ParentMessageEventRequestStatus:    usersModels.ParentMessageEventRequestStatus,
		ParentMessageSenderGuardian:        usersModels.ParentMessageSenderGuardian,
		ParentMessageSenderStaff:           usersModels.ParentMessageSenderStaff,
		ParentMessageRequestStatusOpen:     usersModels.ParentMessageRequestStatusOpen,
		ParentMessageRequestStatusDone:     usersModels.ParentMessageRequestStatusDone,
		ParentMessageRequestStatusRejected: usersModels.ParentMessageRequestStatusRejected,
		ParentMessageRequestExcusedAbsence: usersModels.ParentMessageRequestExcusedAbsence,
		ParentMessageRequestSickAbsence:    usersModels.ParentMessageRequestSickAbsence,
	}
}
