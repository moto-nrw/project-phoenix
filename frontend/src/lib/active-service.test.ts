/* eslint-disable @typescript-eslint/unbound-method */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type {
  BackendCombinedGroup,
  BackendGroupMapping,
  BackendAnalytics,
} from "./active-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { mockSessionData } from "~/test/mocks/next-auth";
import {
  buildBackendActiveSession,
  buildBackendVisit,
  buildBackendSupervisor,
} from "~/test/fixtures";

// Mock dependencies before importing the module
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

// Import after mocks are set up
import { getSession } from "next-auth/react";
import api from "./api";
import { activeService } from "./active-service";

// Type for mocked functions
const mockedGetSession = vi.mocked(getSession);
const mockedApiGet = vi.mocked(api.get);
const mockedApiPost = vi.mocked(api.post);
const mockedApiPut = vi.mocked(api.put);
const mockedApiDelete = vi.mocked(api.delete);

// Sample backend data
const sampleBackendActiveGroup = buildBackendActiveSession({
  id: 1,
  group_id: 10,
  room_id: 5,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Morning session",
  visit_count: 25,
  supervisor_count: 2,
  room: { id: 5, name: "Room A", category: "classroom" },
  actual_group: { id: 10, name: "Class 3A" },
});

const sampleBackendVisit = buildBackendVisit({
  id: 100,
  student_id: 50,
  active_group_id: 1,
  check_in_time: "2024-01-15T08:30:00Z",
  check_out_time: "2024-01-15T11:45:00Z",
  is_active: false,
  notes: "Early checkout",
  student_name: "Max Mustermann",
  school_class: "3a",
  group_name: "OGS Group A",
  active_group_name: "Morning Session",
});

const sampleBackendSupervisor = buildBackendSupervisor({
  id: 200,
  staff_id: 30,
  active_group_id: 1,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Primary supervisor",
  staff_name: "Frau Schmidt",
  active_group_name: "Morning Session",
});

const sampleBackendCombinedGroup: BackendCombinedGroup = {
  id: 300,
  name: "Combined Morning",
  description: "Combined session",
  room_id: 5,
  start_time: "2024-01-15T08:00:00Z",
  is_active: true,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T08:00:00Z",
};

const sampleBackendGroupMapping: BackendGroupMapping = {
  id: 400,
  active_group_id: 1,
  combined_group_id: 300,
  group_name: "Class 3A",
  combined_name: "Combined Morning",
};

const sampleBackendAnalytics: BackendAnalytics = {
  active_groups_count: 5,
  total_visits_count: 150,
  active_visits_count: 45,
  room_utilization: 0.75,
  attendance_rate: 0.92,
};

