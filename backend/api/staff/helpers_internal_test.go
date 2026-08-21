package staff

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestedCaregiverPool(t *testing.T) {
	t.Parallel()

	assert.False(t, requestedCaregiverPool(nil))
	assert.False(t, requestedCaregiverPool([]string{"admin"}))
	assert.False(t, requestedCaregiverPool([]string{"admin", "user"}))
	assert.False(t, requestedCaregiverPool([]string{"teacher", "manager"}))
	assert.False(t, requestedCaregiverPool([]string{"teacher"}))
	assert.False(t, requestedCaregiverPool([]string{"user", " teacher ", "staff"}))
	assert.True(t, requestedCaregiverPool([]string{" user "}))
	assert.True(t, requestedCaregiverPool([]string{"STAFF"}))
}

// Avatar path resolution tests — uses centralized common.ResolveStoredPath.

func TestStaffAvatarPath_ValidPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		avatarPath string
		wantSuffix string
	}{
		{"global avatar path", "/uploads/avatars/global/ada.jpg", "uploads/avatars/global/ada.jpg"},
		{"legacy tenant avatar path", "/uploads/avatars/42/grace.png", "uploads/avatars/42/grace.png"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := common.ResolveStoredPath("public", tt.avatarPath, "/uploads/avatars/")
			require.NoError(t, err)
			assert.Contains(t, got, tt.wantSuffix)
		})
	}
}

func TestStaffAvatarPath_InvalidPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		avatarPath string
		wantErr    string
	}{
		{"invalid prefix", "/uploads/not-avatars/file.jpg", "invalid path"},
		{"path traversal", "/uploads/avatars/global/../../secret.txt", "invalid path"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := common.ResolveStoredPath("public", tt.avatarPath, "/uploads/avatars/")
			assert.Empty(t, got)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestCaregiverStaffIDSet(t *testing.T) {
	t.Parallel()

	set := caregiverStaffIDSet([]*users.ActiveCaregiver{
		{StaffID: 11},
		{StaffID: 22},
		{StaffID: 11},
	})

	assert.Len(t, set, 2)
	_, has11 := set[11]
	_, has22 := set[22]
	assert.True(t, has11)
	assert.True(t, has22)
}

func TestNewPersonResponse_IncludesAvatarAndTag(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tagID := "rfid-123"
	accountID := int64(77)
	person := &users.Person{
		Model: base.Model{
			ID:        5,
			CreatedAt: now,
			UpdatedAt: now,
		},
		FirstName: "Ada",
		LastName:  "Lovelace",
		TagID:     &tagID,
		AccountID: &accountID,
	}

	response := newPersonResponse(person, "ada@example.com", "/uploads/avatars/global/ada.jpg")
	require.NotNil(t, response)

	assert.Equal(t, int64(5), response.ID)
	assert.Equal(t, "Ada", response.FirstName)
	assert.Equal(t, "Lovelace", response.LastName)
	assert.Equal(t, "ada@example.com", response.Email)
	assert.Equal(t, "/uploads/avatars/global/ada.jpg", response.Avatar)
	assert.Equal(t, "rfid-123", response.TagID)
	assert.Equal(t, &accountID, response.AccountID)
}

func TestNewPersonResponse_NilPerson(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newPersonResponse(nil, "", ""))
}
