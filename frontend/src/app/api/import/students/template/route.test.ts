import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
  }),
);

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/import/students/template", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("downloads CSV template by default", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("first_name,second_name,class_name\n", {
        status: 200,
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition":
            'attachment; filename="schueler-import-vorlage.csv"',
        },
      }),
    );

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    // Without a format the BFF forwards none; the backend defaults to CSV.
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/students/template",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="schueler-import-vorlage.csv"',
    );
    expect(await response.text()).toBe("first_name,second_name,class_name\n");
  });

  it("supports custom format parameter", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("mock excel data", {
        status: 200,
        headers: {
          "Content-Type":
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          "Content-Disposition":
            'attachment; filename="schueler-import-vorlage.xlsx"',
        },
      }),
    );

    const request = createMockRequest(
      "/api/import/students/template?format=xlsx",
    );
    const response = await GET(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/students/template?format=xlsx",
      expect.any(Object),
    );

    expect(response.status).toBe(200);
  });

  it("replaces a backend server error with a safe problem envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("Template generation failed", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });

  it("replaces an empty backend server error with a safe problem envelope", async () => {
    mockFetch.mockResolvedValueOnce(new Response("", { status: 503 }));

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      status: 503,
      detail: "Service Unavailable",
      code: "general.unavailable",
    });
  });

  it("handles fetch errors", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    // Contract change: an unreachable backend answers 503 general.unavailable.
    expect(response.status).toBe(503);
    const json = await parseJsonResponse<{ code: string }>(response);
    expect(json.code).toBe("general.unavailable");
  });

  it("uses default headers when backend doesn't provide them", async () => {
    const backend = new Response("test", { status: 200 });
    backend.headers.delete("Content-Type");
    mockFetch.mockResolvedValueOnce(backend);

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request, createMockContext());

    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="schueler-import-vorlage.csv"',
    );
  });
});
