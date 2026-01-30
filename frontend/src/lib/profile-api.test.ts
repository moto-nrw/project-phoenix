import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type {
  Profile,
  ProfileUpdateRequest,
  BackendProfile,
} from "./profile-helpers";

// Mock dependencies before importing the module under test
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

vi.mock("./profile-helpers", () => ({
  mapProfileResponse: vi.fn(),
  mapProfileUpdateRequest: vi.fn(),
}));

// Import after mocks are set up
import { getSession } from "next-auth/react";
import { mapProfileResponse, mapProfileUpdateRequest } from "./profile-helpers";
import {
  fetchProfile,
  updateProfile,
  uploadAvatar,
  deleteAvatar,
} from "./profile-api";

// Type-safe mocks
const mockedGetSession = vi.mocked(getSession);
const mockedMapProfileResponse = vi.mocked(mapProfileResponse);
const mockedMapProfileUpdateRequest = vi.mocked(mapProfileUpdateRequest);

// Sample data
const sampleBackendProfile: BackendProfile = {
  id: 123,
  first_name: "Max",
  last_name: "Mustermann",
  email: "max@example.com",
  username: "maxmuster",
  avatar: "/uploads/avatars/avatar.jpg",
  bio: "Test bio",
  rfid_card: "CARD001",
  settings: '{"theme":"dark"}',
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
  last_login: "2024-01-20T10:00:00Z",
};

const sampleProfile: Profile = {
  id: "123",
  firstName: "Max",
  lastName: "Mustermann",
  email: "max@example.com",
  username: "maxmuster",
  avatar: "/api/me/profile/avatar/avatar.jpg",
  bio: "Test bio",
  rfidCard: "CARD001",
  createdAt: "2024-01-01T00:00:00Z",
  updatedAt: "2024-01-15T12:00:00Z",
  lastLogin: "2024-01-20T10:00:00Z",
  settings: { theme: "dark" },
};

const sampleUpdateRequest: ProfileUpdateRequest = {
  firstName: "Max",
  lastName: "Mustermann",
  bio: "Updated bio",
};

