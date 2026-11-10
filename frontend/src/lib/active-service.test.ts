import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { BackendSchulhofStatus } from "./active-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { mockSessionData } from "~/test/mocks/next-auth";
import {
  buildBackendActiveSession,
  buildBackendVisit,
  buildBackendSupervisor,
} from "~/test/fixtures/sessions";

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
import { activeService } from "./active-service";

// Type for mocked functions
const mockedGetSession = vi.mocked(getSession);

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

describe("active-service", () => {
  suppressConsole("error", "warn");
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
            headers: expect.not.objectContaining({
              Authorization: expect.anything(),
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

      it("filters active visits by several active groups in one request", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [sampleBackendVisit] }),
        } as Response);

        await activeService.getVisits({
          active: true,
          activeGroupIds: ["17", "23"],
        });

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits?active=true&active_group_ids=17%2C23",
          expect.any(Object),
        );
      });
    });

    describe("student movement", () => {
      it("assigns transit students to an active group", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: {
                assigned: [50],
                skipped: [],
                active_group_id: 1,
                room_id: 5,
              },
            }),
        } as Response);

        const result = await activeService.assignTransitStudents(["50"], "1");

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits/transit/assign",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({
              student_ids: [50],
              active_group_id: 1,
            }),
          }),
        );
        expect(result.assigned).toEqual([50]);
      });

      it("keeps the code and details of a refused assignment (#3633)", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 409,
          text: () =>
            Promise.resolve(
              JSON.stringify({
                error: "room capacity exceeded: Aula (30/30)",
                code: "presence.room_capacity_exceeded",
                details: { room_name: "Aula", max_capacity: 30 },
              }),
            ),
        } as Response);

        await expect(
          activeService.assignTransitStudents(["50"], "1"),
        ).rejects.toMatchObject({
          message: "Assign transit students failed: 409",
          status: 409,
          code: "presence.room_capacity_exceeded",
          details: { room_name: "Aula", max_capacity: 30 },
        });
      });

      it("keeps the code and details when moving into a full room (#3633)", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 409,
          text: () =>
            Promise.resolve(
              JSON.stringify({
                error: "room capacity exceeded: Aula (30/30)",
                code: "presence.room_capacity_exceeded",
                details: { room_name: "Aula", max_capacity: 30 },
              }),
            ),
        } as Response);

        await expect(
          activeService.moveStudentsToOpenRoom(["50"], "7"),
        ).rejects.toMatchObject({
          message: "Move students to open room failed: 409",
          status: 409,
          code: "presence.room_capacity_exceeded",
          details: { room_name: "Aula", max_capacity: 30 },
        });
      });

      it("moves students to an active group through the batch move endpoint", async () => {
        const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              data: {
                moved: [50, 51],
                unchanged: [],
                skipped: [],
                active_group_id: 1,
                room_id: 5,
              },
            }),
        } as Response);

        const result = await activeService.moveStudentsToActiveGroup(
          ["50", "51"],
          "1",
        );

        expect(mockFetch).toHaveBeenCalledWith(
          "/api/active/visits/move-to-group",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({
              student_ids: [50, 51],
              target_active_group_id: 1,
            }),
          }),
        );
        expect(result.moved).toEqual([50, 51]);
      });
    });
  });

  describe("Supervisors", () => {
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

  describe("Unclaimed Groups", () => {
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

  describe("Schulhof (Schoolyard)", () => {
    describe("getSchulhofStatus", () => {
      it("fetches Schulhof status via proxy in browser context", async () => {
        const sampleBackendStatus: BackendSchulhofStatus = {
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
            headers: expect.not.objectContaining({
              Authorization: expect.anything(),
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
        const sampleBackendStatus: BackendSchulhofStatus = {
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
    });

    it("returns empty response when session has no token", async () => {
      // Clear the session cache so a fresh getSession call happens
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();
      mockedGetSession.mockResolvedValueOnce(null);

      const result = await activeService.getTrackingIndicators(["10", "20"]);

      expect(result).toEqual({ labels: [], results: {} });
    });

    it("fetches tracking indicators and returns data on success", async () => {
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();

      const mockResponse = {
        data: {
          labels: ["Hausaufgaben", "Mensa"],
          results: { "10": [true, false], "20": [false, true] },
        },
      };

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const result = await activeService.getTrackingIndicators(["10", "20"]);

      expect(result).toEqual(mockResponse.data);
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/active/tracking-indicators",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ student_ids: [10, 20] }),
        }),
      );
    });

    it("converts string student IDs to integers in request", async () => {
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { labels: [], results: {} } }),
      } as Response);

      await activeService.getTrackingIndicators(["42", "99"]);

      const callBody = JSON.parse(
        (mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string,
      ) as { student_ids: number[] };
      expect(callBody.student_ids).toEqual([42, 99]);
    });

    it("returns empty response on non-ok HTTP status", async () => {
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      } as Response);

      const result = await activeService.getTrackingIndicators(["10"]);

      expect(result).toEqual({ labels: [], results: {} });
    });

    // The route reads the session cookie; the browser holds no token (#5005).
    it("sends no Authorization header", async () => {
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();

      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { labels: [], results: {} } }),
      } as Response);

      await activeService.getTrackingIndicators(["10"]);

      const callBody = mockFetch.mock.calls.find(
        (call: unknown[]) => call[0] === "/api/active/tracking-indicators",
      ) as [string, RequestInit] | undefined;
      expect(callBody).toBeDefined();
      const headers = callBody![1].headers as Record<string, string>;
      expect(headers).not.toHaveProperty("Authorization");
      expect(headers["Content-Type"]).toBe("application/json");
    });
  });
});
