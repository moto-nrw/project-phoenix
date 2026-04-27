import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers", () => ({
  apiGet: vi.fn(),
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("POST /api/students/[id]/arrival-notes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/arrival-notes", {
      method: "POST",
      body: { date: "2024-01-15", note: "Test note" },
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("creates an arrival note successfully", async () => {
    const mockNote = {
      data: {
        id: 1,
        student_id: 123,
        date: "2024-01-15",
        note: "Test note",
      },
    };
    mockApiPost.mockResolvedValueOnce(mockNote);

    const request = createMockRequest("/api/students/123/arrival-notes", {
      method: "POST",
      body: { date: "2024-01-15", note: "Test note" },
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/123/arrival-notes",
      "test-token",
      { date: "2024-01-15", note: "Test note" },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: number; student_id: number };
    }>(response);
    expect(json.data.id).toBe(1);
  });

  it("handles backend errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Validation failed"));

    const request = createMockRequest("/api/students/123/arrival-notes", {
      method: "POST",
      body: { date: "2024-01-15" },
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
