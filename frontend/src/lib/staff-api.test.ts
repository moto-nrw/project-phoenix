/* eslint-disable @typescript-eslint/no-misused-promises */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { BackendStaffResponse, StaffFilters } from "./staff-api";
import { suppressConsole } from "~/test/helpers/console";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock session-cache before importing the module
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

// Import after mocks are set up
import { getCachedSession } from "./session-cache";
import { staffService } from "./staff-api";

// Type for mocked functions
const mockedGetSession = vi.mocked(getCachedSession);

// Sample backend staff data
const sampleBackendStaff: BackendStaffResponse = {
  id: "1",
  name: "Max Mustermann",
  firstName: "Max",
  lastName: "Mustermann",
  specialization: "Mathematics",
  role: "Teacher",
  qualifications: "M.Ed",
  tag_id: "TAG001",
  staff_notes: "Senior staff member",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T00:00:00Z",
  staff_id: "1",
  teacher_id: "10",
};

const sampleBackendStaffWithoutTeacher: BackendStaffResponse = {
  id: "2",
  name: "Anna Schmidt",
  firstName: "Anna",
  lastName: "Schmidt",
  specialization: "Administration",
  role: "Staff",
  qualifications: null,
  tag_id: null,
  staff_notes: null,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T00:00:00Z",
  staff_id: "2",
};

// Sample active groups for supervision testing
const sampleActiveGroups = [
  {
    id: 1,
    name: "Morning Session",
    room: { id: 5, name: "Room A" },
    supervisors: [{ staff_id: 1, role: "supervisor" }],
  },
  {
    id: 2,
    name: "Afternoon Session",
    room: { id: 6, name: "Room B" },
    supervisors: [
      { staff_id: 1, role: "supervisor" },
      { staff_id: 3, role: "assistant" },
    ],
  },
];

