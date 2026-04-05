package staff

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestedCaregiverPool(t *testing.T) {
	assert.False(t, requestedCaregiverPool(nil))
	assert.False(t, requestedCaregiverPool([]string{"admin"}))
	assert.False(t, requestedCaregiverPool([]string{"admin", "user"}))
	assert.False(t, requestedCaregiverPool([]string{"teacher", "manager"}))
	assert.True(t, requestedCaregiverPool([]string{" user "}))
	assert.True(t, requestedCaregiverPool([]string{"teacher"}))
	assert.True(t, requestedCaregiverPool([]string{"STAFF"}))
	assert.True(t, requestedCaregiverPool([]string{"user", " teacher ", "staff"}))
}

func TestStaffAvatarPathToFilePath_ValidPaths(t *testing.T) {
	publicDir, err := resolvePublicDir()
	require.NoError(t, err)

	testCases := []struct {
		name       string
		avatarPath string
		want       string
	}{
		{
			name:       "global avatar path",
			avatarPath: "/uploads/avatars/global/ada.jpg",
			want: filepath.Join(
				publicDir,
				"uploads",
				"avatars",
				"global",
				"ada.jpg",
			),
		},
		{
			name:       "legacy tenant avatar path",
			avatarPath: "/uploads/avatars/42/grace.png",
			want: filepath.Join(
				publicDir,
				"uploads",
				"avatars",
				"42",
				"grace.png",
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := staffAvatarPathToFilePath(tt.avatarPath)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStaffAvatarPathToFilePath_InvalidPaths(t *testing.T) {
	testCases := []struct {
		name       string
		avatarPath string
		wantErr    string
	}{
		{
			name:       "invalid prefix",
			avatarPath: "/uploads/not-avatars/file.jpg",
			wantErr:    "invalid avatar path",
		},
		{
			name:       "path traversal",
			avatarPath: "/uploads/avatars/global/../../secret.txt",
			wantErr:    "invalid path",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := staffAvatarPathToFilePath(tt.avatarPath)
			assert.Empty(t, got)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestCaregiverStaffIDSet(t *testing.T) {
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
	assert.Nil(t, newPersonResponse(nil, "", ""))
}
