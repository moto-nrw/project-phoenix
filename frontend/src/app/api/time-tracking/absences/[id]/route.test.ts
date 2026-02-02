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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("PUT /api/time-tracking/absences/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/absences/42", {
      method: "PUT",
      body: { note: "Updated note" },
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(response.status).toBe(401);
  });

  it("updates an absence successfully", async () => {
    const updateRequest = {
      absence_type: "sick",
      date_start: "2024-01-15",
      date_end: "2024-01-16",
      note: "Doctor visit",
    };
    const mockUpdatedAbsence = {
      id: 42,
      absence_type: "sick",
      date_start: "2024-01-15",
      date_end: "2024-01-16",
      note: "Doctor visit",
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedAbsence });

    const request = createMockRequest("/api/time-tracking/absences/42", {
      method: "PUT",
      body: updateRequest,
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/absences/42",
      "test-token",
      updateRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedAbsence>>(response);
    expect(json.data).toEqual(mockUpdatedAbsence);
  });
});

describe("DELETE /api/time-tracking/absences/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/absences/42", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "42" }));

    expect(response.status).toBe(401);
  });

  it("deletes an absence successfully", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/time-tracking/absences/42", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "42" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/time-tracking/absences/42",
      "test-token",
    );
    expect(response.status).toBe(204);
  });
});
