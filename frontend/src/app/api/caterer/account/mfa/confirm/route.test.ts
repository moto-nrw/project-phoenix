import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockSetSession } = vi.hoisted(() => ({ mockSetSession: vi.fn() }));

vi.mock("~/server/auth/caterer", () => ({
  setCatererSession: mockSetSession,
  withCatererAuth: <T>(handler: T) => handler,
}));

import { POST } from "./route";

const fetchMock = vi.fn<typeof fetch>();
const ORIGIN = "http://caterer.localhost:3000";

function confirmRequest(token: string | undefined, origin = ORIGIN) {
  const request = new NextRequest(`${ORIGIN}/api/caterer/account/mfa/confirm`, {
    method: "POST",
    headers: { origin, "Content-Type": "application/json" },
    body: JSON.stringify({ challenge_token: "challenge", code: "123456" }),
  });
  return Object.assign(request, {
    auth: token ? { user: { token } } : null,
  }) as unknown as Parameters<typeof POST>[0];
}

describe("POST /api/caterer/account/mfa/confirm (#4497)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv("API_URL", "http://backend.test");
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("installs the new session in the BFF and never returns its tokens", async () => {
    fetchMock.mockResolvedValueOnce(
      Response.json({ access_token: "access", refresh_token: "refresh" }),
    );
    mockSetSession.mockResolvedValueOnce(undefined);

    const response = await POST(confirmRequest("caterer-token"));

    expect(response.status).toBe(200);
    const body = await response.text();
    expect(JSON.parse(body)).toEqual({ session_renewed: true });
    expect(body).not.toContain("access");
    expect(body).not.toContain("refresh");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toBe("http://backend.test/caterer/account/mfa/confirm");
    expect((init!.headers as Record<string, string>).Authorization).toBe(
      "Bearer caterer-token",
    );
    expect(JSON.parse(String(init?.body))).toEqual({
      challenge_token: "challenge",
      code: "123456",
    });
    expect(mockSetSession).toHaveBeenCalledWith("access", "refresh");
  });

  it("reports a session it could not install and revokes it", async () => {
    fetchMock
      .mockResolvedValueOnce(
        Response.json({ access_token: "access", refresh_token: "refresh" }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    mockSetSession.mockRejectedValueOnce(new Error("refused"));

    const response = await POST(confirmRequest("caterer-token"));

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ session_renewed: false });
    const [url, init] = fetchMock.mock.calls[1]!;
    expect(String(url)).toBe("http://backend.test/auth/logout");
    expect((init!.headers as Record<string, string>).Authorization).toBe(
      "Bearer refresh",
    );
  });

  it("passes a wrong code through without touching the session", async () => {
    const problem = { code: "identity.mfa_code_invalid", status: 401 };
    fetchMock.mockResolvedValueOnce(Response.json(problem, { status: 401 }));

    const response = await POST(confirmRequest("caterer-token"));

    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject(problem);
    expect(mockSetSession).not.toHaveBeenCalled();
  });

  it("refuses a request without a caterer session or from another origin", async () => {
    expect((await POST(confirmRequest(undefined))).status).toBe(401);
    expect(
      (await POST(confirmRequest("caterer-token", "http://other.localhost")))
        .status,
    ).toBe(403);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
