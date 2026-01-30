import { describe, it, expect } from "vitest";
import {
  mapProfileResponse,
  mapProfileUpdateRequest,
  type BackendProfile,
  type ProfileUpdateRequest,
  type ProfileSettings,
} from "./profile-helpers";

describe("profile-helpers", () => {
  describe("mapProfileResponse", () => {
    it("should map backend profile with settings as valid JSON string", () => {
      const settingsObj: ProfileSettings = {
        theme: "dark",
        language: "de",
        notifications: {
          email: true,
          push: false,
          activities: true,
          roomChanges: false,
        },
        privacy: {
          showEmail: false,
          showProfile: true,
        },
      };

      const backendProfile: BackendProfile = {
        id: 123,
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        username: "johndoe",
        avatar: "/uploads/avatars/profile.jpg",
        bio: "Test bio",
        rfid_card: "RFID123",
        settings: JSON.stringify(settingsObj),
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
        last_login: "2024-01-15T09:00:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result).toEqual({
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john.doe@example.com",
        username: "johndoe",
        avatar: "/api/me/profile/avatar/profile.jpg",
        bio: "Test bio",
        rfidCard: "RFID123",
        createdAt: "2024-01-01T00:00:00Z",
        updatedAt: "2024-01-15T10:30:00Z",
        lastLogin: "2024-01-15T09:00:00Z",
        settings: settingsObj,
      });
    });

    it("should map backend profile with settings as object", () => {
      const settingsObj: ProfileSettings = {
        theme: "light",
        language: "en",
      };

      const backendProfile: BackendProfile = {
        id: 456,
        first_name: "Jane",
        last_name: "Smith",
        email: "jane.smith@example.com",
        settings: settingsObj as unknown as Record<string, unknown>,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.settings).toEqual(settingsObj);
    });

    it("should handle invalid JSON string in settings", () => {
      const backendProfile: BackendProfile = {
        id: 789,
        first_name: "Invalid",
        last_name: "Settings",
        email: "invalid@example.com",
        settings: "{invalid json}",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.settings).toBeUndefined();
    });

    it("should handle profile with no settings", () => {
      const backendProfile: BackendProfile = {
        id: 100,
        first_name: "No",
        last_name: "Settings",
        email: "nosettings@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.settings).toBeUndefined();
    });

    it("should rewrite avatar path starting with /uploads/", () => {
      const backendProfile: BackendProfile = {
        id: 200,
        first_name: "Avatar",
        last_name: "Test",
        email: "avatar@example.com",
        avatar: "/uploads/avatars/user123.png",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.avatar).toBe("/api/me/profile/avatar/user123.png");
    });

    it("should not rewrite avatar path not starting with /uploads/", () => {
      const backendProfile: BackendProfile = {
        id: 300,
        first_name: "External",
        last_name: "Avatar",
        email: "external@example.com",
        avatar: "https://example.com/avatar.jpg",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.avatar).toBe("https://example.com/avatar.jpg");
    });

    it("should handle null avatar", () => {
      const backendProfile: BackendProfile = {
        id: 400,
        first_name: "No",
        last_name: "Avatar",
        email: "noavatar@example.com",
        avatar: undefined,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result.avatar).toBeUndefined();
    });

    it("should handle avatar path with no filename", () => {
      const backendProfile: BackendProfile = {
        id: 500,
        first_name: "Empty",
        last_name: "Path",
        email: "emptypath@example.com",
        avatar: "/uploads/",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      // When split('/').pop() returns empty string, no rewrite occurs
      expect(result.avatar).toBe("/uploads/");
    });

    it("should handle missing optional fields", () => {
      const backendProfile: BackendProfile = {
        id: 600,
        first_name: "Minimal",
        last_name: "Profile",
        email: "minimal@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapProfileResponse(backendProfile);

      expect(result).toEqual({
        id: "600",
        firstName: "Minimal",
        lastName: "Profile",
        email: "minimal@example.com",
        username: undefined,
        avatar: undefined,
        bio: undefined,
        rfidCard: undefined,
        createdAt: "2024-01-01T00:00:00Z",
        updatedAt: "2024-01-15T10:30:00Z",
        lastLogin: undefined,
        settings: undefined,
      });
    });
  });

  describe("mapProfileUpdateRequest", () => {
    it("should map all fields", () => {
      const settings: ProfileSettings = {
        theme: "dark",
        language: "de",
      };

      const request: ProfileUpdateRequest = {
        firstName: "Updated",
        lastName: "Name",
        username: "updated_username",
        bio: "Updated bio",
        avatar: "/new/avatar.jpg",
        settings,
      };

      const result = mapProfileUpdateRequest(request);

      expect(result).toEqual({
        first_name: "Updated",
        last_name: "Name",
        username: "updated_username",
        bio: "Updated bio",
        avatar: "/new/avatar.jpg",
        settings: JSON.stringify(settings),
      });
    });

    it("should map partial fields", () => {
      const request: ProfileUpdateRequest = {
        firstName: "Only",
        lastName: "Names",
      };

      const result = mapProfileUpdateRequest(request);

      expect(result).toEqual({
        first_name: "Only",
        last_name: "Names",
      });
    });

    it("should stringify settings object", () => {
      const settings: ProfileSettings = {
        theme: "light",
        notifications: {
          email: true,
          push: false,
        },
      };

      const request: ProfileUpdateRequest = {
        settings,
      };

      const result = mapProfileUpdateRequest(request);

      expect(result.settings).toBe(JSON.stringify(settings));
    });

    it("should handle empty request", () => {
      const request: ProfileUpdateRequest = {};

      const result = mapProfileUpdateRequest(request);

      expect(result).toEqual({});
    });

    it("should include undefined values when explicitly set", () => {
      const request: ProfileUpdateRequest = {
        firstName: "Test",
        bio: undefined,
      };

      const result = mapProfileUpdateRequest(request);

      expect(result).toEqual({
        first_name: "Test",
        bio: undefined,
      });
    });
  });
});
