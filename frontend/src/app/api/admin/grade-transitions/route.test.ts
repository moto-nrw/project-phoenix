import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    (error: unknown) =>
      new Response(
        JSON.stringify({
          error:
            error instanceof Error ? error.message : "Internal Server Error",
        }),
        { status: 500 },
      ),
  ),
}));

// ============================================================================
// Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"), {
    method: "GET",
  });
}

function createMockContext() {
  return { params: Promise.resolve({}) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/admin/grade-transitions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockApiGet.mockResolvedValue({ data: [] });
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createMockRequest("/api/admin/grade-transitions"),
      createMockContext(),
    );

    expect(response.status).toBe(401);
  });

  it("forwards the after_id keyset cursor to the backend", async () => {
    const response = await GET(
      createMockRequest(
        "/api/admin/grade-transitions?after_id=9007199254740993&page_size=100",
      ),
      createMockContext(),
    );

    expect(response.status).toBe(200);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/admin/grade-transitions?after_id=9007199254740993&page_size=100",
      "test-token",
    );
  });

  it("forwards the remaining list filters", async () => {
    await GET(
      createMockRequest(
        "/api/admin/grade-transitions?status=applied&academic_year=2026%2F2027&page=2",
      ),
      createMockContext(),
    );

    const [path] = mockApiGet.mock.calls[0] as [string, string];
    const query = new URL(path, "http://localhost").searchParams;
    expect(query.get("status")).toBe("applied");
    expect(query.get("academic_year")).toBe("2026/2027");
    expect(query.get("page")).toBe("2");
  });

  it("drops unknown query parameters", async () => {
    await GET(
      createMockRequest("/api/admin/grade-transitions?tenant_id=42"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/admin/grade-transitions",
      "test-token",
    );
  });

  it("returns an empty list when the backend sends no data", async () => {
    mockApiGet.mockResolvedValueOnce({});

    const response = await GET(
      createMockRequest("/api/admin/grade-transitions"),
      createMockContext(),
    );

    expect(await response.json()).toMatchObject({ data: [] });
  });
});
