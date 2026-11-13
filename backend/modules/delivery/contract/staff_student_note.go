package contract

// NotificationStaffStudentNoteShared is the notification type of a child's
// note a colleague shared with the team (#6854). The root sends it; Delivery
// registers it for the holders of users:read, who may read the notes.
const NotificationStaffStudentNoteShared = "staff_student_note_shared"
