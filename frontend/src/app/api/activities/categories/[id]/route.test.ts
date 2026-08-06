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

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function createContext(params: Record<string, string>) {
  return { params: Promise.resolve(params) };
}

function createRequest(method: string, body?: unknown): NextRequest {
  const url = new URL("/api/activities/categories/7", "http://localhost:3000");
  return new NextRequest(url, {
    method,
    ...(body === undefined
      ? {}
      : {
          body: JSON.stringify(body),
          headers: { "Content-Type": "application/json" },
        }),
  });
}

describe("PUT /api/activities/categories/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await PUT(
      createRequest("PUT", { name: "Essen" }),
      createContext({ id: "7" }),
    );

    expect(response.status).toBe(401);
  });

  it("forwards only the editable fields to the backend", async () => {
    mockApiPut.mockResolvedValueOnce({ data: { id: 7, name: "Essen" } });

    const response = await PUT(
      createRequest("PUT", {
        name: "Essen",
        description: "Mittagessen",
        color: "#FF9500",
        // Server-managed; must not be forwarded.
        is_system: true,
        archived_at: "2026-08-01T10:00:00Z",
      }),
      createContext({ id: "7" }),
    );

    expect(response.status).toBe(200);
    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/categories/7",
      "test-token",
      { name: "Essen", description: "Mittagessen", color: "#FF9500" },
    );
  });

  // The route wrapper falls back to the last numeric path segment when the
  // context carries no params, so the guard only fires for a genuinely
  // id-less URL.
  it("rejects a request without a resolvable id", async () => {
    const response = await PUT(
      new NextRequest(
        new URL("/api/activities/categories/", "http://localhost:3000"),
        {
          method: "PUT",
          body: JSON.stringify({ name: "Essen" }),
          headers: { "Content-Type": "application/json" },
        },
      ),
      createContext({}),
    );

    expect(response.status).toBe(500);
    expect(mockApiPut).not.toHaveBeenCalled();
  });
});

describe("DELETE /api/activities/categories/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("archives via the backend delete endpoint", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const response = await DELETE(
      createRequest("DELETE"),
      createContext({ id: "7" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/activities/categories/7",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await DELETE(
      createRequest("DELETE"),
      createContext({ id: "7" }),
    );

    expect(response.status).toBe(401);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });
});
