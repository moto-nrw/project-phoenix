/**
 * Tests for Active Supervision Dashboard BFF Route
 * Tests the endpoint that consolidates 8+ API calls into 1 for performance
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : message.includes("(403)")
          ? 403
          : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/active-supervision-dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns early with unclaimed groups when no supervised groups", async () => {
    const unclaimedGroups = [
      { id: 1, name: "Schulhof", room: { id: 10, name: "Schulhof" } },
    ];
    const educationalGroups = [
      { id: 2, name: "OGS Gruppe A", room: { id: 20, name: "Raum 101" } },
    ];
    const staff = { id: 5, person_id: 50 };

    // Mock parallel API calls - supervised returns empty
    mockApiGet
      .mockResolvedValueOnce({ data: [] }) // supervised groups
      .mockResolvedValueOnce({ data: unclaimedGroups }) // unclaimed groups
      .mockResolvedValueOnce({ data: staff }) // staff
      .mockResolvedValueOnce({ data: educationalGroups }); // educational groups

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: unknown[];
        unclaimedGroups: Array<{ id: string; name: string }>;
        currentStaff: { id: string } | null;
        educationalGroups: Array<{ id: string; name: string }>;
        firstRoomVisits: unknown[];
        firstRoomId: string | null;
      }>
    >(response);

    expect(json.data.supervisedGroups).toEqual([]);
    expect(json.data.unclaimedGroups).toHaveLength(1);
    expect(json.data.unclaimedGroups[0]?.id).toBe("1");
    expect(json.data.currentStaff?.id).toBe("5");
    expect(json.data.educationalGroups).toHaveLength(1);
    expect(json.data.firstRoomVisits).toEqual([]);
    expect(json.data.firstRoomId).toBeNull();
  });

  it("fetches supervised groups with room data and visits", async () => {
    const supervisedGroups = [
      {
        id: 1,
        name: "Schulhof",
        room_id: 10,
        room: { id: 10, name: "Schulhof" },
      },
    ];
    const unclaimedGroups = [] as Array<{
      id: number;
      name: string;
      room?: { id: number; name: string };
    }>;
    const staff = { id: 5, person_id: 50 };
    const educationalGroups = [
      { id: 2, name: "OGS A", room: { id: 20, name: "Raum 101" } },
    ];
    const visits = [
      {
        id: 100,
        student_id: 200,
        active_group_id: 1,
        check_in_time: "2024-01-15T10:00:00Z",
        student_name: "Max Mustermann",
        school_class: "1a",
        group_name: "OGS Gruppe A",
        is_active: true,
      },
      {
        id: 101,
        student_id: 201,
        active_group_id: 1,
        check_in_time: "2024-01-15T09:00:00Z",
        check_out_time: "2024-01-15T10:30:00Z",
        student_name: "Lisa Schmidt",
        school_class: "2b",
        group_name: "OGS Gruppe B",
        is_active: false, // Should be filtered out
      },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: unclaimedGroups })
      .mockResolvedValueOnce({ data: staff })
      .mockResolvedValueOnce({ data: educationalGroups })
      .mockResolvedValueOnce({ data: visits }); // visits for first group

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{ id: string; name: string }>;
        firstRoomVisits: Array<{
          studentId: string;
          studentName: string;
          isActive: boolean;
        }>;
        firstRoomId: string | null;
      }>
    >(response);

    expect(json.data.supervisedGroups).toHaveLength(1);
    expect(json.data.supervisedGroups[0]?.id).toBe("1");
    expect(json.data.firstRoomId).toBe("1");

    // Only active visits should be returned
    expect(json.data.firstRoomVisits).toHaveLength(1);
    expect(json.data.firstRoomVisits[0]?.studentName).toBe("Max Mustermann");
    expect(json.data.firstRoomVisits[0]?.isActive).toBe(true);
  });

  it("fetches room data when not included in supervised group response", async () => {
    const supervisedGroups = [
      { id: 1, name: "Schulhof", room_id: 10 }, // No room object, needs fetch
    ];
    const roomData = { id: 10, name: "Schulhof" };

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] }) // unclaimed
      .mockResolvedValueOnce({ data: null }) // staff (not found)
      .mockResolvedValueOnce({ data: [] }) // educational groups
      .mockResolvedValueOnce({ data: roomData }) // room fetch
      .mockResolvedValueOnce({ data: [] }); // visits

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{
          id: string;
          name: string;
          room?: { id: string; name: string };
        }>;
      }>
    >(response);

    expect(json.data.supervisedGroups[0]?.room?.name).toBe("Schulhof");
  });

  it("handles API failures gracefully", async () => {
    // All parallel fetches fail
    mockApiGet
      .mockRejectedValueOnce(new Error("Supervised groups error"))
      .mockRejectedValueOnce(new Error("Unclaimed groups error"))
      .mockRejectedValueOnce(new Error("Staff error"))
      .mockRejectedValueOnce(new Error("Educational groups error"));

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: unknown[];
        unclaimedGroups: unknown[];
        currentStaff: unknown;
        educationalGroups: unknown[];
        firstRoomVisits: unknown[];
        firstRoomId: string | null;
      }>
    >(response);

    // Should return empty arrays instead of crashing
    expect(json.data.supervisedGroups).toEqual([]);
    expect(json.data.unclaimedGroups).toEqual([]);
    expect(json.data.currentStaff).toBeNull();
    expect(json.data.educationalGroups).toEqual([]);
    expect(json.data.firstRoomVisits).toEqual([]);
    expect(json.data.firstRoomId).toBeNull();
  });

  it("handles null response data safely", async () => {
    mockApiGet
      .mockResolvedValueOnce({ data: null }) // supervised (null instead of array)
      .mockResolvedValueOnce({ data: null }) // unclaimed
      .mockResolvedValueOnce({ data: null }) // staff
      .mockResolvedValueOnce({ data: null }); // educational groups

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: unknown[];
        unclaimedGroups: unknown[];
        firstRoomId: string | null;
      }>
    >(response);

    expect(json.data.supervisedGroups).toEqual([]);
    expect(json.data.unclaimedGroups).toEqual([]);
    expect(json.data.firstRoomId).toBeNull();
  });

  it("handles 403 error for visits without crashing", async () => {
    const supervisedGroups = [
      { id: 1, name: "Schulhof", room: { id: 10, name: "Schulhof" } },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: { id: 5 } })
      .mockResolvedValueOnce({ data: [] })
      .mockRejectedValueOnce(new Error("403 Forbidden")); // visits fetch fails with 403

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: unknown[];
        firstRoomVisits: unknown[];
        firstRoomId: string | null;
      }>
    >(response);

    // Should still return supervised groups, just with empty visits
    expect(json.data.supervisedGroups).toHaveLength(1);
    expect(json.data.firstRoomVisits).toEqual([]);
    expect(json.data.firstRoomId).toBe("1");
  });

  it("handles room fetch failure without crashing", async () => {
    const supervisedGroups = [
      { id: 1, name: "Schulhof", room_id: 10 }, // No room object
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] })
      .mockRejectedValueOnce(new Error("Room not found")) // room fetch fails
      .mockResolvedValueOnce({ data: [] }); // visits

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{
          id: string;
          name: string;
          room?: { id: string; name: string };
        }>;
      }>
    >(response);

    // Should still have the group, just without room data
    expect(json.data.supervisedGroups).toHaveLength(1);
    expect(json.data.supervisedGroups[0]?.id).toBe("1");
    expect(json.data.supervisedGroups[0]?.room).toBeUndefined();
  });

  it("converts backend int64 IDs to frontend strings", async () => {
    const supervisedGroups = [
      { id: 123456789, name: "Test", room: { id: 987654321, name: "Room" } },
    ];
    const staff = { id: 555666777 };

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: staff })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{
          id: string;
          room?: { id: string };
        }>;
        currentStaff: { id: string };
      }>
    >(response);

    // All IDs should be strings
    expect(typeof json.data.supervisedGroups[0]?.id).toBe("string");
    expect(json.data.supervisedGroups[0]?.id).toBe("123456789");
    expect(json.data.supervisedGroups[0]?.room?.id).toBe("987654321");
    expect(json.data.currentStaff.id).toBe("555666777");
  });

  it("handles supervised group without room_id", async () => {
    const supervisedGroups = [
      { id: 1, name: "Draussen" }, // No room_id, no room object
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] }) // unclaimed
      .mockResolvedValueOnce({ data: null }) // staff
      .mockResolvedValueOnce({ data: [] }) // educational groups
      .mockResolvedValueOnce({ data: [] }); // visits

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{
          id: string;
          name: string;
          room_id?: string;
          room?: { id: string; name: string };
        }>;
      }>
    >(response);

    expect(json.data.supervisedGroups).toHaveLength(1);
    expect(json.data.supervisedGroups[0]?.id).toBe("1");
    expect(json.data.supervisedGroups[0]?.room_id).toBeUndefined();
    expect(json.data.supervisedGroups[0]?.room).toBeUndefined();
  });

  it("handles visits with missing optional fields", async () => {
    const supervisedGroups = [
      { id: 1, name: "Test", room: { id: 10, name: "Room A" } },
    ];
    const visits = [
      {
        id: 100,
        student_id: 200,
        active_group_id: 1,
        check_in_time: "2024-01-15T10:00:00Z",
        // Missing: student_name, school_class, group_name
        is_active: true,
      },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: visits });

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        firstRoomVisits: Array<{
          studentName: string;
          schoolClass: string;
          groupName: string;
        }>;
      }>
    >(response);

    // Should use empty string defaults for missing optional fields
    expect(json.data.firstRoomVisits).toHaveLength(1);
    expect(json.data.firstRoomVisits[0]?.studentName).toBe("");
    expect(json.data.firstRoomVisits[0]?.schoolClass).toBe("");
    expect(json.data.firstRoomVisits[0]?.groupName).toBe("");
  });

  it("sorts supervised groups by room name", async () => {
    const supervisedGroups = [
      { id: 2, name: "Group B", room: { id: 20, name: "Zimmer" } },
      { id: 1, name: "Group A", room: { id: 10, name: "Aula" } },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: supervisedGroups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: [] }); // visits for first (Aula)

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{ id: string; name: string }>;
        firstRoomId: string | null;
      }>
    >(response);

    // Sorted by room name: "Aula" before "Zimmer"
    expect(json.data.supervisedGroups[0]?.id).toBe("1");
    expect(json.data.firstRoomId).toBe("1");
  });

  it("maps unclaimed groups with room names correctly", async () => {
    const unclaimedGroups = [
      { id: 5, name: "Unclaimed", room: { id: 50, name: "Hof" } },
      { id: 6, name: "No Room" }, // No room object
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: [] }) // no supervised groups
      .mockResolvedValueOnce({ data: unclaimedGroups })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/active-supervision-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        unclaimedGroups: Array<{
          id: string;
          name: string;
          room?: { name: string };
        }>;
      }>
    >(response);

    expect(json.data.unclaimedGroups).toHaveLength(2);
    expect(json.data.unclaimedGroups[0]?.room?.name).toBe("Hof");
    expect(json.data.unclaimedGroups[1]?.room).toBeUndefined();
  });
});
