import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT, DELETE } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
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

interface ApiErrorResponse {
  error: string;
  code?: string;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("PUT /api/settings/og/[ogId]/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({ ogId: "1", key: "test.key" }));

    expect(response.status).toBe(401);
  });

  it("updates OG setting successfully", async () => {
    const mockData = { key: "test.key", value: true };
    mockApiPut.mockResolvedValueOnce(mockData);

    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({ ogId: "1", key: "test.key" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/og/1/test.key",
      "test-token",
      { value: true },
    );
    expect(response.status).toBe(200);
  });

  it("throws error when ogId parameter is missing", async () => {
    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({ key: "test.key" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid ogId or key parameter");
  });

  it("throws error when key parameter is missing", async () => {
    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({ ogId: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid ogId or key parameter");
  });

  it("throws error when both parameters are missing", async () => {
    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid ogId or key parameter");
  });
});

describe("DELETE /api/settings/og/[ogId]/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ ogId: "1", key: "test.key" }));

    expect(response.status).toBe(401);
  });

  it("deletes OG setting successfully", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ ogId: "1", key: "test.key" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/settings/og/1/test.key",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("throws error when ogId parameter is missing", async () => {
    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ key: "test.key" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid ogId or key parameter");
  });

  it("throws error when key parameter is missing", async () => {
    const request = createMockRequest("/api/settings/og/1/test.key", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ ogId: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid ogId or key parameter");
  });
});
