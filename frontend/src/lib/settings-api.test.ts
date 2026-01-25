import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next-auth/react before importing the module under test
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

import { getSession } from "next-auth/react";
import {
  fetchDefinitions,
  fetchDefinition,
  fetchUserSettings,
  updateUserSetting,
  fetchSystemSettings,
  updateSystemSetting,
  resetSystemSetting,
  fetchOGSettings,
  updateOGSetting,
  resetOGSetting,
  initializeDefinitions,
  fetchSettingHistory,
  fetchOGSettingHistory,
  fetchOGKeyHistory,
} from "./settings-api";
import type {
  BackendSettingDefinition,
  BackendResolvedSetting,
  BackendSettingChange,
} from "./settings-helpers";

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Sample backend data
const sampleBackendDefinition: BackendSettingDefinition = {
  id: 1,
  key: "session.timeout",
  type: "int",
  default_value: 30,
  category: "session",
  description: "Session timeout in minutes",
  allowed_scopes: ["system", "og"],
  scope_permissions: {},
  sort_order: 10,
};

const sampleBackendResolvedSetting: BackendResolvedSetting = {
  key: "session.timeout",
  value: 30,
  type: "int",
  category: "session",
  is_default: true,
  is_active: true,
  can_modify: true,
};

const sampleBackendSettingChange: BackendSettingChange = {
  id: 100,
  setting_key: "session.timeout",
  scope_type: "system",
  change_type: "update",
  old_value: 20,
  new_value: 30,
  created_at: "2024-01-15T10:00:00Z",
};

// Mock session
const mockSession = {
  user: {
    token: "test-token-123",
  },
};

describe("settings-api", () => {
  const mockedGetSession = vi.mocked(getSession);

  beforeEach(() => {
    vi.clearAllMocks();
    mockedGetSession.mockResolvedValue(mockSession as never);
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // Helper to create mock response
  function createMockResponse<T>(data: T, ok = true, status = 200) {
    return {
      ok,
      status,
      json: vi.fn().mockResolvedValue({ data }),
    };
  }

  describe("fetchDefinitions", () => {
    it("fetches all definitions successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendDefinition]),
      );

      const result = await fetchDefinitions();

      expect(mockFetch).toHaveBeenCalledWith("/api/settings/definitions", {
        headers: {
          Authorization: "Bearer test-token-123",
          "Content-Type": "application/json",
        },
      });
      expect(result).toHaveLength(1);
      expect(result[0]?.key).toBe("session.timeout");
      expect(result[0]?.id).toBe("1"); // Mapped to string
    });

    it("passes filters as query params", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendDefinition]),
      );

      await fetchDefinitions({ category: "session" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/definitions?category=session",
        expect.any(Object),
      );
    });

    it("throws error when not authenticated", async () => {
      mockedGetSession.mockResolvedValueOnce(null as never);

      await expect(fetchDefinitions()).rejects.toThrow("Not authenticated");
    });
  });

  describe("fetchDefinition", () => {
    it("fetches single definition successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(sampleBackendDefinition),
      );

      const result = await fetchDefinition("session.timeout");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/definitions/session.timeout",
        expect.any(Object),
      );
      expect(result?.key).toBe("session.timeout");
    });

    it("returns null for 404 response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: vi.fn().mockResolvedValue({}),
      });

      const result = await fetchDefinition("nonexistent");

      expect(result).toBeNull();
    });
  });

  describe("fetchUserSettings", () => {
    it("fetches user settings successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendResolvedSetting]),
      );

      const result = await fetchUserSettings();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/user/me",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.key).toBe("session.timeout");
    });
  });

  describe("updateUserSetting", () => {
    it("updates user setting successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await updateUserSetting("session.timeout", 45, "Testing");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/user/me/session.timeout",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ value: 45, reason: "Testing" }),
        }),
      );
    });

    it("throws error on failure", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: vi.fn().mockResolvedValue({ message: "Invalid value" }),
      });

      await expect(updateUserSetting("test", "invalid")).rejects.toThrow(
        "Invalid value",
      );
    });
  });

  describe("fetchSystemSettings", () => {
    it("fetches system settings successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendResolvedSetting]),
      );

      const result = await fetchSystemSettings();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/system",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });
  });

  describe("updateSystemSetting", () => {
    it("updates system setting successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await updateSystemSetting("session.timeout", 60);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/system/session.timeout",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ value: 60, reason: undefined }),
        }),
      );
    });
  });

  describe("resetSystemSetting", () => {
    it("resets system setting successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await resetSystemSetting("session.timeout");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/system/session.timeout",
        expect.objectContaining({
          method: "DELETE",
        }),
      );
    });

    it("throws error on failure", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: vi.fn().mockResolvedValue({ message: "Reset failed" }),
      });

      await expect(resetSystemSetting("test")).rejects.toThrow("Reset failed");
    });
  });

  describe("fetchOGSettings", () => {
    it("fetches OG settings successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendResolvedSetting]),
      );

      const result = await fetchOGSettings("123");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });
  });

  describe("updateOGSetting", () => {
    it("updates OG setting successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await updateOGSetting("123", "session.timeout", 45);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123/session.timeout",
        expect.objectContaining({
          method: "PUT",
        }),
      );
    });
  });

  describe("resetOGSetting", () => {
    it("resets OG setting successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await resetOGSetting("123", "session.timeout");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123/session.timeout",
        expect.objectContaining({
          method: "DELETE",
        }),
      );
    });
  });

  describe("initializeDefinitions", () => {
    it("initializes definitions successfully", async () => {
      mockFetch.mockResolvedValueOnce({ ok: true, json: vi.fn() });

      await initializeDefinitions();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/initialize",
        expect.objectContaining({
          method: "POST",
        }),
      );
    });

    it("throws error on failure", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: vi.fn().mockResolvedValue({}),
      });

      await expect(initializeDefinitions()).rejects.toThrow(
        "Failed to initialize definitions",
      );
    });
  });

  describe("fetchSettingHistory", () => {
    it("fetches history successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendSettingChange]),
      );

      const result = await fetchSettingHistory();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/history",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.settingKey).toBe("session.timeout");
    });

    it("passes filters as query params", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendSettingChange]),
      );

      await fetchSettingHistory({
        scopeType: "system",
        scopeId: "1",
        limit: 10,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/history?scope_type=system&scope_id=1&limit=10",
        expect.any(Object),
      );
    });
  });

  describe("fetchOGSettingHistory", () => {
    it("fetches OG history successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendSettingChange]),
      );

      const result = await fetchOGSettingHistory("123", 20);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123/history?limit=20",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("works without limit parameter", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendSettingChange]),
      );

      await fetchOGSettingHistory("123");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123/history",
        expect.any(Object),
      );
    });
  });

  describe("fetchOGKeyHistory", () => {
    it("fetches OG key history successfully", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse([sampleBackendSettingChange]),
      );

      const result = await fetchOGKeyHistory("123", "session.timeout", 10);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/settings/og/123/session.timeout/history?limit=10",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });
  });
});
