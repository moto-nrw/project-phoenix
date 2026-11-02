import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { RequestCookies } from "next/dist/server/web/spec-extension/cookies";
import {
  MutableRequestCookiesAdapter,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";

const state = vi.hoisted(() => ({
  jar: null as ResponseCookies | null,
  cookie: "",
}));
vi.mock("next/headers", () => ({
  cookies: async () => state.jar,
  headers: async () =>
    new Headers({
      host: "localhost:3000",
      "x-forwarded-proto": "http",
      cookie: state.cookie,
    }),
}));
vi.mock("~/server/auth/tenant-config", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/server/auth/tenant-config")>();
  return {
    tenantAuthConfig: {
      ...original.tenantAuthConfig,
      secret: "synthetic-local-stage-two-session-secret",
    },
  };
});
const { POST: login } = await import("./login/route");
const { POST: verify } = await import("./mfa/verify/route");
const { POST: enroll } = await import("./mfa/enroll/confirm/route");
const { POST: passkey } = await import("./passkeys/login/verify/route");
const { handlers } = await import("~/server/auth");
const backend = vi.fn();
const claims = () => ({
  id: 42,
  sub: "local@example.test",
  tenant_id: 7,
  scope: "",
  exp: Math.floor(Date.now() / 1000) + 900,
});
const access = () =>
  `e30.${Buffer.from(JSON.stringify(claims())).toString("base64url")}.synthetic`;
const pair = () => ({
  access_token: access(),
  refresh_token: "synthetic-server-refresh",
});

function request(
  path: string,
  body: unknown = {},
  origin: string | null = "http://localhost:3000",
  cookie = "",
) {
  state.cookie = cookie;
  state.jar = MutableRequestCookiesAdapter.wrap(
    new RequestCookies(new Headers({ cookie })),
  );
  return new NextRequest(`http://localhost:3000/api/auth/${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(origin === null ? {} : { Origin: origin }),
      cookie,
      Authorization: "Bearer enrollment-proof",
      "X-Forwarded-For": "127.0.0.2",
    },
    body: JSON.stringify(body),
  });
}
function browserCookie(response: Response) {
  const browser = new Map<string, string>();
  for (const cookie of response.headers.getSetCookie()) {
    const part = cookie.split(";")[0]!;
    if (!/Max-Age=0/i.test(cookie))
      browser.set(
        part.slice(0, part.indexOf("=")),
        part.slice(part.indexOf("=") + 1),
      );
  }
  return [...browser].map(([name, value]) => `${name}=${value}`).join("; ");
}
beforeEach(() => {
  vi.clearAllMocks();
  backend.mockReset();
  vi.stubGlobal("fetch", backend);
});
afterEach(() => vi.unstubAllGlobals());

const paths = [
  ["login", "/auth/login", login],
  ["mfa/verify", "/auth/mfa/verify", verify],
  ["mfa/enroll/confirm", "/auth/mfa/enroll/confirm", enroll],
  ["passkeys/login/verify", "/auth/passkeys/login/verify", passkey],
] as const;

describe.each(paths)(
  "%s server session minting",
  (path, backendPath, handler) => {
    it("sets a real httpOnly Auth.js session and exposes no bearer material", async () => {
      backend
        .mockResolvedValueOnce(
          Response.json(
            {
              ...pair(),
              status: "authenticated",
              extra_token: "must-not-leak",
            },
            {
              headers: {
                "Set-Cookie":
                  "trusted_device=synthetic-device; Path=/; HttpOnly; SameSite=Lax",
              },
            },
          ),
        )
        .mockResolvedValueOnce(Response.json(claims()));
      const response = await handler(
        request(
          path,
          { code: "123456" },
          undefined,
          "authjs.session-token.0=obsolete; authjs.session-token.1=obsolete",
        ),
      );
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({ status: "authenticated" });
      expect(response.headers.get("Cache-Control")).toBe("private, no-store");
      expect(backend.mock.calls[0]?.[0]).toBe(
        `http://server:8080${backendPath}`,
      );
      const init = backend.mock.calls[0]?.[1] as RequestInit;
      expect(init.headers).toMatchObject({ "X-Forwarded-For": "127.0.0.2" });
      if (path === "mfa/enroll/confirm")
        expect(init.headers).toMatchObject({
          Authorization: "Bearer enrollment-proof",
        });
      else expect(init.headers).not.toHaveProperty("Authorization");
      expect(response.headers.getSetCookie()).toEqual(
        expect.arrayContaining([
          expect.stringContaining("trusted_device=synthetic-device"),
          expect.stringMatching(/authjs.session-token=.*HttpOnly/),
          expect.stringMatching(/authjs.session-token\.0=.*Max-Age=0/),
          expect.stringMatching(/authjs.session-token\.1=.*Max-Age=0/),
        ]),
      );
      const cookie = browserCookie(response);
      const session = await handlers.GET(
        new NextRequest("http://localhost:3000/api/auth/session", {
          headers: { cookie },
        }),
      );
      const metadata = await session.json();
      expect(metadata.user).toMatchObject({ id: "42", tenantId: 7 });
      // This is the raw server Auth.js view, not a browser route. The session
      // routes' separate allowlist is protected by session-tokens.test.ts.
      expect(cookie).not.toContain(pair().access_token);
      expect(cookie).not.toContain(pair().refresh_token);
    });

    it.each([
      null,
      "null",
      "https://foreign.test",
      "http://other.localhost:3000",
    ])("refuses origin %s before backend authentication", async (origin) => {
      const response = await handler(request(path, {}, origin));
      expect(response.status).toBe(403);
      expect(backend).not.toHaveBeenCalled();
      expect(response.headers.getSetCookie()).toEqual([]);
    });
    it("refuses cross-site requests even with a matching Origin", async () => {
      const req = request(path);
      req.headers.set("Sec-Fetch-Site", "cross-site");
      expect((await handler(req)).status).toBe(403);
      expect(backend).not.toHaveBeenCalled();
    });
    it.each([
      "identity.use_parent_portal",
      "identity.use_school_portal",
      "identity.mfa_code_invalid",
      "identity.tenant_access_denied",
    ])(
      "preserves backend refusal %s without minting a cookie",
      async (code) => {
        const body = JSON.stringify({ code, status: 403, detail: "refused" });
        backend.mockResolvedValueOnce(
          new Response(body, {
            status: 403,
            headers: {
              "Content-Type": "application/problem+json",
              "Retry-After": "12",
            },
          }),
        );
        const response = await handler(request(path));
        expect(response.status).toBe(403);
        expect(await response.text()).toBe(body);
        expect(response.headers.get("Retry-After")).toBe("12");
        expect(backend).toHaveBeenCalledOnce();
        expect(response.headers.getSetCookie()).toEqual([]);
      },
    );
    it.each(["parent", "school", "platform"])(
      "rejects backend-validated wrong-portal scope %s and revokes the new family",
      async (scope) => {
        backend
          .mockResolvedValueOnce(Response.json(pair()))
          .mockResolvedValueOnce(Response.json({ ...claims(), scope }))
          .mockResolvedValueOnce(new Response(null, { status: 204 }));
        const response = await handler(request(path));
        expect(response.status).toBe(500);
        expect(await response.json()).toMatchObject({ code: "general.server" });
        expect(backend).toHaveBeenNthCalledWith(
          3,
          "http://server:8080/auth/logout",
          expect.objectContaining({
            headers: expect.objectContaining({
              Authorization: `Bearer ${pair().refresh_token}`,
            }),
          }),
        );
        expect(response.headers.getSetCookie().join(";")).not.toContain(
          "authjs.session-token=",
        );
      },
    );
    it("keeps rollback/network failures token-free", async () => {
      backend
        .mockResolvedValueOnce(Response.json(pair()))
        .mockResolvedValueOnce(new Response(null, { status: 401 }))
        .mockRejectedValueOnce(new Error("offline"));
      const response = await handler(request(path));
      expect(response.status).toBe(500);
      const text = await response.text();
      expect(text).toContain("general.server");
      expect(text).not.toContain(pair().access_token);
      expect(text).not.toContain(pair().refresh_token);
    });
    it("rejects malformed JSON before authentication", async () => {
      const req = request(path);
      const invalid = new NextRequest(req.url, {
        method: "POST",
        headers: req.headers,
        body: "{",
      });
      expect((await handler(invalid)).status).toBe(400);
      expect(backend).not.toHaveBeenCalled();
    });
  },
);

