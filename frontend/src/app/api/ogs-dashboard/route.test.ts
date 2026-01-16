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
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual([]);
    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.firstGroupId).toBeNull();
  });

  it("fetches dashboard data for the first group", async () => {
    const groups = [{ id: 12, name: "OGS A" }];
    const students = [{ id: 1, first_name: "Mia", second_name: "M" }];
    const roomStatus = { group_has_room: true, student_room_status: {} };
    const substitutions = [{ id: 99, group_id: 12 }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: students })
      .mockResolvedValueOnce({ data: roomStatus })
      .mockResolvedValueOnce({ data: substitutions });

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

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: typeof groups;
        students: typeof students;
        roomStatus: typeof roomStatus;
        substitutions: typeof substitutions;
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual(groups);
    expect(json.data.students).toEqual(students);
    expect(json.data.roomStatus).toEqual(roomStatus);
    expect(json.data.substitutions).toEqual(substitutions);
    expect(json.data.firstGroupId).toBe("12");
  });

  it("falls back to defaults when parallel fetches fail", async () => {
    const groups = [{ id: 5, name: "OGS B" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockRejectedValueOnce(new Error("Students failed"))
      .mockRejectedValueOnce(new Error("Room status failed"))
      .mockRejectedValueOnce(new Error("Substitutions failed"));

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.firstGroupId).toBe("5");
  });
});
