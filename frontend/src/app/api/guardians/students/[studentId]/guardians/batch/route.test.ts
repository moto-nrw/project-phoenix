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

vi.mock("@/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(400)")
        ? 400
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

describe("POST /api/guardians/students/[studentId]/guardians/batch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/guardians/students/5/guardians/batch",
      { method: "POST", body: { guardians: [] } },
    );
    const response = await POST(request, createMockContext({ studentId: "5" }));

    expect(response.status).toBe(401);
  });

  it("proxies the batch body to the backend batch endpoint", async () => {
    const batchBody = {
      guardians: [
        { first_name: "A", last_name: "B", relationship_type: "parent" },
      ],
    };
    mockApiPost.mockResolvedValueOnce({ data: null });

    const request = createMockRequest(
      "/api/guardians/students/5/guardians/batch",
      { method: "POST", body: batchBody },
    );
    const response = await POST(request, createMockContext({ studentId: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/guardians/students/5/guardians/batch",
      "test-token",
      batchBody,
    );
    expect(response.status).toBe(200);
  });
});