describe("profile-api", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    // Default session mock
    mockedGetSession.mockResolvedValue({
      user: { id: "1", token: "test-token" },
      expires: "2099-01-01",
    });

    // Default mapper mocks
    mockedMapProfileResponse.mockReturnValue(sampleProfile);
    mockedMapProfileUpdateRequest.mockReturnValue({
      first_name: "Max",
      last_name: "Mustermann",
      bio: "Updated bio",
    });
  });

  afterEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleErrorSpy.mockRestore();
  });

  describe("fetchProfile", () => {
    it("fetches profile successfully", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: true,
            message: "Profile fetched",
            data: sampleBackendProfile,
          }),
      });
      global.fetch = mockFetch;

      const result = await fetchProfile();

      expect(mockFetch).toHaveBeenCalledWith("/api/me/profile", {
        method: "GET",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
      });
      expect(mockedMapProfileResponse).toHaveBeenCalledWith(
        sampleBackendProfile,
      );
      expect(result).toEqual(sampleProfile);
    });

    it("throws error when no token is available", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(fetchProfile()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when session has no user", async () => {
      mockedGetSession.mockResolvedValue({
        expires: "2099-01-01",
      } as never);

      await expect(fetchProfile()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when session user has no token", async () => {
      mockedGetSession.mockResolvedValue({
        user: { id: "1" },
        expires: "2099-01-01",
      } as never);

      await expect(fetchProfile()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when response is not ok", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
      });
      global.fetch = mockFetch;

      await expect(fetchProfile()).rejects.toThrow("Failed to fetch profile");
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching profile:",
        expect.any(Error),
      );
    });

    it("throws error when response success is false", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            message: "Profile not found",
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(fetchProfile()).rejects.toThrow("Failed to fetch profile");
    });

    it("throws error when response success is false with no message", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(fetchProfile()).rejects.toThrow("Failed to fetch profile");
    });

    it("throws generic error when fetch fails", async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error("Network error"));
      global.fetch = mockFetch;

      await expect(fetchProfile()).rejects.toThrow("Failed to fetch profile");
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching profile:",
        expect.any(Error),
      );
    });
  });

  describe("updateProfile", () => {
    it("updates profile successfully", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: true,
            message: "Profile updated",
            data: sampleBackendProfile,
          }),
      });
      global.fetch = mockFetch;

      const result = await updateProfile(sampleUpdateRequest);

      expect(mockedMapProfileUpdateRequest).toHaveBeenCalledWith(
        sampleUpdateRequest,
      );
      expect(mockFetch).toHaveBeenCalledWith("/api/me/profile", {
        method: "PUT",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          first_name: "Max",
          last_name: "Mustermann",
          bio: "Updated bio",
        }),
      });
      expect(mockedMapProfileResponse).toHaveBeenCalledWith(
        sampleBackendProfile,
      );
      expect(result).toEqual(sampleProfile);
    });

    it("throws error when no token is available", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(updateProfile(sampleUpdateRequest)).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when response is not ok", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
      });
      global.fetch = mockFetch;

      await expect(updateProfile(sampleUpdateRequest)).rejects.toThrow(
        "Failed to update profile",
      );
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error updating profile:",
        expect.any(Error),
      );
    });

    it("throws error when response success is false", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            message: "Validation error",
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(updateProfile(sampleUpdateRequest)).rejects.toThrow(
        "Failed to update profile",
      );
    });

    it("throws generic error when fetch fails", async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error("Network error"));
      global.fetch = mockFetch;

      await expect(updateProfile(sampleUpdateRequest)).rejects.toThrow(
        "Failed to update profile",
      );
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error updating profile:",
        expect.any(Error),
      );
    });
  });

  describe("uploadAvatar", () => {
    const mockFile = new File(["avatar"], "avatar.jpg", { type: "image/jpeg" });

    it("uploads avatar successfully", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: true,
            message: "Avatar uploaded",
            data: sampleBackendProfile,
          }),
      });
      global.fetch = mockFetch;

      const result = await uploadAvatar(mockFile);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/me/profile/avatar",
        expect.objectContaining({
          method: "POST",
          headers: {
            Authorization: "Bearer test-token",
          },
        }),
      );

      // Verify FormData was created with the file
      const callArgs = mockFetch.mock.calls[0];
      expect(callArgs).toBeDefined();
      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
      const body = callArgs?.[1]?.body as FormData;
      expect(body).toBeInstanceOf(FormData);
      expect(body.get("avatar")).toBe(mockFile);

      expect(mockedMapProfileResponse).toHaveBeenCalledWith(
        sampleBackendProfile,
      );
      expect(result).toEqual(sampleProfile);
    });

    it("throws error when no token is available", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(uploadAvatar(mockFile)).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error with message from response when upload fails", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        json: () => Promise.resolve({ error: "File too large" }),
      });
      global.fetch = mockFetch;

      await expect(uploadAvatar(mockFile)).rejects.toThrow("File too large");
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error uploading avatar:",
        expect.any(Error),
      );
    });

    it("throws error with status when JSON parsing fails", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error("Parse error")),
      });
      global.fetch = mockFetch;

      await expect(uploadAvatar(mockFile)).rejects.toThrow(
        "HTTP error! status: 500",
      );
    });

    it("throws error when response success is false", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            message: "Upload failed",
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(uploadAvatar(mockFile)).rejects.toThrow("Upload failed");
    });

    it("preserves error message when fetch throws Error", async () => {
      const mockFetch = vi
        .fn()
        .mockRejectedValue(new Error("Custom network error"));
      global.fetch = mockFetch;

      await expect(uploadAvatar(mockFile)).rejects.toThrow(
        "Custom network error",
      );
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error uploading avatar:",
        expect.any(Error),
      );
    });

    it("throws generic error when fetch throws non-Error", async () => {
      const mockFetch = vi.fn().mockRejectedValue("String error");
      global.fetch = mockFetch;

      await expect(uploadAvatar(mockFile)).rejects.toThrow(
        "Failed to upload avatar",
      );
    });
  });

  describe("deleteAvatar", () => {
    it("deletes avatar successfully", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: true,
            message: "Avatar deleted",
            data: { ...sampleBackendProfile, avatar: undefined },
          }),
      });
      global.fetch = mockFetch;

      const result = await deleteAvatar();

      expect(mockFetch).toHaveBeenCalledWith("/api/me/profile/avatar", {
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
      });
      expect(mockedMapProfileResponse).toHaveBeenCalled();
      expect(result).toEqual(sampleProfile);
    });

    it("throws error when no token is available", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(deleteAvatar()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when response is not ok", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
      });
      global.fetch = mockFetch;

      await expect(deleteAvatar()).rejects.toThrow("Failed to delete avatar");
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error deleting avatar:",
        expect.any(Error),
      );
    });

    it("throws error when response success is false", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            message: "Delete failed",
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(deleteAvatar()).rejects.toThrow("Failed to delete avatar");
    });

    it("throws error when response success is false with no message", async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            success: false,
            data: null,
          }),
      });
      global.fetch = mockFetch;

      await expect(deleteAvatar()).rejects.toThrow("Failed to delete avatar");
    });

    it("throws generic error when fetch fails", async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error("Network error"));
      global.fetch = mockFetch;

      await expect(deleteAvatar()).rejects.toThrow("Failed to delete avatar");
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error deleting avatar:",
        expect.any(Error),
      );
    });
  });
});
