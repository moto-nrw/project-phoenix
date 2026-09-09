package data

import "github.com/moto-nrw/project-phoenix/modules/devicescan"

// Preserve the established kiosk response names at the HTTP seam.
type DeviceTeacherResponse = devicescan.Teacher
type TeacherStudentResponse = devicescan.DirectoryStudent

// TeacherActivityResponse represents an activity in the teacher's activity list
type TeacherActivityResponse = devicescan.DirectoryActivity

// DeviceRoomResponse represents a room available for RFID device selection
type DeviceRoomResponse = devicescan.AvailableRoom

type RFIDTagAssignmentResponse = devicescan.TagAssignment
type RFIDTagAssignedPerson = devicescan.TagAssignedPerson
type RFIDTagAssignedStudent = devicescan.TagAssignedStudent
