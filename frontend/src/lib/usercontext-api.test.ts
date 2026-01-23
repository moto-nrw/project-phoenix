/**
 * Tests for usercontext-api.ts
 *
 * Tests the userContextService functions:
 * - getMyEducationalGroups
 * - getMySupervisedGroups
 * - hasEducationalGroups
 * - getCurrentUser
 * - getCurrentStaff
 * - getMyActiveGroups
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock dependencies
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
  },
}));

vi.mock("./fetch-with-auth", () => ({
  fetchWithAuth: vi.fn(),
}));

// Import after mocking
import { userContextService } from "./usercontext-api";
import { fetchWithAuth } from "./fetch-with-auth";

describe("userContextService", () => {
  const fetchWithAuthMock = vi.mocked(fetchWithAuth);

  beforeEach(() => {
    vi.clearAllMocks();
    // Simulate browser environment
    Object.defineProperty(globalThis, "window", {
      value: {},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    // Reset window to undefined for server-side tests
    Object.defineProperty(globalThis, "window", {
      value: undefined,
      writable: true,
      configurable: true,
    });
  });

  describe("getMyEducationalGroups", () => {
    it("returns mapped educational groups on success", async () => {
      const backendData = [
        {
          id: 1,
          name: "Group 1",
          room_id: 10,
          room: { id: 10, name: "Raum 101" },
        },
      ];

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getMyEducationalGroups();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.name).toBe("Group 1");
      expect(result[0]?.room?.name).toBe("Raum 101");
    });

    it("returns empty array when data is null", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "", data: null }),
      } as Response);

      const result = await userContextService.getMyEducationalGroups();

      expect(result).toEqual([]);
    });

    it("returns empty array when data is not an array", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: "invalid" }),
      } as Response);

      const result = await userContextService.getMyEducationalGroups();

      expect(result).toEqual([]);
    });

    it("throws error on fetch failure", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Server error"),
      } as Response);

      await expect(
        userContextService.getMyEducationalGroups(),
      ).rejects.toThrow();
    });

    // Note: With BetterAuth, authentication is handled via cookies (credentials: "include")
    // The optional token parameter has been removed as it's no longer needed
  });

  describe("getMySupervisedGroups", () => {
    it("returns mapped supervised groups on success", async () => {
      const backendData = [
        {
          id: 2,
          name: "Supervised Group",
          room_id: 20,
          room: { id: 20, name: "Raum 202" },
        },
      ];

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getMySupervisedGroups();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("2");
      expect(result[0]?.name).toBe("Supervised Group");
    });

    it("handles null response data", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "", data: null }),
      } as Response);

      const result = await userContextService.getMySupervisedGroups();

      expect(result).toEqual([]);
    });
  });

  describe("hasEducationalGroups", () => {
    it("returns true when groups exist", async () => {
      const backendData = [{ id: 1, name: "Group 1" }];

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.hasEducationalGroups();

      expect(result).toBe(true);
    });

    it("returns false when no groups exist", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "", data: [] }),
      } as Response);

      const result = await userContextService.hasEducationalGroups();

      expect(result).toBe(false);
    });

    it("returns false on error", async () => {
      fetchWithAuthMock.mockRejectedValueOnce(new Error("Network error"));

      const result = await userContextService.hasEducationalGroups();

      expect(result).toBe(false);
    });
  });

  describe("getCurrentUser", () => {
    it("returns mapped user profile on success", async () => {
      const backendData = {
        id: 1,
        email: "test@example.com",
        username: "testuser",
        name: "Test User",
        active: true,
      };

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getCurrentUser();

      expect(result.id).toBe("1");
      expect(result.email).toBe("test@example.com");
    });

    it("throws error on fetch failure", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: () => Promise.resolve("Unauthorized"),
      } as Response);

      await expect(userContextService.getCurrentUser()).rejects.toThrow(
        "Get current user failed: 401",
      );
    });
  });

  describe("getCurrentStaff", () => {
    it("returns mapped staff profile on success", async () => {
      const backendData = {
        id: 10,
        person_id: 100,
      };

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getCurrentStaff();

      expect(result.id).toBe("10");
      expect(result.person_id).toBe("100");
    });

    it("throws error for 404 without console spam", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () => Promise.resolve("Not found"),
      } as Response);

      await expect(userContextService.getCurrentStaff()).rejects.toThrow(
        "Get current staff failed: 404",
      );

      // Should not have logged "Get current staff error:" for 404
      const errorCalls = consoleSpy.mock.calls.filter(
        (call: unknown[]) =>
          String(call[0]).includes("Get current staff error:") &&
          !String(call[0]).includes("404"),
      );
      expect(errorCalls.length).toBe(0);

      consoleSpy.mockRestore();
    });
  });

  describe("getMyActiveGroups", () => {
    it("returns mapped active groups on success", async () => {
      const backendData = [
        {
          id: 3,
          name: "Active Group",
          room_id: 30,
          room: { id: 30, name: "Raum 303" },
        },
      ];

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getMyActiveGroups();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("3");
    });

    it("handles null response data", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "", data: null }),
      } as Response);

      const result = await userContextService.getMyActiveGroups();

      expect(result).toEqual([]);
    });

    it("throws error on fetch failure", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Server error"),
      } as Response);

      await expect(userContextService.getMyActiveGroups()).rejects.toThrow(
        "Get active groups failed: 500",
      );
    });
  });

  describe("getCurrentTeacher", () => {
    it("returns mapped teacher profile on success", async () => {
      const backendData = {
        id: 5,
        staff_id: 10,
        staff: {
          id: 10,
          person_id: 100,
          email: "teacher@school.com",
        },
      };

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getCurrentTeacher();

      expect(result.id).toBe("5");
      expect(result.staff_id).toBe("10");
      expect(result.staff?.id).toBe("10");
    });

    it("throws error on fetch failure", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: () => Promise.resolve("Unauthorized"),
      } as Response);

      await expect(userContextService.getCurrentTeacher()).rejects.toThrow(
        "Get current teacher failed: 401",
      );
    });

    it("handles teacher without staff data", async () => {
      const backendData = {
        id: 5,
        staff_id: 10,
      };

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getCurrentTeacher();

      expect(result.id).toBe("5");
      expect(result.staff).toBeUndefined();
    });
  });

  describe("getMyActivityGroups", () => {
    it("returns mapped activity groups on success", async () => {
      const backendData = [
        {
          id: 10,
          name: "Chess Club",
          room_id: 50,
          room: { id: 50, name: "Activity Room" },
        },
        {
          id: 11,
          name: "Music Group",
        },
      ];

      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({ success: true, message: "", data: backendData }),
      } as Response);

      const result = await userContextService.getMyActivityGroups();

      expect(result).toHaveLength(2);
      expect(result[0]?.id).toBe("10");
      expect(result[0]?.name).toBe("Chess Club");
      expect(result[0]?.room?.name).toBe("Activity Room");
      expect(result[1]?.id).toBe("11");
      expect(result[1]?.room).toBeUndefined();
    });

    it("throws error on fetch failure", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Server error"),
      } as Response);

      await expect(userContextService.getMyActivityGroups()).rejects.toThrow(
        "Get activity groups failed: 500",
      );
    });
  });
});
