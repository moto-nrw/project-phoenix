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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches supervisors from activities API with data wrapper", async () => {
    const mockSupervisors = [
      { id: 10, name: "Mr. Smith" },
      { id: 11, name: "Ms. Johnson" },
    ];

    mockApiGet.mockResolvedValueOnce({ data: mockSupervisors });

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/supervisors/available",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.name).toBe("Mr. Smith");
  });

  it("fetches supervisors from activities API as direct array", async () => {
    const mockSupervisors = [
      { id: 15, name: "Dr. Brown" },
      { id: 16, name: "Prof. White" },
    ];

    mockApiGet.mockResolvedValueOnce(mockSupervisors);

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
  });

  it("falls back to staff endpoint with data wrapper", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("Activities endpoint unavailable"))
      .mockResolvedValueOnce({
        data: [
          {
            id: 20,
            person: { first_name: "Alice", last_name: "Green" },
          },
          {
            id: 21,
            person: { first_name: "Bob", last_name: "Lee" },
          },
        ],
      });

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff?teachers_only=true",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.name).toBe("Alice Green");
    expect(json.data[1]?.name).toBe("Bob Lee");
  });

  it("falls back to staff endpoint as direct array", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("Activities endpoint unavailable"))
      .mockResolvedValueOnce([
        {
          id: 25,
          person: { first_name: "Charlie", last_name: "King" },
        },
      ]);

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.name).toBe("Charlie King");
  });

  it("handles staff without person data gracefully", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("Activities endpoint unavailable"))
      .mockResolvedValueOnce({
        data: [
          { id: 30 },
          {
            id: 31,
            person: { first_name: "Diana", last_name: "Prince" },
          },
        ],
      });

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.name).toBe("Teacher 30");
    expect(json.data[1]?.name).toBe("Diana Prince");
  });

  it("returns empty array when all endpoints fail", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("Activities endpoint unavailable"))
      .mockRejectedValueOnce(new Error("Staff endpoint unavailable"));

    const request = createMockRequest("/api/activities/supervisors");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<unknown>;
    }>(response);
    expect(json.data).toEqual([]);
  });
});
