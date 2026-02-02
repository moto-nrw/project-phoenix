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

vi.mock("~/lib/api-client", () => ({
  apiGet: mockApiGet,
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

describe("GET /api/database/counts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns database statistics", async () => {
    const dbStats = {
      students: 150,
      teachers: 25,
      rooms: 12,
      activities: 8,
      groups: 6,
      roles: 4,
      devices: 10,
      permissionCount: 50,
      permissions: {
        canViewStudents: true,
        canViewTeachers: true,
        canViewRooms: true,
        canViewActivities: false,
        canViewGroups: true,
        canViewRoles: false,
        canViewDevices: false,
        canViewPermissions: false,
      },
    };

    mockApiGet.mockResolvedValueOnce({ data: dbStats });

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/database/stats",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual(dbStats);
  });

  it("handles backend errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Database connection failed"));

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
