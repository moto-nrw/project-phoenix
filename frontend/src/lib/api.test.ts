/**
 * Tests for api.ts helper functions
 * These test the pure helper functions that parse, validate, and transform API data
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock dependencies before imports
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() => Promise.resolve({ user: { token: "test-token" } })),
}));

vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

vi.mock("./auth-api", () => ({
  handleAuthFailure: vi.fn(() => Promise.resolve(true)),
}));

vi.mock("./api-helpers", () => ({
  fetchWithRetry: vi.fn(),
  convertToBackendRoom: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./student-helpers", () => ({
  mapSingleStudentResponse: vi.fn(<T>(data: T): T => data),
  mapStudentsResponse: vi.fn(<T>(data: T): T => data),
  mapStudentDetailResponse: vi.fn(<T>(data: T): T => data),
  prepareStudentForBackend: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./group-helpers", () => ({
  mapSingleGroupResponse: vi.fn(<T>(data: T): T => data),
  mapGroupResponse: vi.fn(<T>(data: T): T => data),
  prepareGroupForBackend: vi.fn(<T>(data: T): T => data),
  mapSingleCombinedGroupResponse: vi.fn(<T>(data: T): T => data),
  prepareCombinedGroupForBackend: vi.fn(<T>(data: T): T => data),
  mapGroupsResponse: vi.fn(<T>(data: T): T => data),
  mapCombinedGroupsResponse: vi.fn(<T>(data: T): T => data),
  mapCombinedGroupResponse: vi.fn(<T>(data: T): T => data),
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

  describe("studentService.getStudents", () => {
    it("builds correct query params with filters", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {
          data: [],
          pagination: {
            current_page: 1,
            page_size: 50,
            total_pages: 1,
            total_records: 0,
          },
        },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          search: "test",
          inHouse: true,
          groupId: "123",
          page: 2,
          pageSize: 25,
          token: "test-token",
        });

        expect(fetchWithRetry).toHaveBeenCalled();
        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("search=test");
        expect(callUrl).toContain("in_house=true");
        expect(callUrl).toContain("group_id=123");
        expect(callUrl).toContain("page=2");
        expect(callUrl).toContain("page_size=25");
      } finally {
        restore();
      }
    });

    it("handles wrapped API response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockStudents = [
        { id: 1, first_name: "Test", last_name: "Student" },
      ];
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {
          success: true,
          data: {
            data: mockStudents,
            pagination: {
              current_page: 1,
              page_size: 50,
              total_pages: 1,
              total_records: 1,
            },
          },
        },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudents({
          token: "test-token",
        });
        expect(result.students).toBeDefined();
      } finally {
        restore();
      }
    });

    it("handles direct paginated response format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockStudents = [
        { id: 1, first_name: "Test", last_name: "Student" },
      ];
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {
          data: mockStudents,
          pagination: {
            current_page: 1,
            page_size: 50,
            total_pages: 1,
            total_records: 1,
          },
        },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudents({
          token: "test-token",
        });
        expect(result.students).toBeDefined();
      } finally {
        restore();
      }
    });

    it("handles legacy array format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockStudents = [
        { id: 1, first_name: "Test", last_name: "Student" },
      ];
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: mockStudents,
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudents({
          token: "test-token",
        });
        expect(result.students).toEqual(mockStudents);
      } finally {
        restore();
      }
    });

    it("throws error when authentication fails", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          studentService.getStudents({ token: "test-token" }),
        ).rejects.toThrow("Authentication failed");
      } finally {
        restore();
      }
    });
  });

  describe("studentService.createStudent", () => {
    it("validates required fields - first_name", async () => {
      const { studentService } = await import("./api");

      await expect(
        studentService.createStudent({
          first_name: "",
          second_name: "Student",
          school_class: "1a",
          name: "Test Student",
          current_location: "",
        }),
      ).rejects.toThrow("First name is required");
    });

    it("validates required fields - second_name", async () => {
      const { studentService } = await import("./api");

      await expect(
        studentService.createStudent({
          first_name: "Test",
          second_name: "",
          school_class: "1a",
          name: "Test",
          current_location: "",
        }),
      ).rejects.toThrow("Last name is required");
    });

    it("validates required fields - school_class", async () => {
      const { studentService } = await import("./api");

      await expect(
        studentService.createStudent({
          first_name: "Test",
          second_name: "Student",
          school_class: "",
          name: "Test Student",
          current_location: "",
        }),
      ).rejects.toThrow("School class is required");
    });
  });

  describe("groupService.getGroups", () => {
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

    it("validates required capacity field", async () => {
      const { roomService } = await import("./api");

      await expect(
        roomService.createRoom({
          name: "Room 1",
          capacity: 0,
          category: "classroom",
        }),
      ).rejects.toThrow(
        "Missing required field: capacity must be greater than 0",
      );
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
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("building=Building+A");
        expect(callUrl).toContain("floor=2");
        expect(callUrl).toContain("category=classroom");
        expect(callUrl).toContain("occupied=true");
        expect(callUrl).toContain("search=test");
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
  });

  describe("combinedGroupService.createCombinedGroup", () => {
    it("validates required name field", async () => {
      const { combinedGroupService } = await import("./api");
      const { prepareCombinedGroupForBackend } =
        await import("./group-helpers");
      vi.mocked(prepareCombinedGroupForBackend).mockReturnValue({
        name: "",
        access_policy: "all",
      });

      // Use type assertion to test validation with minimal data
      await expect(
        combinedGroupService.createCombinedGroup({
          name: "",
          access_policy: "all",
          is_active: true,
        }),
      ).rejects.toThrow("Missing required field: name");
    });

    it("validates required access_policy field", async () => {
      const { combinedGroupService } = await import("./api");
      const { prepareCombinedGroupForBackend } =
        await import("./group-helpers");
      vi.mocked(prepareCombinedGroupForBackend).mockReturnValue({
        name: "Test",
        access_policy: "" as "all", // Empty string to test validation
      });

      await expect(
        combinedGroupService.createCombinedGroup({
          name: "Test",
          access_policy: "" as "all", // Empty string to test validation
          is_active: true,
        }),
      ).rejects.toThrow("Missing required field: access_policy");
    });
  });

  describe("groupService.deleteGroup error handling", () => {
    it("extracts detailed error from JSON response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () =>
          Promise.resolve('{"error":"cannot delete group with students"}'),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.deleteGroup("1")).rejects.toThrow(
          "cannot delete group with students",
        );
      } finally {
        restore();
      }
    });

    it("extracts error from known patterns", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve("Error: cannot delete group with students"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.deleteGroup("1")).rejects.toThrow(
          "cannot delete group with students",
        );
      } finally {
        restore();
      }
    });
  });

  describe("parseStudentsPaginatedResponse edge cases", () => {
    it("handles empty object response", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {},
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudents({
          token: "test-token",
        });
        expect(result.students).toEqual([]);
      } finally {
        restore();
      }
    });

    it("handles wrapped response with non-array data", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { success: true, data: { notAnArray: true } },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudents({
          token: "test-token",
        });
        expect(result.students).toEqual([]);
      } finally {
        restore();
      }
    });
  });

  describe("buildStudentQueryParams", () => {
    it("builds params with all filter options", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          search: "John",
          inHouse: false,
          groupId: "456",
          page: 3,
          pageSize: 100,
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("search=John");
        expect(callUrl).toContain("in_house=false");
        expect(callUrl).toContain("group_id=456");
        expect(callUrl).toContain("page=3");
        expect(callUrl).toContain("page_size=100");
      } finally {
        restore();
      }
    });

    it("omits undefined filter values", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          search: "Test",
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("search=Test");
        expect(callUrl).not.toContain("in_house");
        expect(callUrl).not.toContain("group_id");
        expect(callUrl).not.toContain("page=");
        expect(callUrl).not.toContain("page_size");
      } finally {
        restore();
      }
    });
  });

  describe("studentService.getStudent", () => {
    it("returns student from successful response", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockStudent = { id: 1, first_name: "Test", last_name: "Student" };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: mockStudent },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudent("1");
        expect(result).toBeDefined();
        expect(fetchWithRetry).toHaveBeenCalledWith(
          expect.stringContaining("/api/students/1"),
          expect.any(String),
          expect.any(Object),
        );
      } finally {
        restore();
      }
    });

    it("throws error on auth failure", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(studentService.getStudent("1")).rejects.toThrow(
          "Authentication failed",
        );
      } finally {
        restore();
      }
    });
  });

  describe("studentService.updateStudent", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: 1, first_name: "Updated" } }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", {
          first_name: "Updated",
          second_name: "Student",
          school_class: "1a",
          name: "Updated Student",
          current_location: "",
        });
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/students/1"),
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
        text: () => Promise.resolve('{"error":"Validation failed"}'),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          studentService.updateStudent("1", {
            first_name: "Test",
            second_name: "Student",
            school_class: "1a",
            name: "Test Student",
            current_location: "",
          }),
        ).rejects.toThrow("Validation failed");
      } finally {
        restore();
      }
    });
  });

  describe("combinedGroupService.getCombinedGroups", () => {
    it("calls fetch with correct URL", async () => {
      const mockGroups = [{ id: 1, name: "Combined A" }];
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: mockGroups }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await combinedGroupService.getCombinedGroups();
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/combined"),
          expect.any(Object),
        );
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
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(combinedGroupService.getCombinedGroups()).rejects.toThrow(
          "API error",
        );
      } finally {
        restore();
      }
    });
  });

  describe("combinedGroupService.getCombinedGroup", () => {
    it("calls fetch with correct URL", async () => {
      const mockGroup = { id: 1, name: "Combined A" };
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: mockGroup }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await combinedGroupService.getCombinedGroup("1");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/combined/1"),
          expect.any(Object),
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
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          combinedGroupService.getCombinedGroup("1"),
        ).rejects.toThrow("API error");
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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

  describe("combinedGroupService.updateCombinedGroup", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ id: 1, name: "Updated Combined" }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await combinedGroupService.updateCombinedGroup("1", {
          name: "Updated Combined",
          access_policy: "all",
          is_active: true,
        });
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/combined/1"),
          expect.objectContaining({ method: "PUT" }),
        );
      } finally {
        restore();
      }
    });
  });

  describe("studentService.deleteStudent", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.deleteStudent("123");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/students/123"),
          expect.objectContaining({ method: "DELETE" }),
        );
      } finally {
        restore();
      }
    });

    it("throws error on delete failure", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve("Delete failed"),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(studentService.deleteStudent("123")).rejects.toThrow();
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
        user: { token: "test-token" },
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

  describe("combinedGroupService.deleteCombinedGroup", () => {
    it("calls fetch with correct URL and method", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { combinedGroupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await combinedGroupService.deleteCombinedGroup("789");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/combined/789"),
          expect.objectContaining({ method: "DELETE" }),
        );
      } finally {
        restore();
      }
    });
  });
});
