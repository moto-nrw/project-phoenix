import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockGetOperatorToken } = vi.hoisted(() => ({
  mockGetOperatorToken: vi.fn<() => Promise<string | undefined>>(),
}));

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
}));

import { GET } from "./route";

describe("GET /api/operator/me", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns operator data from valid token", async () => {
    const payload = {
      sub: "123",
      username: "admin@test.com",
      first_name: "Admin User",
      scope: "operator",
      exp: Math.floor(Date.now() / 1000) + 3600, // 1 hour from now
    };
    const token = `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
    mockGetOperatorToken.mockResolvedValue(token);

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
  });

  it("returns 401 when no token present", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
  });

  it("returns 401 for invalid token format", async () => {
    mockGetOperatorToken.mockResolvedValue("invalid.token");

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Invalid token");
  });

  it("returns 401 for malformed JWT payload", async () => {
    const token = "header.invalidbase64!@#.signature";
    mockGetOperatorToken.mockResolvedValue(token);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Invalid token");
  });

  it("returns 401 for expired token", async () => {
    const payload = {
      sub: "123",
      username: "admin@test.com",
      first_name: "Admin User",
      scope: "operator",
      exp: Math.floor(Date.now() / 1000) - 3600, // 1 hour ago
    };
    const token = `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
    mockGetOperatorToken.mockResolvedValue(token);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Token expired");
  });

  it("handles token with exactly current timestamp", async () => {
    const payload = {
      sub: "123",
      username: "admin@test.com",
      first_name: "Admin User",
      scope: "operator",
      exp: Math.floor(Date.now() / 1000), // Exactly now
    };
    const token = `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
    mockGetOperatorToken.mockResolvedValue(token);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Token expired");
  });
});
