import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/import/teachers/template", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/import/teachers/template");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("downloads CSV template by default", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("Vorname,Nachname,Email,Rolle\n", {
        status: 200,
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition":
            'attachment; filename="mitarbeiter-import-vorlage.csv"',
        },
      }),
    );

    const request = createMockRequest("/api/import/teachers/template");
    const response = await GET(request, createMockContext());

    // Only the browser's format parameter is forwarded; the backend defaults to CSV (dataimport/http/templates.go).
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/teachers/template",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(await response.text()).toBe("Vorname,Nachname,Email,Rolle\n");
  });

  it("supports custom format parameter", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("mock excel data", {
        status: 200,
        headers: {
          "Content-Type":
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          "Content-Disposition":
            'attachment; filename="mitarbeiter-import-vorlage.xlsx"',
        },
      }),
    );

    const request = createMockRequest(
      "/api/import/teachers/template?format=xlsx",
    );
    const response = await GET(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/teachers/template?format=xlsx",
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

    const request = createMockRequest("/api/import/teachers/template");
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

    const request = createMockRequest("/api/import/teachers/template");
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

    const request = createMockRequest("/api/import/teachers/template");
    const response = await GET(request, createMockContext());

    // Contract: a network failure answers 503 general.unavailable (was 500).
    expect(response.status).toBe(503);
    const json = await parseJsonResponse<{ detail: string; code: string }>(
      response,
    );
    expect(json.code).toBe("general.unavailable");
  });

  it("uses default headers when backend doesn't provide them", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(new Blob(["test"]), { status: 200 }),
    );

    const request = createMockRequest("/api/import/teachers/template");
    const response = await GET(request, createMockContext());

    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="mitarbeiter-import-vorlage.csv"',
    );
  });
});
