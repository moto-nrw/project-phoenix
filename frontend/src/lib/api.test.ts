/**
 * Tests for api.ts helper functions
 * These test the pure helper functions that parse, validate, and transform API data
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock dependencies before imports
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() => Promise.resolve({ user: { token: "test-token" } })),
}));

vi.mock("./auth-failure", () => ({
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

function setupServerEnv() {
  const original = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    value: undefined,
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
          locationState: "transit",
          dayStatus: "comes_today",
          page: 2,
          pageSize: 25,
          token: "test-token",
        });

        expect(fetchWithRetry).toHaveBeenCalled();
        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("search=test");
        expect(callUrl).toContain("in_house=true");
        expect(callUrl).toContain("group_id=123");
        expect(callUrl).toContain("location_state=transit");
        expect(callUrl).toContain("day_status=comes_today");
        expect(callUrl).toContain("page=2");
        expect(callUrl).toContain("page_size=25");
      } finally {
        restore();
      }
    });

    it("includes room filter and falls back to the session token when no token is provided", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const { getSession } = await import("next-auth/react");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "session-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          roomId: "room-7",
        });

        expect(getSession).toHaveBeenCalled();
        expect(fetchWithRetry).toHaveBeenCalledWith(
          expect.stringContaining("room_id=room-7"),
          "session-token",
          expect.any(Object),
        );
      } finally {
        restore();
      }
    });

    it("uses axios and maps paginated backend students on the server", async () => {
      const apiTransport = (await import("./api-transport")).default;
      const getSpy = vi.spyOn(apiTransport, "get").mockResolvedValue({
        data: {
          data: [{ id: 1, first_name: "Server", last_name: "Student" }],
          pagination: {
            current_page: 2,
            page_size: 10,
            total_pages: 3,
            total_records: 21,
          },
        },
      } as never);
      const { mapStudentsResponse } = await import("./student-helpers");
      const { studentService } = await import("./api");

      const restore = setupServerEnv();
      try {
        const result = await studentService.getStudents({
          search: "Server",
          page: 2,
          pageSize: 10,
        });

        expect(getSpy).toHaveBeenCalledWith(
          expect.stringContaining(
            "/api/students?search=Server&page=2&page_size=10",
          ),
          { params: expect.any(URLSearchParams) },
        );
        expect(mapStudentsResponse).toHaveBeenCalledWith([
          { id: 1, first_name: "Server", last_name: "Student" },
        ]);
        expect(result.pagination).toEqual({
          current_page: 2,
          page_size: 10,
          total_pages: 3,
          total_records: 21,
        });
      } finally {
        getSpy.mockRestore();
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

    it("logs 429 failures as rate-limit warnings", async () => {
      const consoleWarn = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockRejectedValueOnce(
        new Error("API error: 429"),
      );

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          studentService.getStudents({ token: "test-token" }),
        ).rejects.toThrow("Error fetching students: API error: 429");

        expect(consoleWarn).toHaveBeenCalledWith("api operation rate limited", {
          context: "Error fetching students",
          error: "API error: 429",
          status: 429,
          rate_limited: true,
        });
      } finally {
        consoleWarn.mockRestore();
        restore();
      }
    });

    it("logs Axios-style 429 responses as rate-limit warnings", async () => {
      const consoleWarn = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});
      const { fetchWithRetry } = await import("./api-helpers");
      const axiosError = new Error(
        "Request failed with status code 429",
      ) as Error & {
        response: { status: number };
      };
      axiosError.response = { status: 429 };
      vi.mocked(fetchWithRetry).mockRejectedValueOnce(axiosError);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          studentService.getStudents({ token: "test-token" }),
        ).rejects.toThrow(
          "Error fetching students: Request failed with status code 429",
        );

        expect(consoleWarn).toHaveBeenCalledWith("api operation rate limited", {
          context: "Error fetching students",
          error: "Request failed with status code 429",
          status: 429,
          rate_limited: true,
        });
      } finally {
        consoleWarn.mockRestore();
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

  describe("studentService.getSchoolClasses", () => {
    it("fetches distinct class options without sampling the students page", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {
          success: true,
          data: ["1a", "2b"],
        },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getSchoolClasses({
          token: "test-token",
        });

        expect(result).toEqual(["1a", "2b"]);
        expect(fetchWithRetry).toHaveBeenCalledWith(
          "/api/students/school-classes",
          "test-token",
          expect.any(Object),
        );
        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).not.toContain("page_size=1000");
      } finally {
        restore();
      }
    });

    it("trims classes and removes blanks or non-string values", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: {
          success: true,
          data: [" 1a ", "", "   ", "2b", 42, null],
        },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getSchoolClasses({
          token: "test-token",
        });

        expect(result).toEqual(["1a", "2b"]);
      } finally {
        restore();
      }
    });

    it("parses direct arrays and invalid payloads", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        vi.mocked(fetchWithRetry).mockResolvedValueOnce({
          data: ["3c", " 4d "],
          response: new Response(),
        });
        await expect(
          studentService.getSchoolClasses({ token: "test-token" }),
        ).resolves.toEqual(["3c", "4d"]);

        vi.mocked(fetchWithRetry).mockResolvedValueOnce({
          data: { success: true, data: { not: "an array" } },
          response: new Response(),
        });
        await expect(
          studentService.getSchoolClasses({ token: "test-token" }),
        ).resolves.toEqual([]);
      } finally {
        restore();
      }
    });

    it("uses getSession when no token is provided and fails on null data", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "session-token" },
      } as never);
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: null,
        response: null,
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(studentService.getSchoolClasses()).rejects.toThrow(
          "Error fetching school classes: Authentication failed",
        );
        expect(fetchWithRetry).toHaveBeenCalledWith(
          "/api/students/school-classes",
          "session-token",
          expect.any(Object),
        );
      } finally {
        restore();
      }
    });

    it("uses axios and parses wrapped school classes on the server", async () => {
      const apiTransport = (await import("./api-transport")).default;
      const getSpy = vi.spyOn(apiTransport, "get").mockResolvedValue({
        data: {
          success: true,
          data: [" 5e ", "6f"],
        },
      } as never);
      const { studentService } = await import("./api");

      const restore = setupServerEnv();
      try {
        const result = await studentService.getSchoolClasses();

        expect(result).toEqual(["5e", "6f"]);
        expect(getSpy).toHaveBeenCalledWith(
          expect.stringContaining("/api/students/school-classes"),
        );
      } finally {
        getSpy.mockRestore();
        restore();
      }
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
          schoolClass: "3a",
          page: 3,
          pageSize: 100,
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("search=John");
        expect(callUrl).toContain("in_house=false");
        expect(callUrl).toContain("group_id=456");
        expect(callUrl).toContain("school_class=3a");
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

    it("appends include_pickup_times=true when includePickupTimes is true", async () => {
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
          includePickupTimes: true,
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("include_pickup_times=true");
      } finally {
        restore();
      }
    });

    it("omits include_pickup_times when includePickupTimes is falsy", async () => {
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
        expect(callUrl).not.toContain("include_pickup_times");
      } finally {
        restore();
      }
    });

    it("appends include_arrival_times=true when includeArrivalTimes is true", async () => {
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
          includeArrivalTimes: true,
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("include_arrival_times=true");
      } finally {
        restore();
      }
    });

    it("appends the administrative filters (bus, photo consent, pickup rule)", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: [],
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          bus: "yes",
          photoConsent: "no",
          pickupStatus: "pickedUp",
          token: "test-token",
        });

        const callUrl = vi.mocked(fetchWithRetry).mock.calls[0]?.[0];
        expect(callUrl).toContain("bus=yes");
        expect(callUrl).toContain("photo_consent=no");
        expect(callUrl).toContain("pickup_status=pickedUp");
      } finally {
        restore();
      }
    });

    it("omits the administrative filters when they are not set", async () => {
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
        expect(callUrl).not.toContain("bus=");
        expect(callUrl).not.toContain("photo_consent=");
        expect(callUrl).not.toContain("pickup_status=");
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

    // The "läuft mit" links are stored in their own table and are absent from
    // this response, and the backend also trims links a shrunken departure plan
    // no longer allows — so a departure-plan update has to tell every mounted
    // companion view to refetch, or it renders the pre-save list (#1694).
    it("announces a companion change after a departure plan update", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: 1, first_name: "Updated" } }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");
      const { subscribeStudentCompanionsChanged } =
        await import("./student-companion-api");

      const listener = vi.fn();
      const unsubscribe = subscribeStudentCompanionsChanged(listener);
      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", {
          first_name: "Updated",
          departure_days: { mon: "accompanied" },
        });
        expect(listener).toHaveBeenCalledTimes(1);
      } finally {
        unsubscribe();
        restore();
      }
    });

    // The announcement is not a free refetch: an open Laufgemeinschaft form
    // answers it by discarding its draft, or by blocking the save while the
    // draft is dirty. A payload that cannot touch a link — a sick flag, a
    // rename, a photo — must therefore stay silent, or an unrelated action in
    // the same tab costs the user the edit they are in the middle of (#1694).
    it("stays silent for an update that cannot change companions", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: 1, first_name: "Updated" } }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");
      const { subscribeStudentCompanionsChanged } =
        await import("./student-companion-api");

      const listener = vi.fn();
      const unsubscribe = subscribeStudentCompanionsChanged(listener);
      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", { sick: true });
        await studentService.updateStudent("1", { first_name: "Updated" });
        expect(listener).not.toHaveBeenCalled();
      } finally {
        unsubscribe();
        restore();
      }
    });

    // An edited list travels as `companions` and replaces the stored one, so it
    // is the one payload that certainly changed the links.
    it("announces a companion change when a list is submitted", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: 1, first_name: "Updated" } }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");
      const { subscribeStudentCompanionsChanged } =
        await import("./student-companion-api");

      const listener = vi.fn();
      const unsubscribe = subscribeStudentCompanionsChanged(listener);
      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", { companions: [] });
        expect(listener).toHaveBeenCalledTimes(1);
      } finally {
        unsubscribe();
        restore();
      }
    });

    // The backend answers the question itself, and its answer beats the shape
    // of the request: the forms resubmit the whole departure plan on every
    // save, so a pure name or address edit looks exactly like a plan change
    // from here. Announcing one costs every other open Laufgemeinschaft editor
    // its draft — for a write the backend already decided changed nothing.
    it("stays silent when the response reports no companion change", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { id: 1, first_name: "Updated", companions_changed: false },
          }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");
      const { subscribeStudentCompanionsChanged } =
        await import("./student-companion-api");

      const listener = vi.fn();
      const unsubscribe = subscribeStudentCompanionsChanged(listener);
      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", {
          first_name: "Updated",
          departure_days: { mon: "accompanied" },
        });
        expect(listener).not.toHaveBeenCalled();
      } finally {
        unsubscribe();
        restore();
      }
    });

    // The other direction: the backend trims links off weekdays a narrowed plan
    // no longer allows, so a write can change them without the payload naming a
    // single link. Its verdict is what the mounted views have to hear.
    it("announces when the response reports a companion change", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { id: 1, first_name: "Updated", companions_changed: true },
          }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");
      const { subscribeStudentCompanionsChanged } =
        await import("./student-companion-api");

      const listener = vi.fn();
      const unsubscribe = subscribeStudentCompanionsChanged(listener);
      const restore = setupBrowserEnv();
      try {
        await studentService.updateStudent("1", {
          departure_days: { mon: "alone" },
        });
        expect(listener).toHaveBeenCalledTimes(1);
      } finally {
        unsubscribe();
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

    // The student PUT answers 409 for several unrelated reasons. Only the one
    // carrying a conflict list is the companion question the caller can answer
    // with extend_companion_plans; typing another 409 as that error would hand
    // the user an empty, unanswerable confirmation instead of the real message.
    it("types only a 409 carrying conflicts as the companion question", async () => {
      const respondWith = async (body: string) => {
        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status: 409,
          text: () => Promise.resolve(body),
        });
        const { getSession } = await import("next-auth/react");
        vi.mocked(getSession).mockResolvedValue({
          user: { token: "test-token" },
        } as never);
        const { studentService } = await import("./api");
        const restore = setupBrowserEnv();
        try {
          return await studentService
            .updateStudent("1", {
              first_name: "Test",
              second_name: "Student",
              school_class: "1a",
              name: "Test Student",
              current_location: "",
            })
            .then(() => null)
            .catch((err: unknown) => err);
        } finally {
          restore();
        }
      };

      const { CompanionPlanConflictError } = await import("./api");

      const companionConflict = await respondWith(
        JSON.stringify({
          error:
            "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
          conflicts: [{ student_id: 42, weekdays: ["mon"] }],
        }),
      );
      expect(companionConflict).toBeInstanceOf(CompanionPlanConflictError);

      // A different 409 (sick + excused) keeps its own contract.
      const sickExcused = await respondWith(
        JSON.stringify({
          error: "a student cannot be both sick and excused at the same time",
          code: "SICK_EXCUSED_CONFLICT",
        }),
      );
      expect(sickExcused).not.toBeInstanceOf(CompanionPlanConflictError);
      expect((sickExcused as Error).message).toContain(
        "cannot be both sick and excused",
      );
    });

    // The stranded-companion refusal is an instruction, not a failure: it names
    // the child whose Heimweg has to be filled in before this link can go. It
    // must survive the trip to the form intact, and only it — every other 400
    // keeps the generic handling.
    it("types only the coded 400 as the stranded-companion refusal", async () => {
      const respondWith = async (body: string) => {
        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status: 400,
          text: () => Promise.resolve(body),
        });
        const { getSession } = await import("next-auth/react");
        vi.mocked(getSession).mockResolvedValue({
          user: { token: "test-token" },
        } as never);
        const { studentService } = await import("./api");
        const restore = setupBrowserEnv();
        try {
          return await studentService
            .updateStudent("1", {
              first_name: "Test",
              second_name: "Student",
              school_class: "1a",
              name: "Test Student",
              current_location: "",
            })
            .then(() => null)
            .catch((err: unknown) => err);
        } finally {
          restore();
        }
      };

      const { CompanionDepartureRefusedError, isCompanionDepartureRefusal } =
        await import("./api");

      const stranded = await respondWith(
        JSON.stringify({
          status: "error",
          error:
            "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
          code: "companion_would_lose_departure",
        }),
      );
      expect(stranded).toBeInstanceOf(CompanionDepartureRefusedError);
      expect(isCompanionDepartureRefusal(stranded)).toBe(true);
      // The German instruction reaches the form verbatim, without the
      // "API error 400:" prefix the generic branch would add.
      expect((stranded as Error).message).toBe(
        "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
      );

      const otherBadRequest = await respondWith(
        JSON.stringify({ status: "error", error: "invalid weekday" }),
      );
      expect(otherBadRequest).not.toBeInstanceOf(
        CompanionDepartureRefusedError,
      );
      expect(isCompanionDepartureRefusal(otherBadRequest)).toBe(false);
    });

    // The stale-list 409 is the third 409 on this endpoint and the only one
    // whose retry must NOT re-send the same list: doing so would delete the
    // change the refusal is protecting. It therefore needs its own type, and
    // must not be mistaken for the answerable companion question.
    it("types the stale-list 409 apart from the companion question", async () => {
      const respondWith = async (body: string) => {
        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status: 409,
          text: () => Promise.resolve(body),
        });
        const { getSession } = await import("next-auth/react");
        vi.mocked(getSession).mockResolvedValue({
          user: { token: "test-token" },
        } as never);
        const { studentService } = await import("./api");
        const restore = setupBrowserEnv();
        try {
          return await studentService
            .updateStudent("1", {
              first_name: "Test",
              second_name: "Student",
              school_class: "1a",
              name: "Test Student",
              current_location: "",
            })
            .then(() => null)
            .catch((err: unknown) => err);
        } finally {
          restore();
        }
      };

      const {
        CompanionsChangedError,
        CompanionPlanConflictError,
        isCompanionsChanged,
        companionsChangedMessage,
      } = await import("./api");

      const stale = await respondWith(
        JSON.stringify({
          status: "error",
          error:
            "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.",
          code: "companions_changed",
        }),
      );
      expect(stale).toBeInstanceOf(CompanionsChangedError);
      expect(stale).not.toBeInstanceOf(CompanionPlanConflictError);
      expect(isCompanionsChanged(stale)).toBe(true);
      expect(companionsChangedMessage(stale)).toBe(
        "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.",
      );
    });

    // The body-fragment predicate the CRUD-service path uses must make the same
    // distinction: a stale-list 409 offers nothing to confirm, so treating it as
    // the question would ask the user to widen a child's plan for a conflict
    // that was never reported.
    it("does not read the stale-list body as the companion question", async () => {
      const { isCompanionPlanConflictBody, isCompanionsChangedBody } =
        await import("./api");

      const body = JSON.stringify({
        error:
          "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert.",
        code: "companions_changed",
      });
      expect(isCompanionsChangedBody(body)).toBe(true);
      expect(isCompanionPlanConflictBody(body)).toBe(false);
    });

    // The student PUT proxy writes the privacy consent first, against a
    // different backend transaction. A failure after that write is a partial
    // success, and the form has to say so rather than report a blanket failure
    // the user answers by abandoning the retry.
    it("reads the partial-success marker off the failed save", async () => {
      const {
        isPrivacyConsentSaved,
        withPrivacyConsentSavedNotice,
        PRIVACY_CONSENT_SAVED_NOTICE,
      } = await import("./api");

      const marked = Object.assign(new Error("Fehler"), {
        body: JSON.stringify({
          error: "Fehler",
          details: { privacy_consent_saved: true },
        }),
      });
      const plain = Object.assign(new Error("Fehler"), {
        body: JSON.stringify({ error: "Fehler" }),
      });

      expect(isPrivacyConsentSaved(marked)).toBe(true);
      expect(isPrivacyConsentSaved(plain)).toBe(false);
      expect(
        withPrivacyConsentSavedNotice(marked, "Fehler beim Speichern."),
      ).toBe(`${PRIVACY_CONSENT_SAVED_NOTICE} Fehler beim Speichern.`);
      expect(
        withPrivacyConsentSavedNotice(plain, "Fehler beim Speichern."),
      ).toBe("Fehler beim Speichern.");
    });

    // The marker also has to survive the wrapping where the body is lost and
    // only the message carries the JSON.
    it("reads the marker out of the message when no body is attached", async () => {
      const { isPrivacyConsentSaved } = await import("./api");

      const fromMessage = new Error(
        `API error (409): ${JSON.stringify({
          error: "Konflikt",
          details: { privacy_consent_saved: true },
        })}`,
      );
      expect(isPrivacyConsentSaved(fromMessage)).toBe(true);
      expect(isPrivacyConsentSaved(new Error("API error (500): boom"))).toBe(
        false,
      );
    });

    // The typed companion refusals reduce the response to a display message, so
    // they have to carry the untouched body along — the marker lives in
    // `details`, and the proxy stamps it onto exactly these refusals when the
    // consent half of the same request already committed.
    it("keeps the marker on the typed companion refusals", async () => {
      const respondWith = async (status: number, body: string) => {
        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status,
          text: () => Promise.resolve(body),
        });
        const { getSession } = await import("next-auth/react");
        vi.mocked(getSession).mockResolvedValue({
          user: { token: "test-token" },
        } as never);
        const { studentService } = await import("./api");
        const restore = setupBrowserEnv();
        try {
          return await studentService
            .updateStudent("1", {
              first_name: "Test",
              second_name: "Student",
              school_class: "1a",
              name: "Test Student",
              current_location: "",
            })
            .then(() => null)
            .catch((err: unknown) => err);
        } finally {
          restore();
        }
      };

      const {
        isPrivacyConsentSaved,
        CompanionPlanConflictError,
        CompanionsChangedError,
        CompanionDepartureRefusedError,
      } = await import("./api");
      const details = { privacy_consent_saved: true };

      const conflict = await respondWith(
        409,
        JSON.stringify({
          error: "Ein verknüpftes Kind läuft an diesem Tag nicht allein.",
          conflicts: [{ student_id: 7, weekdays: ["mon"] }],
          details,
        }),
      );
      expect(conflict).toBeInstanceOf(CompanionPlanConflictError);
      expect(isPrivacyConsentSaved(conflict)).toBe(true);

      const stale = await respondWith(
        409,
        JSON.stringify({
          error: "Die Laufgemeinschaft wurde zwischenzeitlich geändert.",
          code: "companions_changed",
          details,
        }),
      );
      expect(stale).toBeInstanceOf(CompanionsChangedError);
      expect(isPrivacyConsentSaved(stale)).toBe(true);

      const stranded = await respondWith(
        400,
        JSON.stringify({
          error: "Bitte zuerst den Heimweg dieses Kindes anpassen.",
          code: "companion_would_lose_departure",
          details,
        }),
      );
      expect(stranded).toBeInstanceOf(CompanionDepartureRefusedError);
      expect(isPrivacyConsentSaved(stranded)).toBe(true);

      // …and stays absent when the consent write never ran.
      const unmarked = await respondWith(
        409,
        JSON.stringify({
          error: "Die Laufgemeinschaft wurde zwischenzeitlich geändert.",
          code: "companions_changed",
        }),
      );
      expect(unmarked).toBeInstanceOf(CompanionsChangedError);
      expect(isPrivacyConsentSaved(unmarked)).toBe(false);
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

  describe("groupService.getGroupStudents", () => {
    it("fetches students for a group successfully", async () => {
      const mockStudents = [
        { id: 1, first_name: "John", last_name: "Doe" },
        { id: 2, first_name: "Jane", last_name: "Smith" },
      ];
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: mockStudents }),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await groupService.getGroupStudents("123");
        expect(global.fetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/groups/123/students"),
          expect.any(Object),
        );
        expect(result).toEqual(mockStudents);
      } finally {
        restore();
      }
    });

    it("handles direct array response format", async () => {
      const mockStudents = [{ id: 1, first_name: "Test", last_name: "User" }];
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockStudents),
      });

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await groupService.getGroupStudents("123");
        expect(result).toEqual(mockStudents);
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

      const { groupService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(groupService.getGroupStudents("999")).rejects.toThrow();
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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
        user: { token: "test-token" },
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

  describe("error handling utilities", () => {
    it("handleApiError wraps errors with context", async () => {
      const error = new Error("Network failure");
      await import("./api");

      // Trigger an error through studentService
      global.fetch = vi.fn().mockRejectedValue(error);

      const { getSession } = await import("next-auth/react");
      vi.mocked(getSession).mockResolvedValue({
        user: { token: "test-token" },
      } as never);

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await expect(
          studentService.getStudents({ token: "test" }),
        ).rejects.toThrow("Error fetching students");
      } finally {
        restore();
      }
    });

    it("parseApiErrorMessage extracts error from JSON", async () => {
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
          studentService.createStudent({
            first_name: "Test",
            second_name: "User",
            school_class: "1a",
            name: "Test User",
            current_location: "",
          }),
        ).rejects.toThrow("Validation failed");
      } finally {
        restore();
      }
    });

    it("extractApiError falls back to pattern matching", async () => {
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

  describe("query parameter builders", () => {
    it("buildStudentQueryParams includes all filters", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: { data: [], pagination: {} },
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        await studentService.getStudents({
          search: "test search",
          inHouse: true,
          groupId: "42",
          page: 5,
          pageSize: 10,
          token: "test-token",
        });

        const url = vi.mocked(fetchWithRetry).mock.calls[0]?.[0] ?? "";
        expect(url).toContain("search=test+search");
        expect(url).toContain("in_house=true");
        expect(url).toContain("group_id=42");
        expect(url).toContain("page=5");
        expect(url).toContain("page_size=10");
      } finally {
        restore();
      }
    });

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

    it("parseSingleStudentResponse handles wrapped format", async () => {
      const { fetchWithRetry } = await import("./api-helpers");
      const mockStudent = {
        data: { id: 1, first_name: "Test", last_name: "Student" },
      };
      vi.mocked(fetchWithRetry).mockResolvedValue({
        data: mockStudent,
        response: new Response(),
      });

      const { studentService } = await import("./api");

      const restore = setupBrowserEnv();
      try {
        const result = await studentService.getStudent("1");
        expect(result).toBeDefined();
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

    it("validateStudentForCreation checks all required fields", async () => {
      const { studentService } = await import("./api");

      // Test missing first_name
      await expect(
        studentService.createStudent({
          first_name: "",
          second_name: "Last",
          school_class: "1a",
          name: "Test",
          current_location: "",
        }),
      ).rejects.toThrow("First name is required");

      // Test missing second_name
      await expect(
        studentService.createStudent({
          first_name: "First",
          second_name: "",
          school_class: "1a",
          name: "Test",
          current_location: "",
        }),
      ).rejects.toThrow("Last name is required");

      // Test missing school_class
      await expect(
        studentService.createStudent({
          first_name: "First",
          second_name: "Last",
          school_class: "",
          name: "Test",
          current_location: "",
        }),
      ).rejects.toThrow("School class is required");
    });
  });
});
