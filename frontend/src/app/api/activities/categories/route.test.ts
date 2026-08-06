import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

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

vi.mock("~/lib/api-helpers.server", () => ({
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

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/categories", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/categories");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches activity categories successfully", async () => {
    const mockCategories = [
      { id: 1, name: "Sports", description: "Physical activities" },
      { id: 2, name: "Arts", description: "Creative activities" },
      { id: 3, name: "Science", description: "STEM activities" },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockCategories,
    });

    const request = createMockRequest("/api/activities/categories");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/categories",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string; description?: string }>;
    }>(response);
    expect(json.data).toHaveLength(3);
    expect(json.data[0]?.id).toBe("1");
    expect(json.data[0]?.name).toBe("Sports");
    expect(json.data[1]?.id).toBe("2");
    expect(json.data[2]?.id).toBe("3");
  });

  it("throws error for unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "error",
      data: null,
    });

    const request = createMockRequest("/api/activities/categories");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Unexpected response structure");
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend service unavailable"));

    const request = createMockRequest("/api/activities/categories");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });

  // #2131: the Stammdaten screen opts into archived (and system) categories
  // via query parameters. Dropping them here would make restoring impossible.
  it("forwards include_archived and include_system to the backend", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/activities/categories?include_archived=true&include_system=true",
    );
    await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/categories?include_archived=true&include_system=true",
      "test-token",
    );
  });

  it("maps is_system, archived_at and usage_count", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: [
        {
          id: 7,
          name: "Essen",
          is_system: true,
          archived_at: "2026-08-01T10:00:00Z",
          usage_count: 3,
          created_at: "2026-08-01T10:00:00Z",
          updated_at: "2026-08-01T10:00:00Z",
        },
      ],
    });

    const request = createMockRequest("/api/activities/categories");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<{
      data: Array<{
        isSystem: boolean;
        archivedAt?: string;
        usageCount?: number;
      }>;
    }>(response);
    expect(json.data[0]?.isSystem).toBe(true);
    expect(json.data[0]?.archivedAt).toBe("2026-08-01T10:00:00Z");
    expect(json.data[0]?.usageCount).toBe(3);
  });
});

describe("POST /api/activities/categories", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  function createPostRequest(body: unknown): NextRequest {
    return new NextRequest(
      new URL("/api/activities/categories", "http://localhost:3000"),
      {
        method: "POST",
        body: JSON.stringify(body),
        headers: { "Content-Type": "application/json" },
      },
    );
  }

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(
      createPostRequest({ name: "Essen" }),
      createMockContext(),
    );

    expect(response.status).toBe(401);
  });

  it("forwards only name, description and color to the backend", async () => {
    mockApiPost.mockResolvedValueOnce({ data: { id: 9, name: "Essen" } });

    const response = await POST(
      createPostRequest({
        name: "Essen",
        description: "Mittagessen",
        color: "#FF9500",
        // Server-managed fields must never reach the backend.
        is_system: true,
        archived_at: "2026-08-01T10:00:00Z",
      }),
      createMockContext(),
    );

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/categories",
      "test-token",
      { name: "Essen", description: "Mittagessen", color: "#FF9500" },
    );
  });
});
