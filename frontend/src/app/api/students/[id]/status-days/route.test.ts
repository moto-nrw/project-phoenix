import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
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

function createRequest(path: string, body?: unknown): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"), {
    method: body ? "POST" : "GET",
    body: body ? JSON.stringify(body) : undefined,
    headers: body ? { "Content-Type": "application/json" } : undefined,
  });
}

function createContext(id?: string) {
  return { params: Promise.resolve({ id }) };
}

describe("/api/students/[id]/status-days", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies GET with query params", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [{ id: 1 }] });

    const response = await GET(
      createRequest("/api/students/42/status-days?from=2026-05-25"),
      createContext("42"),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/42/status-days?from=2026-05-25",
      "test-token",
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      success: true,
      data: [{ id: 1 }],
    });
  });

  it("proxies POST body to backend", async () => {
    const body = { status: "sick", dates: ["2026-05-26"] };
    mockApiPost.mockResolvedValueOnce({ data: [{ id: 2 }] });

    const response = await POST(
      createRequest("/api/students/42/status-days", body),
      createContext("42"),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/42/status-days",
      "test-token",
      body,
    );
    expect(response.status).toBe(200);
  });

  it("returns 401 without a session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createRequest("/api/students/42/status-days"),
      createContext("42"),
    );

    expect(response.status).toBe(401);
  });

  it("returns an error when the student id is missing", async () => {
    const response = await POST(
      createRequest("/api/students/status-days", {
        status: "excused",
        dates: ["2026-05-26"],
      }),
      createContext(),
    );

    expect(response.status).toBe(500);
    await expect(response.json()).resolves.toEqual({
      error: "Student ID is required",
    });
  });
});
