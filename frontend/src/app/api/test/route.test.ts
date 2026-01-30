import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
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
}));

const defaultSession: ExtendedSession = {
  user: {
    id: "1",
    token: "test-token",
    name: "Test User",
    email: "test@example.com",
  },
  expires: "2099-01-01",
};

describe("GET /api/test", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "log").mockImplementation(() => {
      /* noop */
    });
    vi.spyOn(console, "error").mockImplementation(() => {
      /* noop */
    });
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error: string };
    expect(json).toEqual({ error: "No auth token found" });
  });

  it("returns 401 when session has no token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", name: "Test" },
      expires: "2099-01-01",
    });

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error: string };
    expect(json).toEqual({ error: "No auth token found" });
  });

  it("returns session info and backend response on success", async () => {
    const backendResponse = {
      data: [
        { id: 1, name: "Group A" },
        { id: 2, name: "Group B" },
      ],
    };

    mockApiGet.mockResolvedValueOnce(backendResponse);

    const response = await GET();

    expect(mockApiGet).toHaveBeenCalledWith("/api/groups", "test-token");
    expect(response.status).toBe(200);

    const json = (await response.json()) as {
      session: { userId: string; email: string; hasToken: boolean };
      backendResponse: typeof backendResponse;
    };
    expect(json).toEqual({
      session: {
        userId: "1",
        email: "test@example.com",
        hasToken: true,
      },
      backendResponse,
    });
  });

  it("handles backend errors and returns 500", async () => {
    const error = new Error("Backend unavailable");
    mockApiGet.mockRejectedValueOnce(error);

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string; details: unknown };
    expect(json.error).toBe("Backend unavailable");
    expect(json.details).toBeDefined();
  });

  it("handles errors with response data", async () => {
    const errorWithResponse = Object.assign(new Error("API Error"), {
      response: {
        data: { message: "Forbidden", code: 403 },
      },
    });

    mockApiGet.mockRejectedValueOnce(errorWithResponse);

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string; details: unknown };
    expect(json.error).toBe("API Error");
    expect(json.details).toEqual({ message: "Forbidden", code: 403 });
  });

  it("handles non-Error exceptions", async () => {
    mockApiGet.mockRejectedValueOnce("String error");

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string; details: unknown };
    expect(json.error).toBe("Unknown error");
    expect(json.details).toBe("String error");
  });
});
