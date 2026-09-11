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

vi.mock("@/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("GET /api/students/class-arrival-exceptions/[schoolClass]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = new NextRequest(
      "http://localhost:3000/api/students/class-arrival-exceptions/4a",
    );
    const response = await GET(
      request,
      createMockContext({ schoolClass: "4a" }),
    );

    expect(response.status).toBe(401);
  });

  it("forwards the class and the query window", async () => {
    const payload = { school_class: "4a", can_edit: true, exceptions: [] };
    mockApiGet.mockResolvedValueOnce({ data: payload });

    const request = new NextRequest(
      "http://localhost:3000/api/students/class-arrival-exceptions/4a?from=2027-03-01&to=2027-03-31",
    );
    const response = await GET(
      request,
      createMockContext({ schoolClass: "4a" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/class-arrival-exceptions/4a?from=2027-03-01&to=2027-03-31",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: typeof payload };
    expect(json.data).toEqual(payload);
  });
});
