package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		wantErr bool
	}{
		{
			name: "valid profile with account ID only",
			profile: &Profile{
				AccountID: 1,
			},
			wantErr: false,
		},
		{
			name: "valid profile with all fields",
			profile: &Profile{
				AccountID: 1,
				Avatar:    "avatar.png",
				Bio:       "Hello, world!",
				Settings:  `{"theme": "dark"}`,
			},
			wantErr: false,
		},
		{
			name: "missing account ID",
			profile: &Profile{
				AccountID: 0,
			},
			wantErr: true,
		},
		{
			name: "negative account ID",
			profile: &Profile{
				AccountID: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid settings JSON",
			profile: &Profile{
				AccountID: 1,
				Settings:  "not valid json",
			},
			wantErr: true,
		},
		{
			name: "empty settings is valid",
			profile: &Profile{
				AccountID: 1,
				Settings:  "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Profile.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfile_TableName(t *testing.T) {
	profile := &Profile{}
	expected := "users.profiles"

	if got := profile.TableName(); got != expected {
		t.Errorf("Profile.TableName() = %q, want %q", got, expected)
	}
}

func TestProfile_SetAccount(t *testing.T) {
	t.Run("set with account", func(t *testing.T) {
		profile := &Profile{}

		account := &auth.Account{
			Model: base.Model{ID: 42},
		}

		profile.SetAccount(account)

		if profile.Account != account {
			t.Error("SetAccount should set the Account field")
		}
		if profile.AccountID != 42 {
			t.Errorf("SetAccount should set AccountID = 42, got %d", profile.AccountID)
		}
	})

	t.Run("set with nil account", func(t *testing.T) {
		profile := &Profile{
			AccountID: 10,
		}

		profile.SetAccount(nil)

		if profile.Account != nil {
			t.Error("SetAccount(nil) should set Account to nil")
		}
		// AccountID should remain unchanged when setting nil
		if profile.AccountID != 10 {
			t.Errorf("SetAccount(nil) should not change AccountID, got %d", profile.AccountID)
		}
	})
}

func TestProfile_GetSetting(t *testing.T) {
	t.Run("get existing setting", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  `{"theme": "dark", "notifications": true}`,
		}

		value, exists := profile.GetSetting("theme")
		if !exists {
			t.Error("GetSetting should find existing key")
		}
		if value != "dark" {
			t.Errorf("GetSetting() = %v, want %v", value, "dark")
		}
	})

	t.Run("get non-existing setting", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  `{"theme": "dark"}`,
		}

		_, exists := profile.GetSetting("language")
		if exists {
			t.Error("GetSetting should not find non-existing key")
		}
	})

	t.Run("get from empty settings", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  "",
		}

		_, exists := profile.GetSetting("theme")
		if exists {
			t.Error("GetSetting should not find key in empty settings")
		}
	})
}

func TestProfile_SetSetting(t *testing.T) {
	t.Run("set new setting on empty profile", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
		}

		err := profile.SetSetting("theme", "dark")
		if err != nil {
			t.Errorf("SetSetting() error = %v", err)
		}

		value, exists := profile.GetSetting("theme")
		if !exists || value != "dark" {
			t.Errorf("GetSetting() = %v, %v, want dark, true", value, exists)
		}
	})

	t.Run("update existing setting", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  `{"theme": "light"}`,
		}

		err := profile.SetSetting("theme", "dark")
		if err != nil {
			t.Errorf("SetSetting() error = %v", err)
		}

		value, _ := profile.GetSetting("theme")
		if value != "dark" {
			t.Errorf("GetSetting() = %v, want dark", value)
		}
	})

	t.Run("set setting with invalid existing JSON", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  "invalid json",
		}

		// Should handle gracefully by creating new settings map
		err := profile.SetSetting("theme", "dark")
		if err != nil {
			t.Errorf("SetSetting() error = %v", err)
		}

		value, exists := profile.GetSetting("theme")
		if !exists || value != "dark" {
			t.Errorf("GetSetting() = %v, %v, want dark, true", value, exists)
		}
	})
}

func TestProfile_DeleteSetting(t *testing.T) {
	t.Run("delete existing setting", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
			Settings:  `{"theme": "dark", "notifications": true}`,
		}

		profile.DeleteSetting("theme")

		_, exists := profile.GetSetting("theme")
		if exists {
			t.Error("DeleteSetting should remove the key")
		}

		// Other keys should remain
		value, exists := profile.GetSetting("notifications")
		if !exists || value != true {
			t.Errorf("DeleteSetting should not affect other keys")
		}
	})

	t.Run("delete from empty settings", func(t *testing.T) {
		profile := &Profile{
			AccountID: 1,
		}

		// Should not panic
		profile.DeleteSetting("theme")
	})
}

func TestProfile_HasAvatar(t *testing.T) {
	tests := []struct {
		name     string
		avatar   string
		expected bool
	}{
		{
			name:     "has avatar",
			avatar:   "avatar.png",
			expected: true,
		},
		{
			name:     "no avatar",
			avatar:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &Profile{Avatar: tt.avatar}
			if got := profile.HasAvatar(); got != tt.expected {
				t.Errorf("Profile.HasAvatar() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProfile_HasBio(t *testing.T) {
	tests := []struct {
		name     string
		bio      string
		expected bool
	}{
		{
			name:     "has bio",
			bio:      "Hello, world!",
			expected: true,
		},
		{
			name:     "no bio",
			bio:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &Profile{Bio: tt.bio}
			if got := profile.HasBio(); got != tt.expected {
				t.Errorf("Profile.HasBio() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProfile_EntityInterface(t *testing.T) {
	now := time.Now()
	profile := &Profile{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		AccountID: 1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := profile.GetID()
		if got != int64(123) {
			t.Errorf("Profile.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := profile.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Profile.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := profile.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Profile.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
