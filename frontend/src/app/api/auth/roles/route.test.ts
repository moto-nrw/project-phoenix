import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

global.fetch = mockFetch as typeof fetch;

// ============================================================================
// Test Helpers
// ============================================================================

function createMockResponse(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(data), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles");
    const response = await GET(request);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toBe("Unauthorized");
  });

  it("fetches roles from backend", async () => {
    const mockRoles = {
      roles: [
        { id: 1, name: "Admin", description: "Administrator role" },
        { id: 2, name: "Teacher", description: "Teacher role" },
      ],
      total: 2,
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockRoles));

    const request = createMockRequest("/api/auth/roles");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { roles: unknown[] };
    expect(json.roles).toEqual(mockRoles.roles);
  });

  it("forwards query parameters to backend", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ roles: [] }));

    const request = createMockRequest("/api/auth/roles", {
      searchParams: { page: "2", limit: "10" },
    });
    await GET(request);

    const fetchUrl = (mockFetch.mock.calls[0]?.[0] as string) ?? "";
    expect(fetchUrl).toContain("page=2");
    expect(fetchUrl).toContain("limit=10");
  });

  it("handles backend errors", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ detail: "Backend error" }, 500),
    );

    const request = createMockRequest("/api/auth/roles");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toBeTruthy();
  });
});

describe("POST /api/auth/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles", {
      method: "POST",
      body: { name: "New Role" },
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
  });

  it("creates a new role", async () => {
    const newRole = {
      id: 3,
      name: "Custom Role",
      description: "Custom role description",
    };
    mockFetch.mockReturnValueOnce(createMockResponse(newRole));

    const request = createMockRequest("/api/auth/roles", {
      method: "POST",
      body: { name: "Custom Role", description: "Custom role description" },
    });
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
        body: expect.stringContaining("Custom Role") as string,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { id: number };
    expect(json.id).toBe(3);
  });

  it("handles backend errors during creation", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ detail: "Role already exists" }, 409),
    );

    const request = createMockRequest("/api/auth/roles", {
      method: "POST",
      body: { name: "Duplicate Role" },
    });
    const response = await POST(request);

    expect(response.status).toBe(409);
  });
});
