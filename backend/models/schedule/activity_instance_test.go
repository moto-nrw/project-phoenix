package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validActivityInstance() *ActivityInstance {
	return &ActivityInstance{
		Date:      timezone.NewDate(2026, 9, 15),
		Title:     "Lernzeit 3a",
		StartTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		RoomID:    42,
		Status:    InstanceStatusPlanned,
	}
}

func TestActivityInstance_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*ActivityInstance)
		wantErr string
	}{
		{
			name:    "valid minimal planned instance",
			mutate:  func(_ *ActivityInstance) {},
			wantErr: "",
		},
		{
			name: "valid with spontaneous flag (no activity_group_id)",
			mutate: func(i *ActivityInstance) {
				i.IsSpontaneous = true
				i.ActivityGroupID = nil
			},
			wantErr: "",
		},
		{
			name:    "missing title",
			mutate:  func(i *ActivityInstance) { i.Title = "" },
			wantErr: "title is required",
		},
		{
			name: "title too long",
			mutate: func(i *ActivityInstance) {
				long := make([]byte, 256)
				for j := range long {
					long[j] = 'a'
				}
				i.Title = string(long)
			},
			wantErr: "title cannot exceed 255 characters",
		},
		{
			name:    "missing date",
			mutate:  func(i *ActivityInstance) { i.Date = timezone.Date("") },
			wantErr: "date is required",
		},
		{
			name:    "missing start_time",
			mutate:  func(i *ActivityInstance) { i.StartTime = time.Time{} },
			wantErr: "start_time is required",
		},
		{
			name:    "missing end_time",
			mutate:  func(i *ActivityInstance) { i.EndTime = time.Time{} },
			wantErr: "end_time is required",
		},
		{
			name: "end_time equals start_time",
			mutate: func(i *ActivityInstance) {
				i.EndTime = i.StartTime
			},
			wantErr: "end_time must be after start_time",
		},
		{
			name: "end_time before start_time",
			mutate: func(i *ActivityInstance) {
				i.EndTime = i.StartTime.Add(-time.Hour)
			},
			wantErr: "end_time must be after start_time",
		},
		{
			name:    "missing room_id",
			mutate:  func(i *ActivityInstance) { i.RoomID = 0 },
			wantErr: "room_id is required",
		},
		{
			name:    "invalid status",
			mutate:  func(i *ActivityInstance) { i.Status = "unknown" },
			wantErr: "invalid instance status",
		},
		{
			name:    "negative required_staff override (#1839)",
			mutate:  func(i *ActivityInstance) { rs := -1; i.RequiredStaff = &rs },
			wantErr: "required_staff cannot be negative",
		},
		{
			name:    "zero required_staff override is valid (#1839)",
			mutate:  func(i *ActivityInstance) { rs := 0; i.RequiredStaff = &rs },
			wantErr: "",
		},
		{
			name:    "positive required_staff override is valid (#1839)",
			mutate:  func(i *ActivityInstance) { rs := 3; i.RequiredStaff = &rs },
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := validActivityInstance()
			tt.mutate(inst)
			err := inst.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsValidInstanceStatus(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidInstanceStatus(InstanceStatusPlanned))
	assert.True(t, IsValidInstanceStatus(InstanceStatusActive))
	assert.True(t, IsValidInstanceStatus(InstanceStatusCompleted))
	assert.True(t, IsValidInstanceStatus(InstanceStatusCancelled))
	assert.False(t, IsValidInstanceStatus(""))
	assert.False(t, IsValidInstanceStatus("unknown"))
}

func TestActivityInstance_IsTemplateBacked(t *testing.T) {
	t.Parallel()

	inst := validActivityInstance()
	assert.False(t, inst.IsTemplateBacked(), "no activity_group_id set")

	gid := int64(7)
	inst.ActivityGroupID = &gid
	assert.True(t, inst.IsTemplateBacked())
}

func TestActivityInstance_IsLive(t *testing.T) {
	t.Parallel()

	inst := validActivityInstance()
	assert.False(t, inst.IsLive(), "default planned")

	inst.Status = InstanceStatusActive
	assert.True(t, inst.IsLive())

	inst.Status = InstanceStatusCompleted
	assert.False(t, inst.IsLive())
}

func TestActivityInstance_AccessorContract(t *testing.T) {
	t.Parallel()

	now := time.Now()
	inst := &ActivityInstance{}
	inst.ID = 17
	inst.CreatedAt = now
	inst.UpdatedAt = now.Add(time.Minute)

	assert.Equal(t, int64(17), inst.GetID())
	assert.Equal(t, now, inst.GetCreatedAt())
	assert.Equal(t, now.Add(time.Minute), inst.GetUpdatedAt())
}
