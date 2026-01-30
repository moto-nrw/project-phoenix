import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("@/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
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

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
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

describe("PUT /api/students/[id]/pickup-notes/[noteId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "PUT",
      body: { note: "Updated note content" },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(response.status).toBe(401);
  });

  it("updates pickup note successfully", async () => {
    const updatedNote = {
      id: 10,
      student_id: 1,
      note: "Updated note content",
      created_at: "2024-01-15T10:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce({ data: updatedNote });

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "PUT",
      body: { note: "Updated note content" },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/1/pickup-notes/10",
      "test-token",
      { note: "Updated note content" },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof updatedNote>>(response);
    expect(json.data).toEqual(updatedNote);
  });

  it("handles update errors", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Note not found"));

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "PUT",
      body: { note: "Updated note" },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Note not found");
  });
});

describe("DELETE /api/students/[id]/pickup-notes/[noteId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(response.status).toBe(401);
  });

  it("deletes pickup note successfully", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/1/pickup-notes/10",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles deletion errors", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Deletion failed"));

    const request = createMockRequest("/api/students/1/pickup-notes/10", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", noteId: "10" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Deletion failed");
  });
});
