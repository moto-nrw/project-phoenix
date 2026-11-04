import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

import { createMockContext } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
}));

vi.mock("~/server/auth/caterer", () => ({
  catererAuth: mockAuth,
  uncachedCatererAuth: mockUncachedAuth,
  withCatererAuth: <T>(handler: T) => handler,
}));

import { POST } from "./route";

const fetchMock = vi.fn<typeof fetch>();

function request() {
  return new NextRequest(
    "http://caterer.localhost:3000/api/caterer/account/password",
    {
      method: "POST",
      headers: { "User-Agent": "test-agent", "X-Forwarded-For": "192.0.2.5" },
      body: JSON.stringify({
        currentPassword: "Old-passw0rd!",
        newPassword: "New-passw0rd!",
        confirmPassword: "New-passw0rd!",
      }),
    },
  );
}

describe("POST /api/caterer/account/password (#4496)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv("API_URL", "http://backend.test");
    vi.stubGlobal("fetch", fetchMock);
    mockAuth.mockResolvedValue({ user: { token: "caterer-token" } });
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("sends the fields in the backend's names with the caterer token", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await POST(request(), createMockContext());

    expect(response.status).toBe(204);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://backend.test/caterer/account/password");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      current_password: "Old-passw0rd!",
      new_password: "New-passw0rd!",
      confirm_password: "New-passw0rd!",
    });
    const headers = init?.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer caterer-token");
    expect(headers["User-Agent"]).toBe("test-agent");
  });

  it("forwards a refused current password without renewing the session", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: 401,
          detail: "current password is incorrect",
          code: "identity.current_password_wrong",
        }),
        {
          status: 401,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );

    const response = await POST(request(), createMockContext());

    expect(response.status).toBe(401);
    expect(((await response.json()) as { code: string }).code).toBe(
      "identity.current_password_wrong",
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(mockUncachedAuth).not.toHaveBeenCalled();
  });

  it("refuses a request without a caterer session before calling the backend", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(request(), createMockContext());

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
