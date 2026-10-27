import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

import { POST } from "./route";

const fetchMock = vi.fn<typeof fetch>();

describe("POST /api/caterer/auth/mfa/verify (#4497)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv("API_URL", "http://backend.test");
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("forwards the code and passes the trusted-device cookie back", async () => {
    const backend = Response.json({
      access_token: "access",
      refresh_token: "refresh",
    });
    backend.headers.append(
      "set-cookie",
      "mfa_trust_device=signed; Path=/; HttpOnly; Secure; SameSite=Lax",
    );
    fetchMock.mockResolvedValueOnce(backend);

    const response = await POST(
      new NextRequest(
        "http://caterer.localhost:3000/api/caterer/auth/mfa/verify",
        {
          method: "POST",
          headers: { cookie: "mfa_trust_device=old" },
          body: JSON.stringify({
            challenge_token: "challenge",
            code: "123456",
            remember_device: true,
          }),
        },
      ),
    );

    expect(response.status).toBe(200);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toBe("http://backend.test/caterer/auth/mfa/verify");
    expect(JSON.parse(String(init?.body))).toEqual({
      challenge_token: "challenge",
      code: "123456",
      remember_device: true,
    });
    const headers = init?.headers as Record<string, string>;
    expect(headers.Cookie).toBe("mfa_trust_device=old");
    expect(response.headers.getSetCookie()).toContain(
      "mfa_trust_device=signed; Path=/; HttpOnly; Secure; SameSite=Lax",
    );
  });
});
