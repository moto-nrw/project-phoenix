package activities

import (
	"testing"
	"time"
)

// int64Ptr returns a pointer to the given int64 value.
func int64Ptr(v int64) *int64 { return &v }
func intPtr(v int) *int       { return &v }

func TestGroupValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: false,
		},
		{
			name: "Valid group with planned room",
			group: &Group{
				Name:            "Test Group with Room",
				MaxParticipants: 15,
				IsOpen:          false,
				CategoryID:      2,
				CreatedBy:       int64Ptr(1),
				PlannedRoomID:   func() *int64 { id := int64(3); return &id }(),
			},
			wantErr: false,
		},
		{
			name: "Valid group with nil CreatedBy (system-created)",
			group: &Group{
				Name:            "System Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       nil,
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			group: &Group{
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Unlimited participants",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 0,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: false,
		},
		{
			name: "Invalid max participants (negative)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: -5,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Negative required staff override (#1839)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
				RequiredStaff:   intPtr(-1),
			},
			wantErr: true,
		},
		{
			name: "Zero required staff override is valid (#1839)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
				RequiredStaff:   intPtr(0),
			},
			wantErr: false,
		},
		{
			name: "Nil required staff override is valid (derive) (#1839)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
				RequiredStaff:   nil,
			},
			wantErr: false,
		},
		{
			name: "Missing category ID",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Invalid category ID",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      -1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupHasAvailableSpots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		group                  *Group
		currentEnrollmentCount int
		want                   bool
	}{
		{
			name: "Has available spots",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 5,
			want:                   true,
		},
		{
			name: "No available spots (full)",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 10,
			want:                   false,
		},
		{
			name: "No available spots (over capacity)",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 12,
			want:                   false,
		},
		{
			name: "Unlimited group always has available spots",
			group: &Group{
				MaxParticipants: 0,
			},
			currentEnrollmentCount: 1000,
			want:                   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.HasAvailableSpots(tt.currentEnrollmentCount); got != tt.want {
				t.Errorf("Group.HasAvailableSpots() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_GetID(t *testing.T) {
	t.Parallel()

	group := &Group{
		Model:           Model{ID: 42},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if group.ID != 42 {
		t.Errorf("ID = %v, want 42", group.ID)
	}
}

func TestGroup_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	group := &Group{
		Model:           Model{CreatedAt: now},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if got := group.CreatedAt; !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGroup_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	group := &Group{
		Model:           Model{UpdatedAt: now},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if got := group.UpdatedAt; !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}

// TestGroupValidate_CreatedBy verifies that Validate() does not require CreatedBy.
// CreatedBy is nullable (system-created groups have created_by = NULL).
func TestGroupValidate_CreatedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group with CreatedBy set",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(42),
			},
			wantErr: false,
		},
		{
			name: "Valid group with nil CreatedBy (system-created)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroup_IsOwnedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *Group
		staffID int64
		want    bool
	}{
		{
			name: "Staff is owner",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Staff is not owner",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 99,
			want:    false,
		},
		{
			name: "Staff ID is zero",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 0,
			want:    false,
		},
		{
			name: "System-created group (nil CreatedBy) is not owned by any staff",
			group: &Group{
				CreatedBy: nil,
			},
			staffID: 42,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsOwnedBy(tt.staffID); got != tt.want {
				t.Errorf("Group.IsOwnedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_IsSupervisedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *Group
		staffID int64
		want    bool
	}{
		{
			name: "Staff is a supervisor",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					{StaffID: 42},
					{StaffID: 30},
				},
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Staff is not a supervisor",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					{StaffID: 20},
					{StaffID: 30},
				},
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "No supervisors",
			group: &Group{
				Supervisors: []*SupervisorPlanned{},
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "Nil supervisors slice",
			group: &Group{
				Supervisors: nil,
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "Supervisor slice contains nil entry",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					nil,
					{StaffID: 42},
				},
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Supervisor slice only contains nil entries",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					nil,
					nil,
				},
			},
			staffID: 42,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsSupervisedBy(tt.staffID); got != tt.want {
				t.Errorf("Group.IsSupervisedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// int16Ptr returns a pointer to the given int16 value.
func int16Ptr(v int16) *int16 { return &v }

// stringPtr returns a pointer to the given string value.
func stringPtr(v string) *string { return &v }

func baseValidGroup() *Group {
	return &Group{
		Name:            "Test Group",
		MaxParticipants: 10,
		CategoryID:      1,
	}
}

func TestGroupValidate_TargetGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   func() *Group
		wantErr bool
	}{
		{
			name: "Zero-value TargetGroupType (empty string) is valid - predates this field",
			group: func() *Group {
				return baseValidGroup()
			},
			wantErr: false,
		},
		{
			name: "Explicit none with no value fields is valid",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeNone
				return g
			},
			wantErr: false,
		},
		{
			name: "Invalid target group type is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = "not-a-real-type"
				return g
			},
			wantErr: true,
		},
		{
			name: "Jahrgang with grade level is valid",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeJahrgang
				g.TargetGradeLevel = int16Ptr(3)
				return g
			},
			wantErr: false,
		},
		{
			name: "Jahrgang without grade level is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeJahrgang
				return g
			},
			wantErr: true,
		},
		{
			name: "Jahrgang with non-positive grade is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeJahrgang
				g.TargetGradeLevel = int16Ptr(0)
				return g
			},
			wantErr: true,
		},
		{
			name: "Jahrgang above supported grade 13 is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeJahrgang
				g.TargetGradeLevel = int16Ptr(14)
				return g
			},
			wantErr: true,
		},
		{
			name: "Jahrgang with school class set is rejected (cross-field)",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeJahrgang
				g.TargetGradeLevel = int16Ptr(3)
				g.TargetSchoolClass = stringPtr("3a")
				return g
			},
			wantErr: true,
		},
		{
			name: "Klasse with school class is valid",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeKlasse
				g.TargetSchoolClass = stringPtr("3a")
				return g
			},
			wantErr: false,
		},
		{
			name: "Klasse without school class is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeKlasse
				return g
			},
			wantErr: true,
		},
		{
			name: "Klasse with empty-string school class is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeKlasse
				g.TargetSchoolClass = stringPtr("")
				return g
			},
			wantErr: true,
		},
		{
			name: "Klasse with whitespace-only school class is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeKlasse
				g.TargetSchoolClass = stringPtr(" \t ")
				return g
			},
			wantErr: true,
		},
		{
			name: "Gruppe with education_group_id is valid",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeGruppe
				g.EducationGroupID = int64Ptr(7)
				return g
			},
			wantErr: false,
		},
		{
			name: "Gruppe without education_group_id is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeGruppe
				return g
			},
			wantErr: true,
		},
		{
			name: "Angebot with no value fields is valid",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeAngebot
				return g
			},
			wantErr: false,
		},
		{
			name: "Angebot with a grade level set is rejected",
			group: func() *Group {
				g := baseValidGroup()
				g.TargetGroupType = TargetGroupTypeAngebot
				g.TargetGradeLevel = int16Ptr(2)
				return g
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group().Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidTargetGroupType(t *testing.T) {
	t.Parallel()

	valid := []string{"", TargetGroupTypeJahrgang, TargetGroupTypeKlasse, TargetGroupTypeGruppe, TargetGroupTypeAngebot, TargetGroupTypeNone}
	for _, v := range valid {
		if !IsValidTargetGroupType(v) {
			t.Errorf("IsValidTargetGroupType(%q) = false, want true", v)
		}
	}

	invalid := []string{"jahrgangg", "KLASSE", "unknown"}
	for _, v := range invalid {
		if IsValidTargetGroupType(v) {
			t.Errorf("IsValidTargetGroupType(%q) = true, want false", v)
		}
	}
}

func TestGroupValidateTargetGroup_TrimsSchoolClass(t *testing.T) {
	t.Parallel()

	class := "  Klasse 3a  "
	group := baseValidGroup()
	group.TargetGroupType = TargetGroupTypeKlasse
	group.TargetSchoolClass = &class

	if err := group.ValidateTargetGroup(); err != nil {
		t.Fatalf("ValidateTargetGroup() unexpected error: %v", err)
	}
	if got := *group.TargetSchoolClass; got != "Klasse 3a" {
		t.Fatalf("TargetSchoolClass = %q, want trimmed class", got)
	}
}

func TestGroupValidateTargetGroup_CanonicalizesEmptyType(t *testing.T) {
	t.Parallel()

	group := baseValidGroup()

	if err := group.ValidateTargetGroup(); err != nil {
		t.Fatalf("ValidateTargetGroup() unexpected error: %v", err)
	}
	if group.TargetGroupType != TargetGroupTypeNone {
		t.Fatalf("TargetGroupType = %q, want %q", group.TargetGroupType, TargetGroupTypeNone)
	}
}
