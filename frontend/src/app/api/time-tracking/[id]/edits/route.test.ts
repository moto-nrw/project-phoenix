import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/time-tracking/[id]/edits", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/42/edits");
    const response = await GET(request, createMockContext({ id: "42" }));

    expect(response.status).toBe(401);
  });

  it("fetches edit audit trail for session", async () => {
    const mockEdits = [
      {
        id: 1,
        field: "status",
        old_value: "present",
        new_value: "home_office",
        edited_at: "2024-01-15T10:00:00Z",
      },
      {
        id: 2,
        field: "notes",
        old_value: "",
        new_value: "Updated notes",
        edited_at: "2024-01-15T11:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockEdits });

    const request = createMockRequest("/api/time-tracking/42/edits");
    const response = await GET(request, createMockContext({ id: "42" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/time-tracking/42/edits",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEdits>>(response);
    expect(json.data).toEqual(mockEdits);
  });

  it("returns 500 when id param is invalid", async () => {
    const request = createMockRequest("/api/time-tracking/bad/edits");
    const response = await GET(request, createMockContext({ id: ["a", "b"] }));
    expect(response.status).toBe(500);
  });
});
