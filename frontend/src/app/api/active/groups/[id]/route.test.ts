import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<typeof fetch>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
  }),
);

global.fetch = mockFetch;

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function expectBackendCall(method: string, body?: unknown) {
  expect(mockFetch).toHaveBeenCalledWith(
    "http://server:8080/api/active/groups/123",
    expect.objectContaining({
      method,
      body: body === undefined ? undefined : JSON.stringify(body),
      headers: expect.objectContaining({
        Authorization: "Bearer test-token",
      }) as Record<string, string>,
    }),
  );
}

// ============================================================================
// Test Helpers
// ============================================================================

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/active/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches active group by ID from backend", async () => {
    const mockGroup = {
      id: 123,
      name: "OGS Group A",
      room_id: 10,
      description: "Test group",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(mockGroup));

    const request = createMockRequest("/api/active/groups/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expectBackendCall("GET");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGroup>>(response);
    expect(json.data).toEqual(mockGroup);
  });
});

describe("PUT /api/active/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates active group via backend", async () => {
    const updateBody = {
      name: "Updated OGS Group",
      description: "Updated description",
    };
    const mockUpdatedGroup = {
      id: 123,
      name: "Updated OGS Group",
      description: "Updated description",
      room_id: 10,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(mockUpdatedGroup));

    const request = createMockRequest("/api/active/groups/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expectBackendCall("PUT", updateBody);
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGroup>>(response);
    expect(json.data.name).toBe("Updated OGS Group");
  });

  it("updates group with partial fields", async () => {
    const updateBody = { room_id: "15" };
    const mockUpdatedGroup = {
      id: 123,
      name: "OGS Group A",
      room_id: 15,
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(mockUpdatedGroup));

    const request = createMockRequest("/api/active/groups/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expectBackendCall("PUT", updateBody);
    expect(response.status).toBe(200);
  });
});

describe("DELETE /api/active/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes active group via backend and returns 204", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const request = createMockRequest("/api/active/groups/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expectBackendCall("DELETE");
    expect(response.status).toBe(204);
  });
});