describe("active-service", () => {
  const consoleSpies = suppressConsole("error", "warn");
  let originalFetch: typeof fetch;
  let originalWindow: typeof globalThis.window;

  beforeEach(() => {
    vi.clearAllMocks();
    originalFetch = globalThis.fetch;
    originalWindow = globalThis.window;
    globalThis.fetch = vi.fn();

    // Default session mock
    mockedGetSession.mockResolvedValue(mockSessionData());

    // Default: simulate browser context
    // @ts-expect-error - mocking window
    globalThis.window = {};
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
  });

  describe("Active Groups", () => {
    describe("getActiveGroups", () => {
      it("fetches active groups via proxy in browser context", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendActiveGroup] }),
        } as Response);

        const result = await activeService.getActiveGroups();

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups",
          expect.objectContaining({
            method: "GET",
            headers: expect.objectContaining({
              Authorization: "Bearer test-token",
            }),
          }),
        );
        expect(result).toHaveLength(1);
        expect(result[0]?.id).toBe("1");
        expect(result[0]?.groupId).toBe("10");
      });

      it("applies active filter when provided", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendActiveGroup] }),
        } as Response);

        await activeService.getActiveGroups({ active: true });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups?active=true",
          expect.any(Object),
        );
      });

      it("handles paginated response format", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { data: [sampleBackendActiveGroup] },
              pagination: { page: 1, total: 1 },
            }),
        } as Response);

        const result = await activeService.getActiveGroups();

        expect(result).toHaveLength(1);
      });

      it("throws error on fetch failure", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Internal error"),
        } as Response);

        await expect(activeService.getActiveGroups()).rejects.toThrow(
          "Get active groups failed: 500",
        );
      });

      it("uses axios in server context", async () => {
        // @ts-expect-error - simulating server context
        globalThis.window = undefined;

        // Note: axios response has { data: ... } wrapper, and backend response
        // is also wrapped in { data: [...] }, so the structure is { data: { data: [...] } }
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendActiveGroup] },
        });

        const result = await activeService.getActiveGroups();

        expect(mockedApiGet).toHaveBeenCalledWith(
          "http://localhost:8080/active/groups",
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("getActiveGroup", () => {
      it("fetches single active group by ID", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendActiveGroup }),
        } as Response);

        const result = await activeService.getActiveGroup("1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1",
          expect.any(Object),
        );
        expect(result.id).toBe("1");
        expect(result.notes).toBe("Morning session");
      });
    });

    describe("getActiveGroupsByRoom", () => {
      it("fetches active groups for a specific room", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendActiveGroup] }),
        } as Response);

        const result = await activeService.getActiveGroupsByRoom("5");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/room/5",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("getActiveGroupsByGroup", () => {
      it("fetches active groups for a specific group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendActiveGroup] }),
        } as Response);

        const result = await activeService.getActiveGroupsByGroup("10");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/group/10",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("createActiveGroup", () => {
      it("creates a new active group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendActiveGroup }),
        } as Response);

        const result = await activeService.createActiveGroup({
          groupId: "10",
          roomId: "5",
          startTime: new Date("2024-01-15T08:00:00Z"),
        });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups",
          expect.objectContaining({
            method: "POST",
            body: expect.stringContaining('"group_id":10'),
          }),
        );
        expect(result.id).toBe("1");
      });
    });

    describe("updateActiveGroup", () => {
      it("updates an existing active group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendActiveGroup, notes: "Updated" },
            }),
        } as Response);

        const result = await activeService.updateActiveGroup("1", {
          notes: "Updated",
        });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1",
          expect.objectContaining({
            method: "PUT",
          }),
        );
        expect(result.notes).toBe("Updated");
      });
    });

    describe("deleteActiveGroup", () => {
      it("deletes an active group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({}),
        } as Response);

        await activeService.deleteActiveGroup("1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1",
          expect.objectContaining({
            method: "DELETE",
          }),
        );
      });
    });

    describe("endActiveGroup", () => {
      it("ends an active group session", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendActiveGroup, is_active: false },
            }),
        } as Response);

        const result = await activeService.endActiveGroup("1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1/end",
          expect.objectContaining({
            method: "POST",
          }),
        );
        expect(result.isActive).toBe(false);
      });
    });
  });

  describe("Visits", () => {
    describe("getVisits", () => {
      it("fetches all visits", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendVisit] }),
        } as Response);

        const result = await activeService.getVisits();

        expect(result).toHaveLength(1);
        expect(result[0]?.studentName).toBe("Max Mustermann");
      });

      it("applies active filter", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendVisit] }),
        } as Response);

        await activeService.getVisits({ active: false });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits?active=false",
          expect.any(Object),
        );
      });
    });

    describe("getVisit", () => {
      it("fetches a single visit", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendVisit }),
        } as Response);

        const result = await activeService.getVisit("100");

        expect(result.id).toBe("100");
        expect(result.studentId).toBe("50");
      });
    });

    describe("getStudentVisits", () => {
      it("fetches visits for a student", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendVisit] }),
        } as Response);

        const result = await activeService.getStudentVisits("50");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits/student/50",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("getStudentCurrentVisit", () => {
      it("returns current visit when found", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendVisit, is_active: true },
            }),
        } as Response);

        const result = await activeService.getStudentCurrentVisit("50");

        expect(result).not.toBeNull();
        expect(result?.isActive).toBe(true);
      });

      it("returns null when no current visit", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: null }),
        } as Response);

        const result = await activeService.getStudentCurrentVisit("50");

        expect(result).toBeNull();
      });
    });

    describe("createVisit", () => {
      it("creates a new visit", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendVisit }),
        } as Response);

        const result = await activeService.createVisit({
          studentId: "50",
          activeGroupId: "1",
          checkInTime: new Date("2024-01-15T08:30:00Z"),
        });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits",
          expect.objectContaining({
            method: "POST",
          }),
        );
        expect(result.id).toBe("100");
      });
    });

    describe("endVisit", () => {
      it("ends a visit", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendVisit, is_active: false },
            }),
        } as Response);

        const result = await activeService.endVisit("100");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits/100/end",
          expect.objectContaining({
            method: "POST",
          }),
        );
        expect(result.isActive).toBe(false);
      });
    });

    describe("getActiveGroupVisitsWithDisplay", () => {
      it("fetches visits with display data", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendVisit] }),
        } as Response);

        const result = await activeService.getActiveGroupVisitsWithDisplay("1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1/visits/display",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
        expect(result[0]?.schoolClass).toBe("3a");
      });

      it("returns empty array on 404", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 404,
        } as Response);

        const result =
          await activeService.getActiveGroupVisitsWithDisplay("999");

        expect(result).toEqual([]);
      });

      it("throws on other errors", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Server error"),
        } as Response);

        await expect(
          activeService.getActiveGroupVisitsWithDisplay("1"),
        ).rejects.toThrow("Get visits with display failed: 500");
      });
    });
  });

  describe("Supervisors", () => {
    describe("getSupervisors", () => {
      it("fetches all supervisors", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendSupervisor] }),
        } as Response);

        const result = await activeService.getSupervisors();

        expect(result).toHaveLength(1);
        expect(result[0]?.staffName).toBe("Frau Schmidt");
      });
    });

    describe("getSupervisor", () => {
      it("fetches a single supervisor", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendSupervisor }),
        } as Response);

        const result = await activeService.getSupervisor("200");

        expect(result.id).toBe("200");
        expect(result.staffId).toBe("30");
      });
    });

    describe("getStaffSupervisions", () => {
      it("fetches supervisions for a staff member", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendSupervisor] }),
        } as Response);

        const result = await activeService.getStaffSupervisions("30");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/supervisors/staff/30",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("getStaffActiveSupervisions", () => {
      it("fetches active supervisions for a staff member", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendSupervisor] }),
        } as Response);

        const result = await activeService.getStaffActiveSupervisions("30");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/supervisors/staff/30/active",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });
    });

    describe("createSupervisor", () => {
      it("creates a new supervisor", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendSupervisor }),
        } as Response);

        const result = await activeService.createSupervisor({
          staffId: "30",
          activeGroupId: "1",
          startTime: new Date("2024-01-15T08:00:00Z"),
        });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/supervisors",
          expect.objectContaining({
            method: "POST",
          }),
        );
        expect(result.id).toBe("200");
      });
    });

    describe("endSupervision", () => {
      it("ends a supervision", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendSupervisor, is_active: false },
            }),
        } as Response);

        const result = await activeService.endSupervision("200");

        expect(result.isActive).toBe(false);
      });
    });
  });

  describe("Combined Groups", () => {
    describe("getCombinedGroups", () => {
      it("fetches all combined groups", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendCombinedGroup] }),
        } as Response);

        const result = await activeService.getCombinedGroups();

        expect(result).toHaveLength(1);
        expect(result[0]?.name).toBe("Combined Morning");
      });
    });

    describe("getActiveCombinedGroups", () => {
      it("fetches only active combined groups", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendCombinedGroup] }),
        } as Response);

        await activeService.getActiveCombinedGroups();

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/combined/active",
          expect.any(Object),
        );
      });
    });

    describe("createCombinedGroup", () => {
      it("creates a new combined group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendCombinedGroup }),
        } as Response);

        const result = await activeService.createCombinedGroup({
          name: "Combined Morning",
          roomId: "5",
          startTime: new Date("2024-01-15T08:00:00Z"),
        });

        expect(result.id).toBe("300");
      });
    });

    describe("endCombinedGroup", () => {
      it("ends a combined group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { ...sampleBackendCombinedGroup, is_active: false },
            }),
        } as Response);

        const result = await activeService.endCombinedGroup("300");

        expect(result.isActive).toBe(false);
      });
    });
  });

  describe("Group Mappings", () => {
    describe("getGroupMappingsByGroup", () => {
      it("fetches mappings for a group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendGroupMapping] }),
        } as Response);

        const result = await activeService.getGroupMappingsByGroup("1");

        expect(result).toHaveLength(1);
        expect(result[0]?.groupName).toBe("Class 3A");
      });
    });

    describe("addGroupToCombination", () => {
      it("adds a group to a combined group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendGroupMapping }),
        } as Response);

        const result = await activeService.addGroupToCombination("1", "300");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/mappings/add",
          expect.objectContaining({
            method: "POST",
            body: expect.stringContaining('"active_group_id":1'),
          }),
        );
        expect(result.id).toBe("400");
      });
    });

    describe("removeGroupFromCombination", () => {
      it("removes a group from a combined group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({}),
        } as Response);

        await activeService.removeGroupFromCombination("1", "300");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/mappings/remove",
          expect.objectContaining({
            method: "POST",
          }),
        );
      });
    });
  });

  describe("Analytics", () => {
    describe("getAnalyticsCounts", () => {
      it("fetches analytics counts", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendAnalytics }),
        } as Response);

        const result = await activeService.getAnalyticsCounts();

        expect(result.activeGroupsCount).toBe(5);
        expect(result.totalVisitsCount).toBe(150);
      });
    });

    describe("getRoomUtilization", () => {
      it("fetches room utilization analytics", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendAnalytics }),
        } as Response);

        const result = await activeService.getRoomUtilization("5");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/analytics/room/5/utilization",
          expect.any(Object),
        );
        expect(result.roomUtilization).toBe(0.75);
      });
    });

    describe("getStudentAttendance", () => {
      it("fetches student attendance analytics", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendAnalytics }),
        } as Response);

        const result = await activeService.getStudentAttendance("50");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/analytics/student/50/attendance",
          expect.any(Object),
        );
        expect(result.attendanceRate).toBe(0.92);
      });
    });
  });

  describe("Unclaimed Groups", () => {
    describe("getUnclaimedGroups", () => {
      it("fetches unclaimed groups with array response", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve([sampleBackendActiveGroup]),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/unclaimed",
          expect.any(Object),
        );
        expect(result).toHaveLength(1);
      });

      it("handles data wrapper", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendActiveGroup] }),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toHaveLength(1);
      });

      it("handles nested data wrapper", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: { data: [sampleBackendActiveGroup] },
            }),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toHaveLength(1);
      });

      it("handles items wrapper", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ items: [sampleBackendActiveGroup] }),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toHaveLength(1);
      });

      it("returns empty array for empty response", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [] }),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toEqual([]);
      });

      it("returns empty array for null response", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve(null),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toEqual([]);
      });

      it("logs warning for unexpected response format", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ unexpected: "format", value: 123 }),
        } as Response);

        const result = await activeService.getUnclaimedGroups();

        expect(result).toEqual([]);
        expect(consoleSpies.warn).toHaveBeenCalledWith(
          expect.stringContaining("Unexpected unclaimed groups response shape"),
          expect.anything(),
        );
      });

      it("throws on fetch failure", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Error"),
        } as Response);

        await expect(activeService.getUnclaimedGroups()).rejects.toThrow(
          "Get unclaimed groups failed: 500",
        );
      });
    });

    describe("claimActiveGroup", () => {
      it("claims an active group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({}),
        } as Response);

        await activeService.claimActiveGroup("1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/groups/1/claim",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({ role: "supervisor" }),
          }),
        );
      });
    });
  });

  describe("Student Checkout", () => {
    describe("checkoutStudent", () => {
      it("checks out a student", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({}),
        } as Response);

        await activeService.checkoutStudent("50");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits/student/50/checkout",
          expect.objectContaining({
            method: "POST",
          }),
        );
      });
    });
  });

  describe("Server Context (axios)", () => {
    beforeEach(() => {
      // @ts-expect-error - simulating server context
      globalThis.window = undefined;
    });

    it("uses axios for getActiveGroup in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: sampleBackendActiveGroup },
      });

      const result = await activeService.getActiveGroup("1");

      expect(mockedApiGet).toHaveBeenCalledWith(
        "http://localhost:8080/active/groups/1",
      );
      expect(result.id).toBe("1");
    });

    it("uses axios for createActiveGroup in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: { data: sampleBackendActiveGroup },
      });

      await activeService.createActiveGroup({
        groupId: "10",
        roomId: "5",
        startTime: new Date("2024-01-15T08:00:00Z"),
      });

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/active/groups",
        expect.any(Object),
      );
    });

    it("uses axios for deleteActiveGroup in server context", async () => {
      mockedApiDelete.mockResolvedValueOnce({ data: {} });

      await activeService.deleteActiveGroup("1");

      expect(mockedApiDelete).toHaveBeenCalledWith(
        "http://localhost:8080/active/groups/1",
      );
    });

    it("uses axios for updateActiveGroup in server context", async () => {
      mockedApiPut.mockResolvedValueOnce({
        data: { data: sampleBackendActiveGroup },
      });

      await activeService.updateActiveGroup("1", { notes: "Updated" });

      expect(mockedApiPut).toHaveBeenCalledWith(
        "http://localhost:8080/active/groups/1",
        expect.any(Object),
      );
    });

    it("uses axios for getUnclaimedGroups in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: [sampleBackendActiveGroup],
      });

      const result = await activeService.getUnclaimedGroups();

      expect(mockedApiGet).toHaveBeenCalledWith(
        "http://localhost:8080/active/groups/unclaimed",
      );
      expect(result).toHaveLength(1);
    });
  });

  describe("Schulhof (Schoolyard)", () => {
    describe("getSchulhofStatus", () => {
      it("fetches Schulhof status via proxy in browser context", async () => {
        const sampleBackendStatus: import("./active-helpers").BackendSchulhofStatus =
          {
            exists: true,
            room_id: 42,
            room_name: "Schulhof",
            activity_group_id: 10,
            active_group_id: 5,
            is_user_supervising: true,
            supervision_id: 123,
            supervisor_count: 2,
            student_count: 15,
            supervisors: [
              {
                id: 1,
                staff_id: 100,
                name: "Frau Schmidt",
                is_current_user: true,
              },
            ],
          };

        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendStatus }),
        } as Response);

        const result = await activeService.getSchulhofStatus();

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/schulhof/status",
          expect.objectContaining({
            method: "GET",
            headers: expect.objectContaining({
              Authorization: "Bearer test-token",
            }),
          }),
        );
        expect(result.exists).toBe(true);
        expect(result.roomId).toBe("42");
        expect(result.roomName).toBe("Schulhof");
        expect(result.isUserSupervising).toBe(true);
        expect(result.supervisorCount).toBe(2);
        expect(result.studentCount).toBe(15);
      });

      it("handles non-existent Schulhof", async () => {
        const sampleBackendStatus: import("./active-helpers").BackendSchulhofStatus =
          {
            exists: false,
            room_name: "",
            is_user_supervising: false,
            supervisor_count: 0,
            student_count: 0,
            supervisors: [],
          };

        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendStatus }),
        } as Response);

        const result = await activeService.getSchulhofStatus();

        expect(result.exists).toBe(false);
        expect(result.roomId).toBeNull();
        expect(result.supervisorCount).toBe(0);
      });

      it("throws error on fetch failure", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          text: () => Promise.resolve("Internal error"),
        } as Response);

        await expect(activeService.getSchulhofStatus()).rejects.toThrow(
          "Get Schulhof status failed: 500",
        );
      });

      it("uses axios in server context", async () => {
        // @ts-expect-error - simulating server context
        globalThis.window = undefined;

        const sampleBackendStatus: import("./active-helpers").BackendSchulhofStatus =
          {
            exists: true,
            room_name: "Schulhof",
            is_user_supervising: false,
            supervisor_count: 0,
            student_count: 0,
            supervisors: [],
          };

        mockedApiGet.mockResolvedValueOnce({
          data: { data: sampleBackendStatus },
        });

        const result = await activeService.getSchulhofStatus();

        expect(mockedApiGet).toHaveBeenCalledWith(
          "http://localhost:8080/active/schulhof/status",
        );
        expect(result.exists).toBe(true);
      });
    });

    describe("toggleSchulhofSupervision", () => {
      it("starts supervision via proxy in browser context", async () => {
        const sampleBackendResponse: import("./active-helpers").BackendToggleSupervisionResponse =
          {
            action: "started",
            supervision_id: 456,
            active_group_id: 789,
          };

        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendResponse }),
        } as Response);

        const result =
          await activeService.toggleSchulhofSupervision("start");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/schulhof/supervise",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({ action: "start" }),
            headers: expect.objectContaining({
              Authorization: "Bearer test-token",
            }),
          }),
        );
        expect(result.action).toBe("started");
        expect(result.supervisionId).toBe("456");
        expect(result.activeGroupId).toBe("789");
      });

      it("stops supervision via proxy in browser context", async () => {
        const sampleBackendResponse: import("./active-helpers").BackendToggleSupervisionResponse =
          {
            action: "stopped",
            active_group_id: 789,
          };

        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: sampleBackendResponse }),
        } as Response);

        const result = await activeService.toggleSchulhofSupervision("stop");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/schulhof/supervise",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({ action: "stop" }),
          }),
        );
        expect(result.action).toBe("stopped");
        expect(result.supervisionId).toBeNull();
      });

      it("throws error on fetch failure", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 400,
          text: () => Promise.resolve("Invalid action"),
        } as Response);

        await expect(
          activeService.toggleSchulhofSupervision("start"),
        ).rejects.toThrow("Toggle Schulhof supervision failed: 400");
      });

      it("uses axios in server context for start", async () => {
        // @ts-expect-error - simulating server context
        globalThis.window = undefined;

        const sampleBackendResponse: import("./active-helpers").BackendToggleSupervisionResponse =
          {
            action: "started",
            supervision_id: 111,
            active_group_id: 222,
          };

        mockedApiPost.mockResolvedValueOnce({
          data: { data: sampleBackendResponse },
        });

        const result =
          await activeService.toggleSchulhofSupervision("start");

        expect(mockedApiPost).toHaveBeenCalledWith(
          "http://localhost:8080/active/schulhof/supervise",
          { action: "start" },
        );
        expect(result.action).toBe("started");
      });

      it("uses axios in server context for stop", async () => {
        // @ts-expect-error - simulating server context
        globalThis.window = undefined;

        const sampleBackendResponse: import("./active-helpers").BackendToggleSupervisionResponse =
          {
            action: "stopped",
            active_group_id: 222,
          };

        mockedApiPost.mockResolvedValueOnce({
          data: { data: sampleBackendResponse },
        });

        const result = await activeService.toggleSchulhofSupervision("stop");

        expect(mockedApiPost).toHaveBeenCalledWith(
          "http://localhost:8080/active/schulhof/supervise",
          { action: "stop" },
        );
        expect(result.action).toBe("stopped");
      });
    });
  });
});
