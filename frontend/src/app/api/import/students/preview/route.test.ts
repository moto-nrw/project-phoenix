import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

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

function createMockRequest(formData: FormData): NextRequest {
  const url = new URL("/api/import/students/preview", "http://localhost:3000");
  return new NextRequest(url, {
    method: "POST",
    body: formData,
  });
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

describe("POST /api/import/students/preview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const formData = new FormData();
    formData.append("file", new Blob(["test"]), "test.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("forwards FormData to backend for preview", async () => {
    const mockPreview = {
      valid_students: [
        { first_name: "Max", second_name: "Mustermann", class_name: "1a" },
      ],
      invalid_rows: [],
    };

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockPreview,
    });

    const formData = new FormData();
    const file = new Blob(
      ["first_name,second_name,class_name\nMax,Mustermann,1a"],
      {
        type: "text/csv",
      },
    );
    formData.append("file", file, "students.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/import/students/preview",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockPreview);
  });

  it("forwards backend validation errors", async () => {
    const mockError = {
      error: "Invalid CSV format",
      details: ["Missing required column: first_name"],
    };

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => mockError,
    });

    const formData = new FormData();
    formData.append("file", new Blob(["invalid"]), "invalid.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockError);
  });

  it("handles backend server errors", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({ error: "Database connection failed" }),
    });

    const formData = new FormData();
    formData.append("file", new Blob(["test"]), "test.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(response.status).toBe(500);
  });

  it("handles fetch errors", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const formData = new FormData();
    formData.append("file", new Blob(["test"]), "test.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal server error");
  });
});
