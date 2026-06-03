import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { DELETE } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: vi.fn(),
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

function createRequest(): NextRequest {
  return new NextRequest(
    new URL("/api/students/42/status-days/7", "http://localhost:3000"),
    { method: "DELETE" },
  );
}

function createContext(id?: string, statusDayId?: string) {
  return { params: Promise.resolve({ id, statusDayId }) };
}

describe("DELETE /api/students/[id]/status-days/[statusDayId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies delete to backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const response = await DELETE(createRequest(), createContext("42", "7"));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/42/status-days/7",
      "test-token",
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      success: true,
      data: { deleted: true },
    });
  });

  it("returns 401 without a session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await DELETE(createRequest(), createContext("42", "7"));

    expect(response.status).toBe(401);
  });

  it("requires both route params", async () => {
    const response = await DELETE(createRequest(), createContext("42"));

    expect(response.status).toBe(500);
    await expect(response.json()).resolves.toEqual({
      error: "Student ID and status day ID are required",
    });
  });
});
