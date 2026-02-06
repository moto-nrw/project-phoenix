import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";
import { mockSessionData } from "~/test/mocks/next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
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

const defaultSession = mockSessionData() as ExtendedSession;

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/ogs-dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns empty payload when user has no groups", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledTimes(1);
    expect(mockApiGet).toHaveBeenCalledWith("/api/me/groups", "test-token");

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: unknown[];
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        pickupTimes: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual([]);
    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.pickupTimes).toEqual([]);
    expect(json.data.firstGroupId).toBeNull();
  });

  it("fetches dashboard data for the first group", async () => {
    const groups = [{ id: 12, name: "OGS A" }];
    const students = [{ id: 1, first_name: "Mia", last_name: "M" }];
    const roomStatus = { group_has_room: true, student_room_status: {} };
    const substitutions = [{ id: 99, group_id: 12 }];
    const pickupTimes = [{ student_id: 1, pickup_time: "15:00" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: students })
      .mockResolvedValueOnce({ data: roomStatus })
      .mockResolvedValueOnce({ data: substitutions });

    mockApiPost.mockResolvedValueOnce({ data: pickupTimes });

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/me/groups",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?group_id=12",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      3,
      "/api/groups/12/students/room-status",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      4,
      "/api/groups/12/substitutions",
      "test-token",
    );
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/pickup-times/bulk",
      "test-token",
      { student_ids: [1] },
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: typeof groups;
        students: typeof students;
        roomStatus: typeof roomStatus;
        substitutions: typeof substitutions;
        pickupTimes: typeof pickupTimes;
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual(groups);
    expect(json.data.students).toEqual(students);
    expect(json.data.roomStatus).toEqual(roomStatus);
    expect(json.data.substitutions).toEqual(substitutions);
    expect(json.data.pickupTimes).toEqual(pickupTimes);
    expect(json.data.firstGroupId).toBe("12");
  });

  it("falls back to defaults when parallel fetches fail", async () => {
    const groups = [{ id: 5, name: "OGS B" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockRejectedValueOnce(new Error("Students failed"))
      .mockRejectedValueOnce(new Error("Room status failed"))
      .mockRejectedValueOnce(new Error("Substitutions failed"));

    // When students fail, pickupTimes won't be fetched (no student IDs)

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        pickupTimes: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.pickupTimes).toEqual([]);
    expect(json.data.firstGroupId).toBe("5");
  });

  it("sorts groups alphabetically and uses first group", async () => {
    const groups = [
      { id: 20, name: "Zebra" },
      { id: 10, name: "Adler" },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] });

    // No students, so no pickupTimes fetch

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    // After sorting, "Adler" (id=10) should be first
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?group_id=10",
      "test-token",
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: typeof groups;
        pickupTimes: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.firstGroupId).toBe("10");
    expect(json.data.pickupTimes).toEqual([]);
    // Groups should be sorted alphabetically
    expect(json.data.groups).toHaveLength(2);
    expect(json.data.groups.at(0)?.name).toBe("Adler");
    expect(json.data.groups.at(1)?.name).toBe("Zebra");
  });

  it("handles null data arrays gracefully", async () => {
    const groups = [{ id: 1, name: "Group" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: null }) // students null
      .mockResolvedValueOnce({ data: null }) // roomStatus null
      .mockResolvedValueOnce({ data: null }); // substitutions null

    // No students, so no pickupTimes fetch

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        pickupTimes: unknown[];
      }>
    >(response);

    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.pickupTimes).toEqual([]);
  });

  it("returns empty pickup times when bulk pickup fetch fails", async () => {
    const groups = [{ id: 1, name: "Group A" }];
    const students = [{ id: 1, first_name: "Max", last_name: "M" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups }) // groups
      .mockResolvedValueOnce({ data: students }) // students
      .mockResolvedValueOnce({ data: null }) // roomStatus
      .mockResolvedValueOnce({ data: [] }); // substitutions

    // Pickup times fetch fails
    mockApiPost.mockRejectedValueOnce(new Error("Pickup times fetch failed"));

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    // Should still return successful response with empty pickupTimes
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        students: typeof students;
        pickupTimes: unknown[];
        firstGroupId: string;
      }>
    >(response);

    expect(json.data.students).toEqual(students);
    expect(json.data.pickupTimes).toEqual([]);
    expect(json.data.firstGroupId).toBe("1");
  });

  it("handles pickup times with null data field", async () => {
    const groups = [{ id: 1, name: "Group A" }];
    const students = [{ id: 1, first_name: "Max", last_name: "M" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: students })
      .mockResolvedValueOnce({ data: null })
      .mockResolvedValueOnce({ data: [] });

    // Pickup times returns null data
    mockApiPost.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        pickupTimes: unknown[];
      }>
    >(response);

    expect(json.data.pickupTimes).toEqual([]);
  });
});
