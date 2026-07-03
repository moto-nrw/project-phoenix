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

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
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

function createMockRequest(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"));
}

function createMockContext() {
  return { params: Promise.resolve({}) };
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/students/school-classes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies distinct school classes from backend", async () => {
    mockApiGet.mockResolvedValueOnce({ data: ["1a", "2b"] });

    const response = await GET(
      createMockRequest("/api/students/school-classes"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/school-classes",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ data: string[] }>(response);
    expect(json.data).toEqual(["1a", "2b"]);
  });
});
