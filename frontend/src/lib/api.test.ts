/**
 * Tests for api.ts helper functions
 * These test the pure helper functions that parse, validate, and transform API data
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock dependencies before imports
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() => Promise.resolve({ user: { id: "1" } })),
}));

vi.mock("./auth-failure", () => ({
  handleAuthFailure: vi.fn(() => Promise.resolve(true)),
}));

vi.mock("./api-helpers", () => ({
  fetchWithRetry: vi.fn(),
  convertToBackendRoom: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./group-helpers", () => ({
  mapSingleGroupResponse: vi.fn(<T>(data: T): T => data),
  mapGroupResponse: vi.fn(<T>(data: T): T => data),
  prepareGroupForBackend: vi.fn(<T>(data: T): T => data),
  mapGroupsResponse: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./room-helpers", () => ({
  mapSingleRoomResponse: vi.fn(<T>(data: T): T => data),
  prepareRoomForBackend: vi.fn(<T>(data: T): T => data),
  mapRoomsResponse: vi.fn(<T>(data: T): T => data),
  mapRoomResponse: vi.fn(<T>(data: T): T => data),
}));

// Helper function to setup window mock for browser environment
function setupBrowserEnv() {
  const original = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    value: {},
    writable: true,
    configurable: true,
  });
  return () => {
    Object.defineProperty(globalThis, "window", {
      value: original,
      writable: true,
      configurable: true,
    });
  };
}

describe("api.ts helper functions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("groupService.getGroups", () => {
    it("rejects a refused list in strict mode instead of caching an empty success", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
        accessDeniedStatus: 403,
      });
      const { groupService } = await import("./api");
      const restore = setupBrowserEnv();
      try {
        await expect(
          groupService.getGroups({}, { strict: true }),
        ).rejects.toMatchObject({ status: 403 });
      } finally {
        restore();
      }
    });
    it.each([
      ["groups", 401],
      ["groups", 403],
      ["rooms", 401],
      ["rooms", 403],
    ] as const)(
      "keeps code, details and request ID of a strict %s refusal (%s)",
      async (resource, status) => {
        const { fetchWithRetry } = await import("./api-helpers");
        vi.mocked(fetchWithRetry).mockResolvedValue({
          data: null,
          response: null,
          accessDeniedStatus: status,
          accessDeniedBody: {
            status,
            code: "module.not_booked",
            detail: "module rooms is not booked",
            details: { module: "rooms" },
            instance: "req-42",
          },
        });
        const { groupService, roomService } = await import("./api");
        const restore = setupBrowserEnv();
        try {
          const read =
            resource === "groups"
              ? groupService.getGroups({}, { strict: true })
              : roomService.getRooms({}, { strict: true });
          await expect(read).rejects.toMatchObject({
            status,
            code: "module.not_booked",
            details: { module: "rooms" },
            instance: "req-42",
          });
        } finally {
          restore();
        }
      },
    );
    it("returns empty array when response is null", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
      });

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await groupService.getGroups();
        expect(result).toEqual([]);
      } finally {
        restore();
      }
    });

    it("parses wrapped API response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockGroups = [{ id: 1, name: "Group A" }];
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: mockGroups },
        response: new Response(),
      });

      const { groupService } = await import("./api");
      const { mapGroupsResponse } = await import("./group-helpers");

      const restore = setupBrowserEnv();
      try {
        await groupService.getGroups();
        expect(mapGroupsResponse).toHaveBeenCalledWith(mockGroups);
      } finally {
        restore();
      }
    });
  });

  describe("groupService.createGroup", () => {
    it("validates required name field", async () => {
      const { groupService } = await import("./api");
      const { prepareGroupForBackend } = await import("./group-helpers");
      vi.mocked(prepareGroupForBackend).mockReturnValue({ name: "" });

      await expect(groupService.createGroup({ name: "" })).rejects.toThrow(
        "Missing required field: name",
      );
    });
  });

  describe("roomService.createRoom", () => {
    it("validates required name field", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "",
          capacity: 30,
          category: "classroom",
        }),
      ).rejects.toThrow("Missing required field: name");
    });

    it("rejects non-positive capacity", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "Room 1",
          capacity: 0,
          category: "classroom",
        }),
      ).rejects.toThrow("capacity must be greater than 0");
    });

    it("validates required category field", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "Room 1",
          capacity: 30,
          category: "",
        }),
      ).rejects.toThrow("Missing required field: category");
    });
  });

  describe("roomService.getRooms", () => {
    it.each([401, 403] as const)(
      "preserves a refused shared room list (%s)",
      async (status) => {
        const { fetchWithRetry } = await import("./api-helpers");
        vi.mocked(fetchWithRetry).mockResolvedValue({
          data: null,
          response: null,
          accessDeniedStatus: status,
        });
        const { rooms } = await import("./swr/rooms");
        const restore = setupBrowserEnv();
        try {
          await expect(rooms.fetcher()).rejects.toMatchObject({ status });
        } finally {
          restore();
        }
      },
    );

    it("builds correct query params with filters", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRooms({
          building: "Building A",
          floor: 2,
          category: "classroom",
          occupied: true,
          search: "test",
          page: 1,
          pageSize: 1000,
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("building=Building+A");
        expect(callUrl).toContain("floor=2");
        expect(callUrl).toContain("category=classroom");
        expect(callUrl).toContain("occupied=true");
        expect(callUrl).toContain("search=test");
        expect(callUrl).toContain("page=1");
        expect(callUrl).toContain("page_size=1000");
      } finally {
        restore();
      }
    });

    it("handles non-array response gracefully", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: new Response(),
      });

      const { roomService } = await import("./api");
      const { mapRoomsResponse } = await import("./room-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRooms();
        expect(mapRoomsResponse).toHaveBeenCalledWith([]);
      } finally {
        restore();
      }
    });

    it("unwraps paginated room responses before mapping", async () => {
      const wrappedRooms = {
        data: [
          { id: 101, name: "Raum 101", capacity: 20 },
          { id: 102, name: "Raum 102", capacity: 18 },
        ],
        pagination: {
          current_page: 1,
          page_size: 1000,
          total_pages: 1,
          total_records: 2,
        },
      };
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: wrappedRooms,
        response: new Response(),
      });

      const { roomService } = await import("./api");
      const { mapRoomsResponse } = await import("./room-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRooms({ page: 1, pageSize: 1000 });
        expect(mapRoomsResponse).toHaveBeenCalledWith(wrappedRooms.data);
      } finally {
        restore();
      }
    });

    it("treats wrapped room responses with non-array data as invalid", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: { id: 101, name: "Not a list" } },
        response: new Response(),
      });

      const { roomService } = await import("./api");
      const { mapRoomsResponse } = await import("./room-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRooms();
        expect(mapRoomsResponse).toHaveBeenCalledWith([]);
      } finally {
        restore();
      }
    });
  });

  describe("roomService.getRoom", () => {
    it("extracts room from wrapped response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockRoom = { id: 1, name: "Room 1" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: mockRoom },
        response: new Response(),
      });

      const { roomService } = await import("./api");
      const { mapSingleRoomResponse } = await import("./room-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRoom("1");
        expect(mapSingleRoomResponse).toHaveBeenCalledWith({ data: mockRoom });
      } finally {
        restore();
      }
    });

    it("extracts room from direct response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockRoom = { id: 1, name: "Room 1" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: mockRoom,
        response: new Response(),
      });

      const { roomService } = await import("./api");
      const { convertToBackendRoom } = await import("./api-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRoom("1");
        expect(convertToBackendRoom).toHaveBeenCalledWith(mockRoom);
      } finally {
        restore();
      }
    });

    it("throws error for invalid response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: new Response(),
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(roomService.getRoom("1")).rejects.toThrow();
      } finally {
        restore();
      }
    });

    it("throws error on auth failure", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { id: 1 },
        response: null,
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(roomService.getRoom("1")).rejects.toThrow(
          "Authentication failed",
        );
      } finally {
        restore();
      }
    });

    it("preserves a forbidden response as a permission error", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
        accessDeniedStatus: 403,
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(roomService.getRoom("1")).rejects.toMatchObject({
          status: 403,
          code: "general.permission",
        });
      } finally {
        restore();
      }
    });
  });

  describe("groupService.getGroup", () => {
    it("extracts group from ApiResponse wrapper", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockGroup = { id: 1, name: "Group A" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { success: true, data: mockGroup },
        response: new Response(),
      });

      const { groupService } = await import("./api");
      const { mapGroupResponse } = await import("./group-helpers");

      const restore = setupBrowserEnv();
      try {
        await groupService.getGroup("1");
        expect(mapGroupResponse).toHaveBeenCalledWith(mockGroup);
      } finally {
        restore();
      }
    });

    it("extracts group from double-wrapped format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockGroup = { id: 1, name: "Group A" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { success: true, data: { data: mockGroup } },
        response: new Response(),
      });

      const { groupService } = await import("./api");
      const { mapGroupResponse } = await import("./group-helpers");

      const restore = setupBrowserEnv();
      try {
        await groupService.getGroup("1");
        expect(mapGroupResponse).toHaveBeenCalledWith(mockGroup);
      } finally {
        restore();
      }
    });

    it("extracts group from simple data wrapper", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockGroup = { id: 1, name: "Group A" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: mockGroup },
        response: new Response(),
      });

      const { groupService } = await import("./api");
      const { mapGroupResponse } = await import("./group-helpers");

      const restore = setupBrowserEnv();
      try {
        await groupService.getGroup("1");
        expect(mapGroupResponse).toHaveBeenCalledWith(mockGroup);
      } finally {
        restore();
      }
    });

    it("extracts group from direct format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockGroup = { id: 1, name: "Group A" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: mockGroup,
        response: new Response(),
      });

      const { groupService } = await import("./api");
      const { mapGroupResponse } = await import("./group-helpers");

      const restore = setupBrowserEnv();
      try {
        await groupService.getGroup("1");
        expect(mapGroupResponse).toHaveBeenCalledWith(mockGroup);
      } finally {
        restore();
      }
    });

    it("throws error for invalid response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: new Response(),
      });

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.getGroup("1")).rejects.toThrow();
      } finally {
        restore();
      }
    });

    it("throws error when no group data in response", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { foo: "bar" },
        response: new Response(),
      });

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.getGroup("1")).rejects.toThrow(
          "No group data in response",
        );
      } finally {
        restore();
      }
    });

    it("preserves a forbidden response as a permission error", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
        accessDeniedStatus: 403,
      });

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.getGroup("1")).rejects.toMatchObject({
          status: 403,
          code: "general.permission",
        });
      } finally {
        restore();
      }
    });
  });

  describe("groupService.deleteGroup error handling", () => {
    it("extracts detailed error from JSON response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () =>
          Promise.resolve('{"detail":"cannot delete group with students"}'),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.deleteGroup("1")).rejects.toThrow(
          "API error: undefined",
        );
      } finally {
        restore();
      }
    });

    it("classifies a plain-text failure by its status", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        text: () => Promise.resolve("Error: cannot delete group with students"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.deleteGroup("1")).rejects.toMatchObject({
          status: 409,
          code: "general.business_rejection",
        });
      } finally {
        restore();
      }
    });
  });

  describe("roomService.updateRoom", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ id: 1, name: "Updated Room" }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await roomService.updateRoom("1", {
          name: "Updated Room",
          capacity: 30,
          category: "classroom",
        });
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/rooms/1"),
          expect.objectContaining({ method: "PUT" }),
        );
      } finally {
        restore();
      }
    });

    it("throws error on API failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        text: () => Promise.resolve("Bad Request"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          roomService.updateRoom("1", {
            name: "Room",
            capacity: 30,
            category: "classroom",
          }),
        ).rejects.toThrow();
      } finally {
        restore();
      }
    });
  });

  describe("groupService.updateGroup", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ id: 1, name: "Updated Group" }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await groupService.updateGroup("1", { name: "Updated Group" });
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/1"),
          expect.objectContaining({ method: "PUT" }),
        );
      } finally {
        restore();
      }
    });
  });

  describe("roomService.deleteRoom", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await roomService.deleteRoom("456");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/rooms/456"),
          expect.objectContaining({ method: "DELETE" }),
        );
      } finally {
        restore();
      }
    });
  });

  describe("groupService.addSupervisor", () => {
    it("adds supervisor to group successfully", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await groupService.addSupervisor("123", "456");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/123/supervisors"),
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({ supervisor_id: 456 }),
          }),
        );
      } finally {
        restore();
      }
    });

    it("throws error on API failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        text: () => Promise.resolve("Bad Request"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.addSupervisor("123", "456")).rejects.toThrow(
          "API error: 400",
        );
      } finally {
        restore();
      }
    });
  });

  describe("groupService.removeSupervisor", () => {
    it("removes supervisor from group successfully", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await groupService.removeSupervisor("123", "456");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/123/supervisors/456"),
          expect.objectContaining({ method: "DELETE" }),
        );
      } finally {
        restore();
      }
    });

    it("throws error on API failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        text: () => Promise.resolve("Not Found"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          groupService.removeSupervisor("123", "456"),
        ).rejects.toThrow("API error: 404");
      } finally {
        restore();
      }
    });
  });

  describe("groupService.setRepresentative", () => {
    it("sets representative for group successfully", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await groupService.setRepresentative("123", "789");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/123/representative"),
          expect.objectContaining({
            method: "PUT",
            body: JSON.stringify({ representative_id: 789 }),
          }),
        );
      } finally {
        restore();
      }
    });

    it("throws error on API failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        text: () => Promise.resolve("Forbidden"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          groupService.setRepresentative("123", "789"),
        ).rejects.toThrow("API error: 403");
      } finally {
        restore();
      }
    });
  });

  describe("roomService.getRoomsByCategory", () => {
    it("fetches rooms grouped by category successfully", async () => {
      const mockData = {
        classroom: [{ id: 1, name: "Room A" }],
        lab: [{ id: 2, name: "Lab B" }],
      };
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockData),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await roomService.getRoomsByCategory();
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/rooms/by-category"),
          expect.any(Object),
        );
        expect(result).toBeDefined();
        expect(Object.keys(result)).toContain("classroom");
        expect(Object.keys(result)).toContain("lab");
      } finally {
        restore();
      }
    });

    it("transforms each category's room array", async () => {
      const mockData = {
        classroom: [
          { id: 1, name: "Room 1", capacity: 30 },
          { id: 2, name: "Room 2", capacity: 25 },
        ],
      };
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockData),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");
      const { mapRoomsResponse } = await import("./room-helpers");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRoomsByCategory();
        expect(mapRoomsResponse).toHaveBeenCalledWith(mockData.classroom);
      } finally {
        restore();
      }
    });

    it("throws error on API failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Server Error"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { id: "1" },
      } as never);

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(roomService.getRoomsByCategory()).rejects.toThrow();
      } finally {
        restore();
      }
    });
  });

  describe("query parameter builders", () => {
    it("buildRoomQueryParams includes all filters", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await roomService.getRooms({
          building: "Main Building",
          floor: 3,
          category: "lab",
          occupied: false,
          search: "chemistry",
          page: 1,
          pageSize: 1000,
        });

        const url = vi.mocked(fetchWithRetry).mock.calls[0]?.[0] ?? "";
        expect(url).toContain("building=Main+Building");
        expect(url).toContain("floor=3");
        expect(url).toContain("category=lab");
        expect(url).toContain("occupied=false");
        expect(url).toContain("search=chemistry");
        expect(url).toContain("page=1");
        expect(url).toContain("page_size=1000");
      } finally {
        restore();
      }
    });
  });

  describe("response parsing edge cases", () => {
    it("parseRoomsResponse returns empty array for null", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: new Response(),
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await roomService.getRooms();
        expect(result).toEqual([]);
      } finally {
        restore();
      }
    });

    it("extractBackendRoom throws for invalid response", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { invalid: "structure" },
        response: new Response(),
      });

      const { roomService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(roomService.getRoom("1")).rejects.toThrow(
          "Unexpected room response format",
        );
      } finally {
        restore();
      }
    });
  });

  describe("validation functions", () => {
    it("validateRoomForCreation checks all required fields", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "",
          capacity: 0,
          category: "",
        }),
      ).rejects.toThrow("Missing required field: name");
    });

    it("validateRoomForCreation checks capacity > 0", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "Test Room",
          capacity: -5,
          category: "classroom",
        }),
      ).rejects.toThrow("capacity must be greater than 0");
    });

    it("validateRoomForCreation allows omitted capacity", async () => {
      const { roomService } = await import("./api");
      const restore = setupBrowserEnv();
      const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response(
          JSON.stringify({ data: { id: 1, name: "Unlimited Room" } }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

      try {
        await expect(
          roomService.createRoom({
            name: "Unlimited Room",
            category: "classroom",
          }),
        ).resolves.toBeDefined();
      } finally {
        fetchMock.mockRestore();
        restore();
      }
    });
  });
});
