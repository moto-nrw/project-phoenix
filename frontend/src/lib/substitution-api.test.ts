// substitution-api.test.ts
// Comprehensive tests for substitution API service

import { describe, it, expect, vi, beforeEach } from "vitest";
import type {
  BackendSubstitution,
  BackendStaffWithSubstitutionStatus,
  BackendPerson,
  BackendStaff,
  BackendSubstitutionInfo,
} from "./substitution-helpers";
import { buildBackendGroup } from "~/test/fixtures";

// Mock dependencies
vi.mock("./session-cache", () => {
  const getCachedSession = vi.fn();
  return {
    getCachedSession,
    clearSessionCache: vi.fn(),
    sessionFetch: vi.fn(async (url: string, init?: RequestInit) => {
      const session = (await getCachedSession()) as {
        user?: { token?: string };
      } | null;
      const token = session?.user?.token;
      if (!token) throw new Error("No authentication token available");
      return fetch(url, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          ...(init?.headers as Record<string, string> | undefined),
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });
    }),
  };
});

// Import after mocks
import { getCachedSession } from "./session-cache";
import { substitutionService } from "./substitution-api";

const mockGetSession = vi.fn();
vi.mocked(getCachedSession).mockImplementation(mockGetSession);

// Sample test data
const sampleBackendPerson: BackendPerson = {
  id: 1,
  first_name: "John",
  last_name: "Doe",
  full_name: "John Doe",
};

const sampleBackendStaff: BackendStaff = {
  id: 10,
  person_id: 1,
  person: sampleBackendPerson,
  staff_notes: "Test notes",
};

const sampleBackendGroup = buildBackendGroup({
  id: 5,
  name: "Group A",
  room_id: 101,
  representative_id: 10,
});

const sampleBackendSubstitution: BackendSubstitution = {
  id: 1,
  group_id: 5,
  group: sampleBackendGroup,
  regular_staff_id: 10,
  regular_staff: sampleBackendStaff,
  substitute_staff_id: 11,
  substitute_staff: {
    id: 11,
    person_id: 2,
    person: {
      id: 2,
      first_name: "Jane",
      last_name: "Smith",
      full_name: "Jane Smith",
    },
    staff_notes: "",
  },
  start_date: "2024-01-15",
  end_date: "2024-01-20",
  reason: "Sick leave",
  notes: "Emergency substitution",
  created_at: "2024-01-10T00:00:00Z",
  updated_at: "2024-01-10T00:00:00Z",
};

const sampleBackendSubstitutionInfo: BackendSubstitutionInfo = {
  id: 1,
  group_id: 5,
  group_name: "Group A",
  is_transfer: false,
  start_date: "2024-01-15",
  end_date: "2024-01-20",
  group: sampleBackendGroup,
};

const sampleBackendStaffWithStatus: BackendStaffWithSubstitutionStatus = {
  id: 10,
  person_id: 1,
  person: sampleBackendPerson,
  staff_notes: "Test notes",
  is_substituting: true,
  substitution_count: 2,
  substitutions: [sampleBackendSubstitutionInfo],
  current_group: sampleBackendGroup,
  regular_group: sampleBackendGroup,
  teacher_id: 100,
  specialization: "Mathematics",
  role: "Teacher",
  qualifications: "PhD in Math",
};

