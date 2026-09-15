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

vi.mock("@/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", () => ({
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

describe("PUT /api/students/class-arrival-exceptions/[schoolClass]/[date]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      { method: "PUT", body: { arrival_time: "12:45" } },
    );
    const response = await PUT(
      request,
      createMockContext({ schoolClass: "4a", date: "2027-03-01" }),
    );

    expect(response.status).toBe(401);
  });

  it("forwards the class and date to the backend", async () => {
    const saved = {
      school_class: "4a",
      date: "2027-03-01",
      arrival_time: "12:45",
      reason: "Unterricht fällt aus",
    };
    mockApiPut.mockResolvedValueOnce({ data: saved });

    const request = createMockRequest(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      {
        method: "PUT",
        body: { arrival_time: "12:45", reason: "Unterricht fällt aus" },
      },
    );
    const response = await PUT(
      request,
      createMockContext({ schoolClass: "4a", date: "2027-03-01" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      "test-token",
      { arrival_time: "12:45", reason: "Unterricht fällt aus" },
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: typeof saved };
    expect(json.data).toEqual(saved);
  });

  it("encodes a class with a space", async () => {
    mockApiPut.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest(
      "/api/students/class-arrival-exceptions/Klasse%204a/2027-03-01",
      { method: "PUT", body: { arrival_time: "12:45" } },
    );
    await PUT(
      request,
      createMockContext({ schoolClass: "Klasse 4a", date: "2027-03-01" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/class-arrival-exceptions/Klasse%204a/2027-03-01",
      "test-token",
      { arrival_time: "12:45" },
    );
  });
});

describe("DELETE /api/students/class-arrival-exceptions/[schoolClass]/[date]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes the exception", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      { method: "DELETE" },
    );
    const response = await DELETE(
      request,
      createMockContext({ schoolClass: "4a", date: "2027-03-01" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles deletion errors", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest(
      "/api/students/class-arrival-exceptions/4a/2027-03-01",
      { method: "DELETE" },
    );
    const response = await DELETE(
      request,
      createMockContext({ schoolClass: "4a", date: "2027-03-01" }),
    );

    expect(response.status).toBe(404);
  });
});
