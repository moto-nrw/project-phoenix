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
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() =>
    Promise.resolve({
      user: { id: "1" },
    }),
  ),
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

    // The BFF reads the session cookie; the browser holds no token (#5005).
    it("sends no Authorization header", async () => {
      fetchWithAuthMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "", data: [] }),
      } as Response);

      await userContextService.getMyEducationalGroups();

      expect(fetchWithAuthMock).toHaveBeenCalledWith("/api/me/groups", {
        headers: { "Content-Type": "application/json" },
      });
    });
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
});