describe("SubstitutionService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  describe("fetchSubstitutions", () => {
    it("fetches substitutions without filters", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendSubstitution] }),
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchSubstitutions();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions",
        expect.objectContaining({
          credentials: "include",
          headers: {
            Authorization: "Bearer test-token",
            "Content-Type": "application/json",
          },
        }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.groupName).toBe("Group A");
    });

    it("fetches substitutions with pagination filters", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendSubstitution] }),
      });
      global.fetch = mockFetch;

      await substitutionService.fetchSubstitutions({
        page: 2,
        pageSize: 20,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions?page=2&page_size=20",
        expect.any(Object),
      );
    });

    it("handles direct array response", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => [sampleBackendSubstitution],
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchSubstitutions();

      expect(result).toHaveLength(1);
    });

    it("returns empty array for unexpected response format", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ unexpected: "format" }),
      });
      global.fetch = mockFetch;

      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      const result = await substitutionService.fetchSubstitutions();

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });

    it("throws error on failed request", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Internal Server Error",
      });
      global.fetch = mockFetch;

      await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
        "Failed to fetch substitutions: Internal Server Error",
      );
    });

    it("throws error when no token is available", async () => {
      mockGetSession.mockResolvedValueOnce(null);

      await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
        "No authentication token available",
      );
    });
  });

  describe("fetchActiveSubstitutions", () => {
    it("fetches active substitutions without date", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendSubstitution] }),
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchActiveSubstitutions();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions/active",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("fetches active substitutions with date", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendSubstitution] }),
      });
      global.fetch = mockFetch;

      const testDate = new Date("2024-01-15");
      await substitutionService.fetchActiveSubstitutions(testDate);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions/active?date=2024-01-15",
        expect.any(Object),
      );
    });

    it("handles direct array response", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => [sampleBackendSubstitution],
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchActiveSubstitutions();

      expect(result).toHaveLength(1);
    });

    it("throws error on failed request", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      });
      global.fetch = mockFetch;

      await expect(
        substitutionService.fetchActiveSubstitutions(),
      ).rejects.toThrow("Failed to fetch active substitutions: Not Found");
    });
  });

  describe("fetchAvailableTeachers", () => {
    it("fetches available teachers without filters", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendStaffWithStatus] }),
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchAvailableTeachers();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/available-for-substitution",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("10");
      expect(result[0]?.firstName).toBe("John");
    });

    it("fetches available teachers with date and search", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [sampleBackendStaffWithStatus] }),
      });
      global.fetch = mockFetch;

      const testDate = new Date("2024-01-15");
      await substitutionService.fetchAvailableTeachers(testDate, "John");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/available-for-substitution?date=2024-01-15&search=John",
        expect.any(Object),
      );
    });

    it("handles direct array response", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => [sampleBackendStaffWithStatus],
      });
      global.fetch = mockFetch;

      const result = await substitutionService.fetchAvailableTeachers();

      expect(result).toHaveLength(1);
    });

    it("returns empty array for unexpected response", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => null,
      });
      global.fetch = mockFetch;

      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      const result = await substitutionService.fetchAvailableTeachers();

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });

    it("throws error on failed request", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Forbidden",
      });
      global.fetch = mockFetch;

      await expect(
        substitutionService.fetchAvailableTeachers(),
      ).rejects.toThrow("Failed to fetch available teachers: Forbidden");
    });
  });

  describe("createSubstitution", () => {
    it("creates substitution with regular staff", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendSubstitution }),
      });
      global.fetch = mockFetch;

      const startDate = new Date("2024-01-15");
      const endDate = new Date("2024-01-20");

      const result = await substitutionService.createSubstitution(
        "5",
        "10",
        "11",
        startDate,
        endDate,
        "Sick leave",
        "Emergency",
      );

      const callArgs = mockFetch.mock.calls[0] as unknown as [
        string,
        RequestInit,
      ];
      expect(callArgs[0]).toBe("/api/substitutions");
      expect(callArgs[1]?.method).toBe("POST");
      expect(callArgs[1]?.body).toContain('"group_id":5');
      expect(result.id).toBe("1");
      expect(result.groupId).toBe("5");
    });

    it("creates substitution without regular staff (general coverage)", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendSubstitution }),
      });
      global.fetch = mockFetch;

      const startDate = new Date("2024-01-15");
      const endDate = new Date("2024-01-20");

      await substitutionService.createSubstitution(
        "5",
        null,
        "11",
        startDate,
        endDate,
      );

      const callArgs = mockFetch.mock.calls[0] as unknown as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1]?.body as string) as {
        regular_staff_id?: number;
      };

      expect(body.regular_staff_id).toBeUndefined();
    });

    it("handles direct response without data wrapper", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => sampleBackendSubstitution,
      });
      global.fetch = mockFetch;

      const result = await substitutionService.createSubstitution(
        "5",
        "10",
        "11",
        new Date("2024-01-15"),
        new Date("2024-01-20"),
      );

      expect(result.id).toBe("1");
    });

    it("throws error with error message from response", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        json: async () => ({ error: "Invalid staff ID" }),
      });
      global.fetch = mockFetch;

      await expect(
        substitutionService.createSubstitution(
          "5",
          "10",
          "11",
          new Date("2024-01-15"),
          new Date("2024-01-20"),
        ),
      ).rejects.toThrow("Failed to create substitution: Invalid staff ID");
    });

    it("uses message field if error field not present", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        json: async () => ({ message: "Validation failed" }),
      });
      global.fetch = mockFetch;

      await expect(
        substitutionService.createSubstitution(
          "5",
          "10",
          "11",
          new Date("2024-01-15"),
          new Date("2024-01-20"),
        ),
      ).rejects.toThrow("Failed to create substitution: Validation failed");
    });

    it("includes authorization header when token available", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendSubstitution }),
      });
      global.fetch = mockFetch;

      await substitutionService.createSubstitution(
        "5",
        "10",
        "11",
        new Date("2024-01-15"),
        new Date("2024-01-20"),
      );

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions",
        expect.objectContaining({
          headers: {
            Authorization: "Bearer test-token",
            "Content-Type": "application/json",
          },
        }),
      );
    });

    it("throws error when no token is available", async () => {
      mockGetSession.mockResolvedValueOnce(null);

      await expect(
        substitutionService.createSubstitution(
          "5",
          "10",
          "11",
          new Date("2024-01-15"),
          new Date("2024-01-20"),
        ),
      ).rejects.toThrow("No authentication token available");
    });
  });

  describe("deleteSubstitution", () => {
    it("deletes substitution successfully", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
      });
      global.fetch = mockFetch;

      await substitutionService.deleteSubstitution("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions/1",
        expect.objectContaining({
          method: "DELETE",
          credentials: "include",
          headers: {
            Authorization: "Bearer test-token",
            "Content-Type": "application/json",
          },
        }),
      );
    });

    it("throws error on failed deletion", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      });
      global.fetch = mockFetch;

      await expect(
        substitutionService.deleteSubstitution("999"),
      ).rejects.toThrow("Failed to delete substitution: Not Found");
    });
  });

  describe("getSubstitution", () => {
    it("fetches single substitution by ID", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendSubstitution }),
      });
      global.fetch = mockFetch;

      const result = await substitutionService.getSubstitution("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions/1",
        expect.any(Object),
      );
      expect(result.id).toBe("1");
      expect(result.groupId).toBe("5");
    });

    it("handles direct response without data wrapper", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => sampleBackendSubstitution,
      });
      global.fetch = mockFetch;

      const result = await substitutionService.getSubstitution("1");

      expect(result.id).toBe("1");
    });

    it("throws error when not found", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      });
      global.fetch = mockFetch;

      await expect(substitutionService.getSubstitution("999")).rejects.toThrow(
        "Failed to fetch substitution: Not Found",
      );
    });
  });

  describe("error handling and edge cases", () => {
    it("logs errors when fetch throws", async () => {
      mockGetSession.mockResolvedValueOnce({
        user: { token: "test-token" },
        expires: "2099-01-01",
      });

      const mockFetch = vi
        .fn()
        .mockRejectedValueOnce(new Error("Network error"));
      global.fetch = mockFetch;

      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
        "Network error",
      );

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "error fetching substitutions",
        {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          error: expect.any(String),
        },
      );

      consoleErrorSpy.mockRestore();
    });

    it("throws error when getSession returns null", async () => {
      mockGetSession.mockResolvedValueOnce(null);

      await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when session has no user object", async () => {
      mockGetSession.mockResolvedValueOnce({
        expires: "2099-01-01",
      } as never);

      await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
        "No authentication token available",
      );
    });
  });
});