describe("staff-api", () => {
  const consoleSpies = suppressConsole("error", "warn");
  let originalFetch: typeof fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();

    // Default session mock
    mockedGetSession.mockResolvedValue(mockSessionData());
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  describe("staffService.getAllStaff", () => {
    it("fetches staff and returns mapped data", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock staff endpoint
      mockFetch.mockImplementation((url: string) => {
        if (url === "/api/staff") {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: [] }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.name).toBe("Max Mustermann");
      expect(result[0]?.firstName).toBe("Max");
      expect(result[0]?.lastName).toBe("Mustermann");
      expect(result[0]?.specialization).toBe("Mathematics");
      expect(result[0]?.hasRfid).toBe(true);
      expect(result[0]?.isTeacher).toBe(true);
      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });

    it("throws error when no auth token available", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(staffService.getAllStaff()).rejects.toThrow(
        "No authentication token available",
      );
    });

    it("throws error when staff fetch fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url === "/api/staff") {
          return Promise.resolve({
            ok: false,
            statusText: "Internal Server Error",
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      await expect(staffService.getAllStaff()).rejects.toThrow(
        "Failed to fetch staff: Internal Server Error",
      );
    });

    it("applies search filter to URL", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const filters: StaffFilters = { search: "Max" };
      await staffService.getAllStaff(filters);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff?search=Max",
        expect.any(Object),
      );
    });

    it("filters staff by supervising status", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve([
                sampleBackendStaff,
                sampleBackendStaffWithoutTeacher,
              ]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: sampleActiveGroups }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      // Test supervising filter
      const supervisingResult = await staffService.getAllStaff({
        status: "supervising",
      });
      expect(supervisingResult.every((s) => s.isSupervising)).toBe(true);

      // Test available filter
      const availableResult = await staffService.getAllStaff({
        status: "available",
      });
      expect(availableResult.every((s) => !s.isSupervising)).toBe(true);
    });

    it("filters staff by type (teachers vs staff)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve([
                sampleBackendStaff,
                sampleBackendStaffWithoutTeacher,
              ]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      // Test teachers filter
      const teachersResult = await staffService.getAllStaff({
        type: "teachers",
      });
      expect(teachersResult.every((s) => s.isTeacher)).toBe(true);

      // Test staff filter
      const staffOnlyResult = await staffService.getAllStaff({ type: "staff" });
      expect(staffOnlyResult.every((s) => !s.isTeacher)).toBe(true);
    });

    it("handles wrapped response format", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                data: [sampleBackendStaff],
              }),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
    });

    it("correctly identifies supervising staff with room location", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: sampleActiveGroups }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Staff ID 1 supervises 2 rooms
      expect(result[0]?.isSupervising).toBe(true);
      // currentLocation shows time clock status (Abwesend since no work_status)
      expect(result[0]?.currentLocation).toBe("Abwesend");
      expect(result[0]?.supervisionRole).toBe("supervisor");
      // Room info is in supervisions array
      expect(result[0]?.supervisions).toHaveLength(2);
    });

    it("handles active groups fetch failure gracefully", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: false,
            statusText: "Not Found",
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      // Should not throw, just return staff without supervision info
      const result = await staffService.getAllStaff();

      expect(result).toHaveLength(1);
      expect(result[0]?.isSupervising).toBe(false);
    });

    it("handles empty staff list", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result).toEqual([]);
    });

    it("trims specialization whitespace", async () => {
      const staffWithWhitespace: BackendStaffResponse = {
        ...sampleBackendStaff,
        specialization: "  Mathematics  ",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffWithWhitespace]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.specialization).toBe("Mathematics");
    });

    it("handles staff without staff_id", async () => {
      const staffWithoutStaffId: BackendStaffResponse = {
        ...sampleBackendStaff,
        staff_id: undefined,
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffWithoutStaffId]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: sampleActiveGroups }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });

    it("returns Anwesend location for staff with was_present_today true and not supervising", async () => {
      const staffPresentToday: BackendStaffResponse = {
        ...sampleBackendStaff,
        was_present_today: true,
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffPresentToday]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]), // No active supervisions
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Anwesend");
      expect(result[0]?.wasPresentToday).toBe(true);
    });

    it("returns Abwesend location for staff with was_present_today false", async () => {
      const staffNotPresentToday: BackendStaffResponse = {
        ...sampleBackendStaff,
        was_present_today: false,
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffNotPresentToday]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Abwesend");
      expect(result[0]?.wasPresentToday).toBe(false);
    });

    it("returns Anwesend location when supervising with was_present_today true (legacy)", async () => {
      const staffSupervisingAndPresent: BackendStaffResponse = {
        ...sampleBackendStaff,
        was_present_today: true,
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffSupervisingAndPresent]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: sampleActiveGroups }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Staff is supervising, currentLocation shows time clock status (legacy fallback)
      expect(result[0]?.isSupervising).toBe(true);
      expect(result[0]?.currentLocation).toBe("Anwesend");
      // Room info is in supervisions array
      expect(result[0]?.supervisions).toHaveLength(2);
    });

    it("returns Anwesend for staff without staff_id but with was_present_today true", async () => {
      const staffNoIdButPresent: BackendStaffResponse = {
        ...sampleBackendStaff,
        staff_id: undefined,
        was_present_today: true,
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffNoIdButPresent]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Anwesend");
    });
  });

  describe("staffService.getStaffSupervisions", () => {
    it("fetches supervisions for a staff member", async () => {
      const mockSupervisions = [
        {
          id: 1,
          staff_id: 1,
          group_id: 10,
          role: "supervisor",
          start_date: "2024-01-15T08:00:00Z",
          is_active: true,
          active_group: {
            id: 10,
            name: "Morning Session",
            room_id: 5,
            room: { id: 5, name: "Room A" },
          },
        },
      ];

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSupervisions),
      } as Response);

      const result = await staffService.getStaffSupervisions("1");

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe(1);
      expect(result[0]?.role).toBe("supervisor");
    });

    it("handles wrapped response format", async () => {
      const mockSupervisions = [
        {
          id: 1,
          staff_id: 1,
          group_id: 10,
          role: "supervisor",
          start_date: "2024-01-15T08:00:00Z",
          is_active: true,
        },
      ];

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockSupervisions }),
      } as Response);

      const result = await staffService.getStaffSupervisions("1");

      expect(result).toHaveLength(1);
    });

    it("returns empty array on error", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const result = await staffService.getStaffSupervisions("1");

      expect(result).toEqual([]);
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error fetching supervisions for staff",
        expect.objectContaining({
          staff_id: "1",
          error: expect.stringContaining("Network error") as unknown,
        }),
      );
    });

    it("returns empty array and logs error when no auth token available", async () => {
      mockedGetSession.mockResolvedValue(null);

      // Note: getStaffSupervisions catches the error and returns empty array
      const result = await staffService.getStaffSupervisions("1");

      expect(result).toEqual([]);
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error fetching supervisions for staff",
        expect.objectContaining({
          staff_id: "1",
          error: expect.stringContaining("No authentication token") as unknown,
        }),
      );
    });

    it("returns empty array and logs error when response is not ok", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      // Note: getStaffSupervisions catches the error and returns empty array
      const result = await staffService.getStaffSupervisions("1");

      expect(result).toEqual([]);
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error fetching supervisions for staff",
        expect.objectContaining({
          staff_id: "1",
          error: expect.stringContaining(
            "Failed to fetch staff supervisions",
          ) as unknown,
        }),
      );
    });

    it("returns empty array for unexpected response format", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ unexpected: "format" }),
      } as Response);

      const result = await staffService.getStaffSupervisions("1");

      expect(result).toEqual([]);
    });
  });

  describe("extractActiveGroups edge cases", () => {
    it("handles double-wrapped data (frontend wrapper around backend response)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          // Double wrapped: { data: { data: [...] } }
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                data: { data: sampleActiveGroups },
              }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Should handle double-wrapped and still find supervisions
      expect(result[0]?.isSupervising).toBe(true);
      // currentLocation shows time clock status
      expect(result[0]?.currentLocation).toBe("Abwesend");
      // Room info is in supervisions array
      expect(result[0]?.supervisions).toHaveLength(2);
    });

    it("handles non-array data gracefully", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          // Return object without data array
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: "not-an-array" }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Should fallback to empty array and not throw
      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });

    it("handles null data property", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: null }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
    });
  });

  describe("getSupervisionInfo priority hierarchy", () => {
    it("returns absence status when absence_type is provided and not clocked in", async () => {
      const staffWithAbsence: BackendStaffResponse = {
        ...sampleBackendStaff,
        absence_type: "sick",
        // Note: NO work_status - absence shown when not clocked in
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffWithAbsence]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: [] }), // No supervisions
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Absence shown when not clocked in
      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Krank");
      expect(result[0]?.absenceType).toBe("sick");
    });

    it("returns Urlaub for vacation absence", async () => {
      const staffOnVacation: BackendStaffResponse = {
        ...sampleBackendStaff,
        absence_type: "vacation",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffOnVacation]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.currentLocation).toBe("Urlaub");
    });

    it("returns Fortbildung for training absence", async () => {
      const staffInTraining: BackendStaffResponse = {
        ...sampleBackendStaff,
        absence_type: "training",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffInTraining]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.currentLocation).toBe("Fortbildung");
    });

    it("returns Abwesend for other absence type", async () => {
      const staffAbsent: BackendStaffResponse = {
        ...sampleBackendStaff,
        absence_type: "other",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffAbsent]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.currentLocation).toBe("Abwesend");
    });

    it("returns Abwesend for checked_out status", async () => {
      const staffCheckedOut: BackendStaffResponse = {
        ...sampleBackendStaff,
        work_status: "checked_out",
        was_present_today: true, // Should be overridden by checked_out
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffCheckedOut]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: sampleActiveGroups }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // Checked out shows Abwesend, supervisions still tracked
      expect(result[0]?.isSupervising).toBe(true);
      expect(result[0]?.currentLocation).toBe("Abwesend");
      expect(result[0]?.workStatus).toBe("checked_out");
    });

    it("returns Anwesend for present work status without supervision (priority 4)", async () => {
      const staffPresent: BackendStaffResponse = {
        ...sampleBackendStaff,
        work_status: "present",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffPresent]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]), // No supervision
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Anwesend");
      expect(result[0]?.workStatus).toBe("present");
    });

    it("returns Homeoffice for home_office work status (priority 4)", async () => {
      const staffHomeOffice: BackendStaffResponse = {
        ...sampleBackendStaff,
        work_status: "home_office",
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffHomeOffice]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([]),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Homeoffice");
      expect(result[0]?.workStatus).toBe("home_office");
    });

    it("returns Abwesend for staff supervising one room (room info in supervisions)", async () => {
      const singleRoomSupervision = [
        {
          id: 1,
          name: "Morning Session",
          room: { id: 5, name: "Room A" },
          supervisors: [{ staff_id: 1, role: "supervisor" }],
        },
      ];

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: singleRoomSupervision }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // currentLocation shows time clock status
      expect(result[0]?.isSupervising).toBe(true);
      expect(result[0]?.currentLocation).toBe("Abwesend");
      // Room info is in supervisions array
      expect(result[0]?.supervisions).toHaveLength(1);
      expect(result[0]?.supervisions?.[0]?.roomName).toBe("Room A");
    });

    it("returns Abwesend for staff supervising groups without rooms", async () => {
      const noRoomSupervision = [
        {
          id: 1,
          name: "Mobile Session",
          room: null, // No room assigned
          supervisors: [{ staff_id: 1, role: "supervisor" }],
        },
      ];

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: noRoomSupervision }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // currentLocation shows time clock status, not room
      expect(result[0]?.isSupervising).toBe(true);
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });

    it("handles undefined room name", async () => {
      const undefinedRoomName = [
        {
          id: 1,
          name: "Session",
          room: { id: 5, name: undefined }, // Room exists but no name
          supervisors: [{ staff_id: 1, role: "supervisor" }],
        },
      ];

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: undefinedRoomName }),
          } as Response);
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      const result = await staffService.getAllStaff();

      // currentLocation shows time clock status
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });
  });

  describe("fetchActiveGroups error handling", () => {
    it("handles fetch exception and returns empty array", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([sampleBackendStaff]),
          } as Response);
        }
        if (url.includes("/api/active/groups")) {
          // Throw network error
          throw new Error("Network failure");
        }
        return Promise.reject(new Error(`Unexpected URL: ${url}`));
      });

      // Should not throw, just return staff without supervision info
      const result = await staffService.getAllStaff();

      expect(result).toHaveLength(1);
      expect(result[0]?.isSupervising).toBe(false);
      expect(result[0]?.currentLocation).toBe("Abwesend");
    });
  });
});
