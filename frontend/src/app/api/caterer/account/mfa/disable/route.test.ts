import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

import { createMockContext } from "~/test/route-test-helpers";

const { mockAuth } = vi.hoisted(() => ({ mockAuth: vi.fn() }));

vi.mock("~/server/auth/caterer", () => ({
  catererAuth: mockAuth,
  uncachedCatererAuth: vi.fn(),
  withCatererAuth: <T>(handler: T) => handler,
}));

import { POST } from "./route";

const fetchMock = vi.fn<typeof fetch>();

describe("POST /api/caterer/account/mfa/disable (#4497)", () => {
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

  it("sends the current password in the backend's name with the caterer token", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await POST(
      new NextRequest(
        "http://caterer.localhost:3000/api/caterer/account/mfa/disable",
        {
          method: "POST",
          body: JSON.stringify({ currentPassword: "Passw0rd!" }),
        },
      ),
      createMockContext(),
    );

    expect(response.status).toBe(204);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://backend.test/caterer/account/mfa/disable");
    expect(JSON.parse(String(init?.body))).toEqual({
      current_password: "Passw0rd!",
    });
    const headers = init?.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer caterer-token");
  });
});
