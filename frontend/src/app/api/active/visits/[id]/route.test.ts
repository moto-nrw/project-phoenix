import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
import { mockServerSessionData } from "~/test/mocks/next-auth";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  withTenantAuth: (handler: unknown) => handler,
}));

const mockFetch = vi.fn<typeof fetch>();
global.fetch = mockFetch;

// ============================================================================
// Test Helpers
// ============================================================================

const defaultSession = mockServerSessionData() as ExtendedSession;

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function backendCall(index = 0): [string, RequestInit] {
  const [url, init] = mockFetch.mock.calls[index] ?? [];
  return [String(url), init ?? {}];
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches visit by ID from backend", async () => {
    const mockVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(mockVisit));

    const request = createMockRequest("/api/active/visits/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    const [url, init] = backendCall();
    expect(url).toBe("http://server:8080/api/active/visits/123");
    expect(init).toEqual(
      expect.objectContaining({ method: "GET", cache: "no-store" }),
    );
    expect(init.headers).toEqual(
      expect.objectContaining({ Authorization: "Bearer test-token" }),
    );
    expect(response.status).toBe(200);
    // raw: the backend JSON is answered inside the BFF envelope as it is.
    const json =
      await parseJsonResponse<ApiResponse<typeof mockVisit>>(response);
    expect(json.data).toEqual(mockVisit);
  });
});

describe("PUT /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates visit via backend", async () => {
    const updateBody = { end_time: "2024-01-15T17:00:00Z" };
    const mockUpdatedVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
      end_time: "2024-01-15T17:00:00Z",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(mockUpdatedVisit));

    const request = createMockRequest("/api/active/visits/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    const [url, init] = backendCall();
    expect(url).toBe("http://server:8080/api/active/visits/123");
    expect(init).toEqual(expect.objectContaining({ method: "PUT" }));
    expect(JSON.parse(String(init.body))).toEqual(updateBody);
    expect(init.headers).toEqual(
      expect.objectContaining({
        Authorization: "Bearer test-token",
        "Content-Type": "application/json",
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedVisit>>(response);
    expect(json.data.end_time).toBe("2024-01-15T17:00:00Z");
  });
});

describe("DELETE /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes visit via backend and returns 204", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const request = createMockRequest("/api/active/visits/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    const [url, init] = backendCall();
    expect(url).toBe("http://server:8080/api/active/visits/123");
    expect(init).toEqual(expect.objectContaining({ method: "DELETE" }));
    expect(init.headers).toEqual(
      expect.objectContaining({ Authorization: "Bearer test-token" }),
    );
    expect(response.status).toBe(204);
  });
});
