import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockGetOperatorToken } = vi.hoisted(() => ({
  mockGetOperatorToken: vi.fn<() => Promise<string | undefined>>(),
}));

const { mockOperatorApiGet } = vi.hoisted(() => ({
  mockOperatorApiGet: vi.fn(),
}));

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
}));

vi.mock("~/lib/operator/route-wrapper", () => ({
  operatorApiGet: mockOperatorApiGet,
}));

import { GET } from "./route";

describe("GET /api/operator/me", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns operator data from backend profile", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-jwt-token");
    mockOperatorApiGet.mockResolvedValue({
      id: 123,
      email: "admin@test.com",
      display_name: "Admin User",
    });

    const response = await GET();

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      id?: string;
      email?: string;
      displayName?: string;
    };
    expect(json).toEqual({
      id: "123",
      email: "admin@test.com",
      displayName: "Admin User",
    });
    expect(mockOperatorApiGet).toHaveBeenCalledWith(
      "/operator/profile",
      "valid-jwt-token",
    );
  });

  it("returns 401 when no token present", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
    expect(mockOperatorApiGet).not.toHaveBeenCalled();
  });

  it("returns 401 when backend rejects token", async () => {
    mockGetOperatorToken.mockResolvedValue("expired-or-invalid-token");
    mockOperatorApiGet.mockRejectedValue(
      new Error("API error (401): Unauthorized"),
    );

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
  });
});
