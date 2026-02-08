import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockGetOperatorToken, mockFetch, mockGetServerApiUrl } = vi.hoisted(
  () => ({
    mockGetOperatorToken: vi.fn<() => Promise<string | undefined>>(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }),
);

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

function createMockRequest(body: unknown): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/profile/password",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("POST /api/operator/profile/password", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("changes password successfully with snake_case body", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const body = {
      current_password: "oldpass123",
      new_password: "newpass456",
    };

    const request = createMockRequest(body);
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile/password",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          current_password: "oldpass123",
          new_password: "newpass456",
        }),
      }),
    );
  });

  it("changes password successfully with camelCase body", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const body = {
      currentPassword: "oldpass123",
      newPassword: "newpass456",
    };

    const request = createMockRequest(body);
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile/password",
      expect.objectContaining({
        body: JSON.stringify({
          current_password: "oldpass123",
          new_password: "newpass456",
        }),
      }),
    );
  });

  it("handles empty body gracefully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const request = createMockRequest({});
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        body: JSON.stringify({
          current_password: "",
          new_password: "",
        }),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = createMockRequest({
      current_password: "old",
      new_password: "new",
    });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
  });

  it("handles incorrect current password", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () =>
        JSON.stringify({ error: "Current password is incorrect" }),
    });

    const request = createMockRequest({
      current_password: "wrongpass",
      new_password: "newpass456",
    });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("handles weak new password error", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "Password too weak" }),
    });

    const request = createMockRequest({
      current_password: "oldpass123",
      new_password: "weak",
    });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(400);
  });
});
