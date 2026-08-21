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
import {
  staffAbsenceService,
  staffBalanceAdjustmentService,
  staffHistoryService,
  staffMonthSummaryService,
  staffScheduleService,
  staffService,
  staffSessionEditsService,
  staffSessionService,
  staffStammdatenService,
  workTimeModelService,
} from "./staff-api";

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

  describe("staffService.getDocumentDirectory", () => {
    it("unwraps the API response envelope", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const entries = [
        {
          id: "42",
          name: "Mila Muster",
          firstName: "Mila",
          lastName: "Muster",
        },
      ];
      mockFetch.mockResolvedValue({
        ok: true,
        statusText: "OK",
        json: () =>
          Promise.resolve({ success: true, message: "", data: entries }),
      } as Response);

      await expect(staffService.getDocumentDirectory()).resolves.toEqual(
        entries,
      );
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/documents-directory",
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
    });

    it("throws with the status text when the directory request fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffService.getDocumentDirectory()).rejects.toThrow(
        "Failed to fetch document directory: Forbidden",
      );
    });
  });

  describe("staffService.getDocumentProfile", () => {
    it("maps the reduced document profile", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        statusText: "OK",
        json: () =>
          Promise.resolve({
            data: {
              id: 42,
              name: "Mila Muster",
              firstName: "Mila",
              lastName: "Muster",
            },
          }),
      } as Response);

      const profile = await staffService.getDocumentProfile("42");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/documents-profile/42",
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
      expect(profile).toMatchObject({
        id: "42",
        name: "Mila Muster",
        isLimitedProfile: true,
        isFinancialProfile: false,
        isSupervising: false,
        hasRfid: false,
        isTeacher: false,
        supervisions: [],
      });
    });

    it("throws with the profile type in the message when the request fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffService.getDocumentProfile("42")).rejects.toThrow(
        "Failed to fetch document staff profile: Forbidden",
      );
    });
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

    it("appends the strict flag to the base URL (#1840)", async () => {
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

      await staffService.getAllStaff(undefined, { strict: true });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff?strict=1",
        expect.any(Object),
      );
    });

    it("appends the strict flag after an existing search query (#1840)", async () => {
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

      await staffService.getAllStaff({ search: "Max" }, { strict: true });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff?search=Max&strict=1",
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

    it("returns Freizeitausgleich for comp-time absence", async () => {
      const staffOnCompTime: BackendStaffResponse = {
        ...sampleBackendStaff,
        absence_type: "comp_time",
      };
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockImplementation((url: string) => {
        if (url.includes("/api/staff")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve([staffOnCompTime]),
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

      expect(result[0]?.currentLocation).toBe("Freizeitausgleich");
      expect(result[0]?.absenceType).toBe("comp_time");
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

  describe("staffService.getStaffById", () => {
    it("fetches and maps a single staff member", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              ...sampleBackendStaff,
              employment_type: "mini_job",
              work_status: "present",
            },
          }),
      } as Response);

      const result = await staffService.getStaffById("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1",
        expect.any(Object),
      );
      expect(result.id).toBe("1");
      expect(result.currentLocation).toBe("Anwesend");
      expect(result.employmentType).toBe("mini_job");
    });

    it("throws when the staff member request fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      await expect(staffService.getStaffById("404")).rejects.toThrow(
        "Failed to fetch staff member: Not Found",
      );
    });
  });

  describe("staffScheduleService", () => {
    const backendSchedule = {
      mode: "template" as const,
      model: {
        id: 5,
        name: "Vollzeit",
        rotation_length: 2,
        rotation_anchor_date: "2026-06-01",
      },
      rotation_length: 2,
      rotation_anchor_date: "2026-06-01",
      entries: [{ week_index: 0, day_of_week: 0, target_minutes: 480 }],
      weekly_totals: [2400, 1800],
      valid_from: "2026-06-01T00:00:00Z",
    };

    it("loads and maps a staff schedule", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendSchedule }),
      } as Response);

      const result = await staffScheduleService.getSchedule("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/schedule",
        expect.any(Object),
      );
      expect(result.model?.id).toBe("5");
      expect(result.entries[0]?.targetMinutes).toBe(480);
      expect(result.validFrom).toBe("2026-06-01");
    });

    it("maps optional staff schedule start times", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              ...backendSchedule,
              entries: [
                {
                  week_index: 0,
                  day_of_week: 0,
                  target_minutes: 480,
                  start_time: "09:00",
                },
              ],
            },
          }),
      } as Response);

      const result = await staffScheduleService.getSchedule("1");

      expect(result.entries[0]?.startTime).toBe("09:00");
    });

    it("sends template updates with numeric model id", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendSchedule }),
      } as Response);

      await staffScheduleService.updateSchedule("1", {
        mode: "template",
        modelId: "5",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/schedule",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ mode: "template", model_id: 5 }),
        }),
      );
    });

    it("sends custom updates with mapped entries", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { ...backendSchedule, mode: "custom", model: null },
          }),
      } as Response);

      const result = await staffScheduleService.updateSchedule("1", {
        mode: "custom",
        rotationLength: 1,
        rotationAnchorDate: "2026-06-01",
        entries: [{ weekIndex: 0, dayOfWeek: 1, targetMinutes: 267 }],
        saveAsTemplate: "Teilzeit",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/schedule",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({
            mode: "custom",
            rotation_length: 1,
            rotation_anchor_date: "2026-06-01",
            entries: [{ week_index: 0, day_of_week: 1, target_minutes: 267 }],
            save_as_template: "Teilzeit",
          }),
        }),
      );
      expect(result.model).toBeNull();
    });

    it("sends optional custom schedule start times", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { ...backendSchedule, mode: "custom", model: null },
          }),
      } as Response);

      await staffScheduleService.updateSchedule("1", {
        mode: "custom",
        rotationLength: 1,
        entries: [
          {
            weekIndex: 0,
            dayOfWeek: 1,
            targetMinutes: 360,
            startTime: "09:00",
          },
        ],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/schedule",
        expect.objectContaining({
          body: JSON.stringify({
            mode: "custom",
            rotation_length: 1,
            rotation_anchor_date: undefined,
            entries: [
              {
                week_index: 0,
                day_of_week: 1,
                target_minutes: 360,
                start_time: "09:00",
              },
            ],
            save_as_template: undefined,
          }),
        }),
      );
    });
  });

  describe("workTimeModelService", () => {
    it("lists and maps work time models", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              {
                id: 9,
                name: "Halbtag",
                rotation_length: 1,
                rotation_anchor_date: "2026-06-01",
                entries: [
                  { week_index: 0, day_of_week: 0, target_minutes: 240 },
                ],
                weekly_totals: [1200],
              },
            ],
          }),
      } as Response);

      const result = await workTimeModelService.list();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/work-time-models",
        expect.any(Object),
      );
      expect(result[0]?.id).toBe("9");
      expect(result[0]?.entries[0]?.targetMinutes).toBe(240);
    });

    it("maps optional work time model start times", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              {
                id: 9,
                name: "Halbtag",
                rotation_length: 1,
                rotation_anchor_date: "2026-06-01",
                entries: [
                  {
                    week_index: 0,
                    day_of_week: 0,
                    target_minutes: 240,
                    start_time: "09:00",
                  },
                ],
                weekly_totals: [1200],
              },
            ],
          }),
      } as Response);

      const result = await workTimeModelService.list();

      expect(result[0]?.entries[0]?.startTime).toBe("09:00");
    });
  });

  describe("staffMonthSummaryService.getScheduleTargetsRange", () => {
    // The Übersicht charts price history against these targets and reach back
    // to the account start ("Gesamt"), which can be years — while the endpoint
    // rejects anything over MAX_TARGET_RANGE_DAYS. One request would 400 and
    // the charts would show no Soll at all (#1842).
    it("splits a range longer than the endpoint window and merges the answers", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockImplementation((url: string) =>
        Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              data: [
                {
                  date: new URL(url, "http://x").searchParams.get("from"),
                  target_minutes: 480,
                },
              ],
            }),
        } as unknown as Response),
      );

      const result = await staffMonthSummaryService.getScheduleTargetsRange(
        "1",
        "2024-01-01",
        "2026-06-01",
      );

      const requested = mockFetch.mock.calls.map((c) => {
        const params = new URL(c[0] as string, "http://x").searchParams;
        return `${params.get("from")}..${params.get("to")}`;
      });
      // 2.5 years -> three 366-day windows, contiguous and gapless, the last
      // one clipped at `to`. 2024 is a leap year, so window one ends 12-31.
      expect(requested).toEqual([
        "2024-01-01..2024-12-31",
        "2025-01-01..2026-01-01",
        "2026-01-02..2026-06-01",
      ]);
      // Every window's answer lands in one map — the stub echoes each
      // window's first day back.
      expect(result.get("2024-01-01")).toBe(480);
      expect(result.get("2025-01-01")).toBe(480);
      expect(result.get("2026-01-02")).toBe(480);
      expect(result.size).toBe(3);
    });

    // A "Gesamt" range on a years-old account fans out into one window per
    // ~year. A plain Promise.all would fire every window at once — dozens of
    // simultaneous DB-backed requests from a single overview render. The pool
    // caps how many are in flight (#1842).
    it("bounds how many window requests run at once", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      let inFlight = 0;
      let peak = 0;
      mockFetch.mockImplementation((url: string) => {
        inFlight += 1;
        peak = Math.max(peak, inFlight);
        return new Promise((resolve) => {
          setTimeout(() => {
            inFlight -= 1;
            resolve({
              ok: true,
              json: () =>
                Promise.resolve({
                  data: [
                    {
                      date: new URL(url, "http://x").searchParams.get("from"),
                      target_minutes: 480,
                    },
                  ],
                }),
            } as unknown as Response);
          }, 0);
        });
      });

      // ~15 years -> 15 windows, far more than the pool of 4.
      const result = await staffMonthSummaryService.getScheduleTargetsRange(
        "1",
        "2011-01-01",
        "2026-01-01",
      );

      expect(mockFetch.mock.calls.length).toBeGreaterThan(4);
      expect(peak).toBeLessThanOrEqual(4);
      // Every window still resolves and merges — nothing is dropped by the pool.
      expect(result.get("2011-01-01")).toBe(480);
    });

    it("asks once when the range fits the window", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [{ date: "2026-06-01", target_minutes: 240 }],
          }),
      } as Response);

      const result = await staffMonthSummaryService.getScheduleTargetsRange(
        "1",
        "2026-06-01",
        "2026-06-30",
      );

      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(result.get("2026-06-01")).toBe(240);
    });
  });

  describe("staffHistoryService", () => {
    it("loads staff history for a date range", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              sessions: [
                {
                  date: "2026-06-01",
                  status: "present",
                  net_minutes: 480,
                  check_in_time: "08:00",
                  check_out_time: "16:00",
                  break_minutes: 0,
                },
              ],
            },
          }),
      } as Response);

      const result = await staffHistoryService.getHistory(
        "1",
        "2026-06-01",
        "2026-06-07",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/time-tracking/history?from=2026-06-01&to=2026-06-07",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("throws when staff history fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Gateway",
      } as Response);

      await expect(
        staffHistoryService.getHistory("1", "2026-06-01", "2026-06-07"),
      ).rejects.toThrow("Failed to fetch staff history: Bad Gateway");
    });
  });

  describe("staffAbsenceService", () => {
    const quota = {
      staff_id: 1,
      year: 2026,
      entitled_days: 30,
      carryover_days: 1,
      taken_days: 10,
      reserved_days: 2,
      remaining_days: 19,
    };

    it("lädt Anfragen mit Ansicht, Suche und Art-Filter", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              {
                id: "9007199254740993",
                staff_id: "9007199254740993",
                approved_by: "9007199254740993",
                staff_name: "Mira Muster",
                status: "requested",
              },
            ],
          }),
      } as Response);

      const result = await staffAbsenceService.listRequests("history", {
        search: "  Mira  ",
        types: ["vacation", "sick"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/absences/requests?view=history&search=Mira&types=vacation%2Csick",
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.staff_name).toBe("Mira Muster");
      expect(result[0]).toMatchObject({
        id: "9007199254740993",
        staff_id: "9007199254740993",
        approved_by: "9007199254740993",
      });
    });

    it("lässt leere Suche und leeren Art-Filter aus der Abfrage weg", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: null }),
      } as Response);

      const result = await staffAbsenceService.listRequests("open", {
        search: "   ",
        types: [],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/absences/requests?view=open",
        expect.any(Object),
      );
      expect(result).toEqual([]);
    });

    it("wirft, wenn die Anfragenliste fehlschlägt", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffAbsenceService.listRequests("open")).rejects.toThrow(
        "Failed to fetch absence requests: Forbidden",
      );
    });

    it("loads absences and returns an empty array for null data", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: null }),
      } as Response);

      const result = await staffAbsenceService.getAbsences(
        "1",
        "2026-06-01",
        "2026-06-07",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/absences?from=2026-06-01&to=2026-06-07",
        expect.any(Object),
      );
      expect(result).toEqual([]);
    });

    it("throws when loading absences fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(
        staffAbsenceService.getAbsences("1", "2026-06-01", "2026-06-07"),
      ).rejects.toThrow("Failed to fetch staff absences: Forbidden");
    });

    it("loads vacation quota with and without year", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: quota }),
        } as Response)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: quota }),
        } as Response);

      await staffAbsenceService.getVacationQuota("1", 2026);
      const result = await staffAbsenceService.getVacationQuota("1");

      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/1/vacation/quota?year=2026",
        expect.any(Object),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "/api/staff/1/vacation/quota",
        expect.any(Object),
      );
      expect(result.remaining_days).toBe(19);
    });

    it("throws when vacation quota fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      await expect(staffAbsenceService.getVacationQuota("1")).rejects.toThrow(
        "Failed to fetch quota: Not Found",
      );
    });

    it("saves vacation quota", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: quota }),
      } as Response);

      const payload = {
        year: 2026,
        entitled_days: 30,
        carryover_days: 1,
      };
      const result = await staffAbsenceService.setVacationQuota("1", payload);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/vacation/quota",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify(payload),
        }),
      );
      expect(result.entitled_days).toBe(30);
    });

    it("maps vacation opening identifiers to strings", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: 7,
              staff_id: 4,
              year: 2026,
              effective_date: "2026-02-28",
              taken_before_days: 5.5,
              entered_remaining_days: 26.5,
              note: "Übernahme aus Urlaubsliste",
              decided_by: 9,
              decided_at: "2026-03-01T08:00:00Z",
            },
          }),
      } as Response);

      await expect(
        staffAbsenceService.setVacationOpening("4", {
          effectiveDate: "2026-02-28",
          remainingDays: 26.5,
          note: "Übernahme aus Urlaubsliste",
        }),
      ).resolves.toMatchObject({
        id: "7",
        staff_id: "4",
        decided_by: "9",
      });
    });

    it.each([
      [
        "vacation_opening_already_exists",
        "Für dieses Jahr existiert bereits eine Urlaubs-Übernahme. Lösche zuerst die bestehende Übernahme.",
      ],
      [
        "vacation_opening_absences_before_cutoff",
        "Es existieren bereits Urlaubs-Abwesenheiten vor dem Stichtag. Die Übernahme würde diese Tage doppelt zählen.",
      ],
    ])("explains the takeover rejection %s", async (code, message) => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(JSON.stringify({ error: "rejected", code })),
      } as Response);

      await expect(
        staffAbsenceService.setVacationOpening("4", {
          effectiveDate: "2026-02-28",
          remainingDays: 26.5,
          note: "Übernahme aus Urlaubsliste",
        }),
      ).rejects.toThrow(message);
    });

    it("reports a failed takeover deletion", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () =>
          Promise.resolve(
            JSON.stringify({ error: "Keine Übernahme für dieses Jahr" }),
          ),
      } as Response);

      await expect(
        staffAbsenceService.deleteVacationOpening("4", 2026),
      ).rejects.toThrow("Keine Übernahme für dieses Jahr");
    });

    it("deletes a takeover for the given year", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({ ok: true } as Response);

      await staffAbsenceService.deleteVacationOpening("4", 2026);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/4/vacation/opening?year=2026",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("throws when saving vacation quota fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Conflict",
      } as Response);

      await expect(
        staffAbsenceService.setVacationQuota("1", {
          year: 2026,
          entitled_days: 30,
          carryover_days: 1,
        }),
      ).rejects.toThrow("Failed to save quota: Conflict");
    });

    it("approves and denies absences with decision notes", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({ ok: true } as Response)
        .mockResolvedValueOnce({ ok: true } as Response);

      await staffAbsenceService.approve(7, "Passt");
      await staffAbsenceService.deny(8, "Begründung");

      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/absences/7/approve",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ decision_note: "Passt" }),
        }),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "/api/staff/absences/8/deny",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ decision_note: "Begründung" }),
        }),
      );
    });

    it("uses response text for approve and deny errors", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.resolve("Quota exceeded"),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.resolve("Missing reason"),
        } as Response);

      await expect(staffAbsenceService.approve(7)).rejects.toThrow(
        "Quota exceeded",
      );
      await expect(staffAbsenceService.deny(8, "")).rejects.toThrow(
        "Missing reason",
      );
    });

    it("loads pending absences and normalizes null data", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const pending = [
        {
          id: 7,
          staff_id: 1,
          absence_type: "vacation",
          date_start: "2026-07-14",
          date_end: "2026-07-16",
          half_day: false,
          status: "requested",
        },
      ];
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: pending }),
        } as Response)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: null }),
        } as Response);

      await expect(staffAbsenceService.listPending()).resolves.toEqual(pending);
      await expect(staffAbsenceService.listPending()).resolves.toEqual([]);
      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/absences/pending",
        expect.any(Object),
      );
    });

    it("throws when pending absences cannot be loaded", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffAbsenceService.listPending()).rejects.toThrow(
        "Failed to fetch pending absences: Forbidden",
      );
    });

    it("submits a question with its mandatory decision note", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({ ok: true } as Response);

      await staffAbsenceService.question(7, "Wer übernimmt die Frühschicht?");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/absences/7/question",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            decision_note: "Wer übernimmt die Frühschicht?",
          }),
        }),
      );
    });

    it("uses response text and the fallback for question errors", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.resolve("Rückfrage nicht möglich"),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.reject(new Error("body read failed")),
        } as Response);

      await expect(
        staffAbsenceService.question(7, "Erste Frage"),
      ).rejects.toThrow("Rückfrage nicht möglich");
      await expect(
        staffAbsenceService.question(8, "Zweite Frage"),
      ).rejects.toThrow("Rückfrage fehlgeschlagen");
    });

    it("creates a sick absence and returns the created row (#1843)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const createdRow = {
        id: 42,
        staff_id: 1,
        absence_type: "sick",
        date_start: "2026-07-14",
        date_end: "2026-07-16",
        half_day: false,
        note: "Grippe",
        status: "reported",
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: createdRow }),
      } as Response);

      const body = {
        absence_type: "sick",
        date_start: "2026-07-14",
        date_end: "2026-07-16",
        note: "Grippe",
      };
      const result = await staffAbsenceService.createAbsence("1", body);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/absences",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(body),
        }),
      );
      expect(result.id).toBe(42);
      expect(result.status).toBe("reported");
    });

    it("uses response text when creating an absence fails (#1843)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve("overlapping absence"),
      } as Response);

      await expect(
        staffAbsenceService.createAbsence("1", {
          absence_type: "sick",
          date_start: "2026-07-14",
          date_end: "2026-07-14",
        }),
      ).rejects.toThrow("overlapping absence");
    });

    it("maps the comp_time overdraft conflict to a German message (#1420)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              code: "comp_time_exceeds_balance",
              error: "comp_time absence exceeds accrued balance",
            }),
          ),
      } as Response);

      await expect(
        staffAbsenceService.createAbsence("1", {
          absence_type: "comp_time",
          date_start: "2026-07-27",
          date_end: "2026-07-28",
          note: "FZA",
        }),
      ).rejects.toThrow(
        "Der Freizeitausgleich übersteigt die vor dem Startdatum verfügbaren Plus-Stunden.",
      );
    });

    it("deletes an absence (#1843)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({ ok: true } as Response);

      await staffAbsenceService.deleteAbsence("1", 42);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/absences/42",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("uses response text when deleting an absence fails (#1843)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve("Nicht gefunden"),
      } as Response);

      await expect(staffAbsenceService.deleteAbsence("1", 99)).rejects.toThrow(
        "Nicht gefunden",
      );
    });
  });

  describe("staffSessionEditsService", () => {
    it("loads and maps session edits", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              {
                id: 3,
                session_id: 4,
                staff_id: 1,
                edited_by: 2,
                field_name: "notes",
                old_value: null,
                new_value: "Updated",
                notes: null,
                created_at: "2026-06-01T08:00:00Z",
              },
            ],
          }),
      } as Response);

      const result = await staffSessionEditsService.getEdits("1", "4");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1/time-tracking/sessions/4/edits",
        expect.any(Object),
      );
      expect(result[0]?.fieldName).toBe("notes");
    });

    it("throws when session edits fail", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Unauthorized",
      } as Response);

      await expect(staffSessionEditsService.getEdits("1", "4")).rejects.toThrow(
        "Failed to fetch session edits: Unauthorized",
      );
    });
  });

  describe("staffSessionService", () => {
    const payload = {
      date: "2026-06-01",
      check_in_time: "2026-06-01T08:00:00Z",
      check_out_time: "2026-06-01T16:00:00Z",
      break_minutes: 30,
      status: "present" as const,
      notes: "Nachgetragen",
    };

    it("updates and creates staff sessions", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({ ok: true } as Response)
        .mockResolvedValueOnce({ ok: true } as Response);

      await staffSessionService.updateSession("1", "4", payload);
      await staffSessionService.createSession("1", payload);

      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/1/time-tracking/sessions/4",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify(payload),
        }),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "/api/staff/1/time-tracking/sessions",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(payload),
        }),
      );
    });

    it("uses response bodies for update and create errors", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.resolve("Invalid update"),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          statusText: "Bad Request",
          text: () => Promise.resolve(""),
        } as Response);

      await expect(
        staffSessionService.updateSession("1", "4", payload),
      ).rejects.toThrow("Invalid update");
      await expect(
        staffSessionService.createSession("1", payload),
      ).rejects.toThrow("Failed to create session: Bad Request");
    });

    // #2402: the overlap 409 carries a stable code; its message holds the
    // dynamic conflicting interval and is not presentable to a user.
    it("maps the work_session_overlap code to a German message", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const overlapBody = JSON.stringify({
        status: "error",
        error: "work session overlaps an existing block (08:00-12:00)",
        code: "work_session_overlap",
      });
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          statusText: "Conflict",
          text: () => Promise.resolve(overlapBody),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          statusText: "Conflict",
          text: () => Promise.resolve(overlapBody),
        } as Response);

      await expect(
        staffSessionService.updateSession("1", "4", payload),
      ).rejects.toThrow(
        "Der Zeitraum überschneidet sich mit einem anderen Arbeitsblock an diesem Tag.",
      );
      await expect(
        staffSessionService.createSession("1", payload),
      ).rejects.toThrow(
        "Der Zeitraum überschneidet sich mit einem anderen Arbeitsblock an diesem Tag.",
      );
    });
  });

  describe("staffBalanceAdjustmentService", () => {
    const backendAdjustment = {
      id: 17,
      type: "payout",
      minutes_delta: -120,
      effective_date: "2026-07-31",
      note: "Juligehalt",
      decided_by: 9,
      decided_at: "2026-07-24T08:00:00Z",
    };

    it("lists and maps adjustments, including an empty response", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [backendAdjustment] }),
        } as Response)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: null }),
        } as Response);

      const result = await staffBalanceAdjustmentService.list(
        "4",
        "2026-01-01",
        "9999-12-31",
      );
      const empty = await staffBalanceAdjustmentService.list(
        "4",
        "2026-01-01",
        "9999-12-31",
      );

      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/4/time-tracking/adjustments?from=2026-01-01&to=9999-12-31",
        expect.any(Object),
      );
      expect(result).toEqual([
        {
          id: "17",
          type: "payout",
          minutesDelta: -120,
          effectiveDate: "2026-07-31",
          note: "Juligehalt",
          decidedBy: "9",
          decidedAt: "2026-07-24T08:00:00Z",
        },
      ]);
      expect(empty).toEqual([]);
    });

    it("creates and resets adjustments with backend request keys", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: backendAdjustment }),
        } as Response)
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: {
                ...backendAdjustment,
                id: 18,
                type: "reset",
                minutes_delta: 60,
              },
            }),
        } as Response);

      const created = await staffBalanceAdjustmentService.create("4", {
        type: "payout",
        minutesDelta: -120,
        effectiveDate: "2026-07-31",
        note: "Juligehalt",
      });
      const reset = await staffBalanceAdjustmentService.reset("4", {
        effectiveDate: "2026-07-31",
        carryoverMinutes: 60,
        note: "Schuljahreswechsel",
      });

      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/staff/4/time-tracking/adjustments",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            type: "payout",
            minutes_delta: -120,
            effective_date: "2026-07-31",
            note: "Juligehalt",
          }),
        }),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "/api/staff/4/time-tracking/reset",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            effective_date: "2026-07-31",
            carryover_minutes: 60,
            note: "Schuljahreswechsel",
          }),
        }),
      );
      expect(created.id).toBe("17");
      expect(reset).toMatchObject({
        id: "18",
        type: "reset",
        minutesDelta: 60,
      });
    });

    it("deletes an adjustment", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({ ok: true } as Response);

      await staffBalanceAdjustmentService.delete("4", "17");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/4/time-tracking/adjustments/17",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("explains when an adjustment exceeds the accrued balance", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              error: "balance adjustment exceeds accrued balance",
              code: "balance_adjustment_exceeds_balance",
            }),
          ),
      } as Response);

      await expect(
        staffBalanceAdjustmentService.create("4", {
          type: "payout",
          minutesDelta: -120,
          effectiveDate: "2026-07-31",
          note: "Juligehalt",
        }),
      ).rejects.toThrow(
        "Die Buchung übersteigt die zum gewählten Datum verfügbaren Plus-Stunden.",
      );
    });

    it("explains when a reset would invalidate later balance reductions", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              error: "balance adjustment exceeds accrued balance",
              code: "balance_adjustment_exceeds_balance",
            }),
          ),
      } as Response);

      await expect(
        staffBalanceAdjustmentService.reset("4", {
          effectiveDate: "2026-07-31",
          carryoverMinutes: 0,
          note: "Schuljahreswechsel",
        }),
      ).rejects.toThrow(
        "Der Reset kann nicht durchgeführt werden, weil spätere Buchungen oder Freizeitausgleichstage vom aktuellen Guthaben abhängen.",
      );
    });

    it("explains when deleting a positive reset would overdraw later entries", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              error: "balance adjustment exceeds accrued balance",
              code: "balance_adjustment_exceeds_balance",
            }),
          ),
      } as Response);

      await expect(
        staffBalanceAdjustmentService.delete("4", "17"),
      ).rejects.toThrow(
        "Die Buchung kann nicht gelöscht werden, weil spätere Abzüge vom dadurch entstehenden Guthaben abhängen.",
      );
    });

    it("surfaces list, create, delete, reset, and coded reset conflicts", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          statusText: "Bad Gateway",
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.resolve("Ungültige Buchung"),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          text: () => Promise.reject(new Error("unreadable")),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          text: () =>
            Promise.resolve(
              JSON.stringify({
                error: "duplicate",
                code: "balance_already_reset",
              }),
            ),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          text: () =>
            Promise.resolve(
              JSON.stringify({
                error: "dependent",
                code: "dependent_balance_reset",
              }),
            ),
        } as Response)
        .mockResolvedValueOnce({
          ok: false,
          status: 400,
          text: () => Promise.resolve("Ungültiger Reset"),
        } as Response);

      await expect(
        staffBalanceAdjustmentService.list("4", "2026-01-01", "2026-12-31"),
      ).rejects.toThrow("Failed to fetch adjustments: Bad Gateway");
      await expect(
        staffBalanceAdjustmentService.create("4", {
          type: "payout",
          minutesDelta: -60,
          effectiveDate: "2026-07-31",
          note: "x",
        }),
      ).rejects.toThrow("Ungültige Buchung");
      await expect(
        staffBalanceAdjustmentService.delete("4", "17"),
      ).rejects.toThrow("Löschen fehlgeschlagen");
      await expect(
        staffBalanceAdjustmentService.reset("4", {
          effectiveDate: "2026-07-31",
          carryoverMinutes: 0,
          note: "x",
        }),
      ).rejects.toThrow(
        "Das Stundenkonto wurde für dieses Datum bereits zurückgesetzt.",
      );
      await expect(
        staffBalanceAdjustmentService.reset("4", {
          effectiveDate: "2026-07-31",
          carryoverMinutes: 0,
          note: "x",
        }),
      ).rejects.toThrow(
        "Der Reset liegt vor einem späteren Reset und würde dessen Saldo verfälschen.",
      );
      await expect(
        staffBalanceAdjustmentService.reset("4", {
          effectiveDate: "2026-07-31",
          carryoverMinutes: 0,
          note: "x",
        }),
      ).rejects.toThrow("Ungültiger Reset");
    });

    // Eröffnungssaldo (#2132). The backend's stable error codes carry no German
    // text; the copy the admin reads lives here, so it is pinned here too.
    it("maps the opening balance response", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: 21,
              staff_id: 4,
              type: "opening",
              minutes_delta: -330,
              effective_date: "2026-07-31",
              note: "Übernahme aus Altsystem",
              decided_by: 9,
              decided_at: "2026-08-01T08:00:00Z",
            },
          }),
      } as Response);

      const opening = await staffBalanceAdjustmentService.createOpening("4", {
        effectiveDate: "2026-07-31",
        balanceMinutes: -330,
        note: "Übernahme aus Altsystem",
      });

      expect(opening).toMatchObject({
        id: "21",
        type: "opening",
        minutesDelta: -330,
      });
    });

    it.each([
      [
        "opening_balance_already_exists",
        "Für diese Person existiert bereits ein Eröffnungssaldo. Lösche zuerst die bestehende Buchung.",
      ],
      [
        "dependent_balance_reset",
        "Es existieren bereits spätere Buchungen (Reset), die vom Stichtag abhängen.",
      ],
      [
        "adjustment_in_closed_month",
        "Der gewählte Monat ist abgeschlossen. Wähle ein Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
      ],
    ])("explains the opening rejection %s", async (code, message) => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        text: () =>
          Promise.resolve(JSON.stringify({ error: "rejected", code })),
      } as Response);

      await expect(
        staffBalanceAdjustmentService.createOpening("4", {
          effectiveDate: "2026-07-31",
          balanceMinutes: 600,
          note: "Übernahme",
        }),
      ).rejects.toThrow(message);
    });

    it("passes an unmapped opening rejection through verbatim", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: () =>
          Promise.resolve(
            JSON.stringify({ error: "Stichtag liegt in der Zukunft" }),
          ),
      } as Response);

      await expect(
        staffBalanceAdjustmentService.createOpening("4", {
          effectiveDate: "2099-01-01",
          balanceMinutes: 600,
          note: "Übernahme",
        }),
      ).rejects.toThrow("Stichtag liegt in der Zukunft");
    });
  });

  // Stammdaten (#1423). The section writes carry the snake_case wire shape the
  // backend validates, and the financial endpoints are the only ones that ever
  // return plaintext — both are pinned here.
  describe("staffService.getFinancialProfile", () => {
    it("maps the reduced payroll profile and flags it", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: 42,
              name: "Erika Musterfrau",
              firstName: "Erika",
              lastName: "Musterfrau",
            },
          }),
      } as unknown as Response);

      const result = await staffService.getFinancialProfile("42");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/financial-profile/42",
        expect.any(Object),
      );
      expect(result.id).toBe("42");
      expect(result.name).toBe("Erika Musterfrau");
      expect(result.isFinancialProfile).toBe(true);
      expect(result.hasRfid).toBe(false);
      expect(result.isTeacher).toBe(false);
      expect(result.isSupervising).toBe(false);
      expect(result.supervisions).toEqual([]);
    });

    it("throws when the reduced profile is refused", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffService.getFinancialProfile("42")).rejects.toThrow(
        "Failed to fetch financial staff profile: Forbidden",
      );
    });
  });

  describe("staffStammdatenService", () => {
    const backendStammdaten = {
      staff_id: 7,
      person: {
        first_name: "Max",
        last_name: "Mustermann",
        birthday: "1991-06-02",
        gender: "male" as const,
      },
      kontakt: {
        address_street: "Musterweg 1",
        address_postal_code: "48143",
        address_city: "Münster",
        phone: "+49 251 1",
        email: "max@example.com",
        emergency_contact_name: "Erika Muster",
        emergency_contact_phone: "+49 170 1",
      },
      arbeitsvertrag: {
        entry_date: "2024-08-01",
        contract_end_date: null,
        probation_end_date: "2025-01-31",
        weekly_hours: 29.5,
        employment_type: "part_time",
      },
      qualifikationen: [
        {
          id: 3,
          name: "Erste-Hilfe-Kurs",
          acquired_on: "2023-03-10",
          expires_on: "2026-03-10",
        },
        { name: "Schwimmschein", acquired_on: null, expires_on: null },
      ],
    };

    it("maps the aggregate response to camelCase", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: backendStammdaten }),
      } as unknown as Response);

      const result = await staffStammdatenService.get("7");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/7/stammdaten",
        expect.any(Object),
      );
      expect(result.staffId).toBe("7");
      expect(result.person).toEqual({
        firstName: "Max",
        lastName: "Mustermann",
        birthday: "1991-06-02",
        gender: "male",
      });
      expect(result.kontakt.addressPostalCode).toBe("48143");
      expect(result.kontakt.emergencyContactName).toBe("Erika Muster");
      expect(result.arbeitsvertrag).toEqual({
        entryDate: "2024-08-01",
        contractEndDate: null,
        probationEndDate: "2025-01-31",
        weeklyHours: 29.5,
        employmentType: "part_time",
      });
      // Persisted rows carry an id, a freshly typed one does not.
      expect(result.qualifikationen).toEqual([
        {
          id: "3",
          name: "Erste-Hilfe-Kurs",
          acquiredOn: "2023-03-10",
          expiresOn: "2026-03-10",
        },
        {
          id: null,
          name: "Schwimmschein",
          acquiredOn: null,
          expiresOn: null,
        },
      ]);
    });

    it("tolerates a missing qualification list", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { ...backendStammdaten, qualifikationen: undefined },
          }),
      } as unknown as Response);

      const result = await staffStammdatenService.get("7");

      expect(result.qualifikationen).toEqual([]);
    });

    it("throws when the aggregate read is refused", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffStammdatenService.get("7")).rejects.toThrow(
        "Failed to fetch stammdaten: Forbidden",
      );
    });

    it("writes the person section in the backend wire shape", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({ ok: true } as Response);

      await staffStammdatenService.updatePerson(
        "7",
        {
          firstName: "Max",
          lastName: "Neu",
          birthday: "1991-06-02",
          gender: "diverse",
        },
        "Namensänderung",
      );

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/person");
      expect(init.method).toBe("PUT");
      expect(JSON.parse(init.body as string)).toEqual({
        first_name: "Max",
        last_name: "Neu",
        birthday: "1991-06-02",
        gender: "diverse",
        note: "Namensänderung",
      });
    });

    it("writes the kontakt section in the backend wire shape", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({ ok: true } as Response);

      await staffStammdatenService.updateKontakt(
        "7",
        {
          addressStreet: "Musterweg 1",
          addressPostalCode: "48143",
          addressCity: "Münster",
          phone: "+49 251 1",
          email: "max@example.com",
          emergencyContactName: "Erika Muster",
          emergencyContactPhone: null,
        },
        "Umzug",
      );

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/kontakt");
      expect(JSON.parse(init.body as string)).toEqual({
        address_street: "Musterweg 1",
        address_postal_code: "48143",
        address_city: "Münster",
        phone: "+49 251 1",
        email: "max@example.com",
        emergency_contact_name: "Erika Muster",
        emergency_contact_phone: null,
        note: "Umzug",
      });
    });

    it("writes the arbeitsvertrag section in the backend wire shape", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({ ok: true } as Response);

      await staffStammdatenService.updateArbeitsvertrag(
        "7",
        {
          entryDate: "2024-08-01",
          contractEndDate: null,
          probationEndDate: "2025-01-31",
          weeklyHours: 29.5,
          employmentType: "part_time",
        },
        "Vertrag",
      );

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/arbeitsvertrag");
      expect(JSON.parse(init.body as string)).toEqual({
        entry_date: "2024-08-01",
        contract_end_date: null,
        probation_end_date: "2025-01-31",
        weekly_hours: 29.5,
        employment_type: "part_time",
        note: "Vertrag",
      });
    });

    it("sends the full qualification list without client-side ids", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({ ok: true } as Response);

      await staffStammdatenService.updateQualifikationen(
        "7",
        [
          {
            id: "3",
            name: "Erste-Hilfe-Kurs",
            acquiredOn: "2023-03-10",
            expiresOn: "2026-03-10",
          },
          {
            id: null,
            name: "Schwimmschein",
            acquiredOn: null,
            expiresOn: null,
          },
        ],
        "Nachweise",
      );

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/qualifikationen");
      expect(JSON.parse(init.body as string)).toEqual({
        qualifikationen: [
          {
            name: "Erste-Hilfe-Kurs",
            acquired_on: "2023-03-10",
            expires_on: "2026-03-10",
          },
          { name: "Schwimmschein", acquired_on: null, expires_on: null },
        ],
        note: "Nachweise",
      });
    });

    it("translates the stammdaten_invalid code into a German hint", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        text: () =>
          Promise.resolve(
            JSON.stringify({
              code: "stammdaten_invalid",
              error: "invalid IBAN checksum",
            }),
          ),
      } as unknown as Response);

      await expect(
        staffStammdatenService.updatePerson(
          "7",
          {
            firstName: "Max",
            lastName: "Neu",
            birthday: null,
            gender: null,
          },
          "",
        ),
      ).rejects.toThrow(
        "Ungültige Eingabe. Bitte prüfe die Werte und versuche es erneut.",
      );
    });

    it("surfaces other backend errors verbatim", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Conflict",
        text: () =>
          Promise.resolve(JSON.stringify({ error: "Mitarbeiter gesperrt" })),
      } as unknown as Response);

      await expect(
        staffStammdatenService.updateKontakt(
          "7",
          {
            addressStreet: null,
            addressPostalCode: null,
            addressCity: null,
            phone: null,
            email: null,
            emergencyContactName: null,
            emergencyContactPhone: null,
          },
          "",
        ),
      ).rejects.toThrow("Mitarbeiter gesperrt");
    });

    it("falls back to the generic save message on an empty error body", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Internal Server Error",
        text: () => Promise.resolve(""),
      } as unknown as Response);

      await expect(
        staffStammdatenService.updateArbeitsvertrag(
          "7",
          {
            entryDate: null,
            contractEndDate: null,
            probationEndDate: null,
            weeklyHours: null,
            employmentType: null,
          },
          "",
        ),
      ).rejects.toThrow("Stammdaten konnten nicht gespeichert werden");
    });

    it("reads the masked bank & tax values", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              iban_masked: "•••• 3000",
              tax_id_masked: "•••• 8911",
              social_security_number_masked: null,
            },
          }),
      } as unknown as Response);

      const result = await staffStammdatenService.getFinancial("7");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/7/stammdaten/bank-steuer",
        expect.any(Object),
      );
      expect(result).toEqual({
        ibanMasked: "•••• 3000",
        taxIdMasked: "•••• 8911",
        socialSecurityNumberMasked: null,
      });
    });

    it("throws when the masked read is refused", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffStammdatenService.getFinancial("7")).rejects.toThrow(
        "Failed to fetch financial data: Forbidden",
      );
    });

    it("reveals the plaintext values via POST (audited server-side)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              iban: "DE89370400440532013000",
              tax_id: "12345678911",
              social_security_number: "65170839J003",
            },
          }),
      } as unknown as Response);

      const result = await staffStammdatenService.revealFinancial("7");

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/bank-steuer/reveal");
      expect(init.method).toBe("POST");
      expect(result).toEqual({
        iban: "DE89370400440532013000",
        taxId: "12345678911",
        socialSecurityNumber: "65170839J003",
      });
    });

    it("throws when the reveal is refused", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      } as Response);

      await expect(staffStammdatenService.revealFinancial("7")).rejects.toThrow(
        "Failed to reveal financial data: Forbidden",
      );
    });

    it("writes the bank & tax section in the backend wire shape", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValue({ ok: true } as Response);

      await staffStammdatenService.updateFinancial(
        "7",
        {
          iban: "DE89370400440532013000",
          taxId: "12345678911",
          socialSecurityNumber: null,
        },
        "Lohnbüro",
      );

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/stammdaten/bank-steuer");
      expect(init.method).toBe("PUT");
      expect(JSON.parse(init.body as string)).toEqual({
        iban: "DE89370400440532013000",
        tax_id: "12345678911",
        social_security_number: null,
        note: "Lohnbüro",
      });
    });
  });
});