it("returns only an MFA challenge proof, with no session", async () => {
  backend.mockResolvedValueOnce(
    Response.json({
      status: "mfa_required",
      challenge_token: "challenge-proof",
      masked_email: "l***@example.test",
      trusted_device_enabled: false,
      trusted_device_days: 30,
      unexpected_token: "must-not-leak",
    }),
  );
  const response = await login(request("login"));
  expect(await response.json()).toEqual({
    status: "mfa_required",
    challenge_token: "challenge-proof",
    masked_email: "l***@example.test",
    trusted_device_enabled: false,
    trusted_device_days: 30,
  });
  expect(backend).toHaveBeenCalledOnce();
  expect(response.headers.getSetCookie()).toEqual([]);
});
it("returns only the limited enrollment proof, never a refresh token", async () => {
  backend.mockResolvedValueOnce(
    Response.json({
      status: "mfa_enrollment_required",
      access_token: "enrollment-proof",
      masked_email: "l***@example.test",
      extra_token: "must-not-leak",
    }),
  );
  const response = await login(request("login"));
  expect(await response.json()).toEqual({
    status: "mfa_enrollment_required",
    access_token: "enrollment-proof",
    masked_email: "l***@example.test",
    mfa_enrollment_required: true,
  });
  expect(backend).toHaveBeenCalledOnce();
  expect(response.headers.getSetCookie()).toEqual([]);
});
it("does not disclose a full pair mislabelled as an enrollment challenge", async () => {
  backend.mockResolvedValueOnce(
    Response.json({ ...pair(), status: "mfa_enrollment_required" }),
  );
  const response = await login(request("login"));
  expect(response.status).toBe(500);
  expect(await response.text()).not.toContain(pair().access_token);
});
it("keeps a replayed/expired MFA proof rejected by the backend", async () => {
  const body = {
    status: 401,
    code: "identity.mfa_challenge_expired",
    detail: "expired",
  };
  backend.mockImplementation(async () => Response.json(body, { status: 401 }));
  for (let i = 0; i < 2; i++) {
    const response = await verify(
      request("mfa/verify", { challenge_token: "replayed-proof" }),
    );
    expect(response.status).toBe(401);
    expect(await response.json()).toEqual(body);
    expect(response.headers.getSetCookie()).toEqual([]);
  }
  expect(backend).toHaveBeenCalledTimes(2);
});
