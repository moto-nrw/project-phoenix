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

describe("GET /api/user-context", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("maps backend data into user context response", async () => {
    const groups = [
      {
        id: 1,
        name: "Group A",
        room_id: 10,
        room: { id: 10, name: "Room 10" },
        via_substitution: true,
      },
    ];
    const supervised = [
      {
        id: 2,
        name: "Group B",
        room_id: 11,
        room: { id: 11, name: "Room 11" },
      },
    ];
    const staff = { id: 5, person_id: 7 };

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: supervised })
      .mockResolvedValueOnce({ data: staff });

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/me/groups",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/me/groups/supervised",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      3,
      "/api/me/staff",
      "test-token",
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        educationalGroups: Array<{
          id: string;
          name: string;
          roomId?: string;
          room?: { id: string; name: string };
          viaSubstitution?: boolean;
        }>;
        supervisedGroups: Array<{
          id: string;
          name: string;
          roomId?: string;
          room?: { id: string; name: string };
        }>;
        currentStaff: { id: string; personId: string } | null;
        educationalGroupIds: string[];
        educationalGroupRoomNames: string[];
        supervisedRoomNames: string[];
      }>
    >(response);

    expect(json.success).toBe(true);
    expect(json.data.educationalGroups).toEqual([
      {
        id: "1",
        name: "Group A",
        roomId: "10",
        room: { id: "10", name: "Room 10" },
        viaSubstitution: true,
      },
    ]);
    expect(json.data.supervisedGroups).toEqual([
      {
        id: "2",
        name: "Group B",
        roomId: "11",
        room: { id: "11", name: "Room 11" },
      },
    ]);
    expect(json.data.currentStaff).toEqual({ id: "5", personId: "7" });
    expect(json.data.educationalGroupIds).toEqual(["1"]);
    expect(json.data.educationalGroupRoomNames).toEqual(["Room 10"]);
    expect(json.data.supervisedRoomNames).toEqual(["Room 11"]);
  });

  it("falls back to empty data when backend calls fail", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("Groups failed"))
      .mockRejectedValueOnce(new Error("Supervised failed"))
      .mockRejectedValueOnce(new Error("API error (404)"));

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        educationalGroups: unknown[];
        supervisedGroups: unknown[];
        currentStaff: unknown;
        educationalGroupIds: string[];
        educationalGroupRoomNames: string[];
        supervisedRoomNames: string[];
      }>
    >(response);

    expect(json.data.educationalGroups).toEqual([]);
    expect(json.data.supervisedGroups).toEqual([]);
    expect(json.data.currentStaff).toBeNull();
    expect(json.data.educationalGroupIds).toEqual([]);
    expect(json.data.educationalGroupRoomNames).toEqual([]);
    expect(json.data.supervisedRoomNames).toEqual([]);
  });
});
