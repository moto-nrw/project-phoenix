package studentpresence

// AttendanceActorKind says who recorded an arrival or a pickup (#4761).
type AttendanceActorKind string

const (
	// AttendanceActorStaff is a staff member of the school; the actor
	// carries the name. Staff who left stay named while the record is kept.
	AttendanceActorStaff AttendanceActorKind = "staff"
	// AttendanceActorDevice is a kiosk device without a person behind it.
	AttendanceActorDevice AttendanceActorKind = "device"
	// AttendanceActorFormerStaff is a staff member whose personal data the
	// automatic deletion of staff erased, or who cannot be found any more.
	AttendanceActorFormerStaff AttendanceActorKind = "former_staff"
	// AttendanceActorAutomatic is a checkout moto recorded itself, such as
	// the end-of-day closing, with neither a person nor a device.
	AttendanceActorAutomatic AttendanceActorKind = "automatic"
)

// AttendanceActor names who recorded an arrival or a pickup. Name is set for
// AttendanceActorStaff only.
type AttendanceActor struct {
	Kind AttendanceActorKind
	Name string
}

// AttendanceRecorders resolves the actors of attendance records from the
// names of the staff members who recorded them. The names come from the
// school in context; a staff ID without a name is a former staff member.
type AttendanceRecorders struct {
	names map[int64]string
}

// NewAttendanceRecorders wraps the staff names of the school in context.
func NewAttendanceRecorders(names map[int64]string) AttendanceRecorders {
	return AttendanceRecorders{names: names}
}

// CheckIn is who recorded the arrival of row: a staff member, or the kiosk
// device when no person is attached.
func (r AttendanceRecorders) CheckIn(row Attendance) AttendanceActor {
	if row.CheckedInBy <= 0 {
		return AttendanceActor{Kind: AttendanceActorDevice}
	}
	return r.staff(row.CheckedInBy)
}

// CheckOut is who recorded the pickup of row; nil while the child is still
// checked in. A checkout with neither a person nor a device is automatic.
func (r AttendanceRecorders) CheckOut(row Attendance) *AttendanceActor {
	if row.CheckOutTime == nil {
		return nil
	}
	var actor AttendanceActor
	switch {
	case row.CheckedOutBy != nil && *row.CheckedOutBy > 0:
		actor = r.staff(*row.CheckedOutBy)
	case row.CheckedOutDeviceID != nil:
		actor = AttendanceActor{Kind: AttendanceActorDevice}
	default:
		actor = AttendanceActor{Kind: AttendanceActorAutomatic}
	}
	return &actor
}

func (r AttendanceRecorders) staff(staffID int64) AttendanceActor {
	if name := r.names[staffID]; name != "" {
		return AttendanceActor{Kind: AttendanceActorStaff, Name: name}
	}
	return AttendanceActor{Kind: AttendanceActorFormerStaff}
}

// AttendanceStaffIDs lists the distinct staff members who recorded the
// arrivals and pickups of rows.
func AttendanceStaffIDs(rows []Attendance) []int64 {
	seen := make(map[int64]bool, len(rows))
	ids := make([]int64, 0, len(rows))
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, row := range rows {
		add(row.CheckedInBy)
		if row.CheckedOutBy != nil {
			add(*row.CheckedOutBy)
		}
	}
	return ids
}
