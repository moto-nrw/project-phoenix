/* eslint-disable @typescript-eslint/no-empty-function */
/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-misused-promises */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { BackendStaffResponse, StaffFilters } from "./staff-api";

// Import after mocks are set up
import { staffService } from "./staff-api";

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
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
  let originalFetch: typeof fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
    consoleWarnSpy.mockRestore();
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
      expect(result[0]?.currentLocation).toBe("Zuhause");
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
      expect(result[0]?.currentLocation).toBe("2 Räume");
      expect(result[0]?.supervisionRole).toBe("supervisor");
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
      expect(result[0]?.currentLocation).toBe("Zuhause");
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

    it("returns Zuhause location for staff with was_present_today false", async () => {
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
      expect(result[0]?.currentLocation).toBe("Zuhause");
      expect(result[0]?.wasPresentToday).toBe(false);
    });

    it("returns room location when supervising even if was_present_today is true", async () => {
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

      // Staff is supervising, so should show room location, not "Anwesend"
      expect(result[0]?.isSupervising).toBe(true);
      expect(result[0]?.currentLocation).toBe("2 Räume");
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining("Error fetching supervisions for staff 1"),
        expect.any(Error),
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        expect.stringContaining("Error fetching supervisions for staff 1"),
        expect.any(Error),
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
});
