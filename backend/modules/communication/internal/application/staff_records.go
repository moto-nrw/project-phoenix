package application

import "github.com/moto-nrw/project-phoenix/modules/communication"

// Native workflow values have their own identity, without aliases or ORM data.
type StaffMessage communication.StaffMessageRecord
type StaffMessageThread communication.StaffThreadRecord
type StaffInboxThread communication.StaffInboxRecord
type MessageableStaff communication.AddressableStaffRecord

// These are the exact owned public facets supplied by the module root.
type StaffThreadSource interface {
	communication.StaffThreadRecords
}
type StaffMessageSource interface {
	communication.StaffMessageRecords
}
type StaffReadSource interface{ communication.StaffReadRecords }
type StaffSenderNames interface{ communication.StaffSenderNames }

func StaffMessageFromRecord(value *communication.StaffMessageRecord) *StaffMessage {
	return (*StaffMessage)(value)
}
func StaffMessageToRecord(value *StaffMessage) *communication.StaffMessageRecord {
	return (*communication.StaffMessageRecord)(value)
}
func StaffThreadFromRecord(value *communication.StaffThreadRecord) *StaffMessageThread {
	return (*StaffMessageThread)(value)
}
func StaffInboxFromRecord(value *communication.StaffInboxRecord) *StaffInboxThread {
	return (*StaffInboxThread)(value)
}
func MessageableStaffFromRecord(value *communication.AddressableStaffRecord) *MessageableStaff {
	return (*MessageableStaff)(value)
}
