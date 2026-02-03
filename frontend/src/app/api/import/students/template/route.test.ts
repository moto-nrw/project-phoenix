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

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

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
    const response = await GET(request);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("downloads CSV template by default", async () => {
    const csvBlob = new Blob(["first_name,second_name,class_name\n"], {
      type: "text/csv",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition":
          'attachment; filename="schueler-import-vorlage.csv"',
      }),
      blob: async () => csvBlob,
    });

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/import/students/template?format=csv",
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
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="schueler-import-vorlage.csv"',
    );
  });

  it("supports custom format parameter", async () => {
    const xlsxBlob = new Blob(["mock excel data"], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({
        "Content-Type":
          "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "Content-Disposition":
          'attachment; filename="schueler-import-vorlage.xlsx"',
      }),
      blob: async () => xlsxBlob,
    });

    const request = createMockRequest(
      "/api/import/students/template?format=xlsx",
    );
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/import/students/template?format=xlsx",
      expect.any(Object),
    );

    expect(response.status).toBe(200);
  });

  it("handles backend errors", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Template generation failed",
    });

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Template generation failed");
  });

  it("handles empty error text from backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      text: async () => "",
    });

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request);

    expect(response.status).toBe(503);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Failed to download template");
  });

  it("handles fetch errors", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal server error");
  });

  it("uses default headers when backend doesn't provide them", async () => {
    const csvBlob = new Blob(["test"]);

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({}),
      blob: async () => csvBlob,
    });

    const request = createMockRequest("/api/import/students/template");
    const response = await GET(request);

    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="schueler-import-vorlage.csv"',
    );
  });
});
