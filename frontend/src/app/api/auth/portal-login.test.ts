import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { RequestCookies } from "next/dist/server/web/spec-extension/cookies";
import {
  MutableRequestCookiesAdapter,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";

const state = vi.hoisted(() => ({
  jar: null as ResponseCookies | null,
  host: "localhost:3000",
  cookie: "",
}));
vi.mock("next/headers", () => ({
  cookies: async () => state.jar,
  headers: async () =>
    new Headers({
      host: state.host,
      "x-forwarded-proto": "http",
      cookie: state.cookie,
    }),
}));
vi.mock("~/server/auth/operator-config", async (original) => ({
  operatorAuthConfig: {
    ...(await original<typeof import("~/server/auth/operator-config")>())
      .operatorAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));
vi.mock("~/server/auth/school-config", async (original) => ({
  schoolAuthConfig: {
    ...(await original<typeof import("~/server/auth/school-config")>())
      .schoolAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));
vi.mock("~/server/auth/traeger-config", async (original) => ({
  traegerAuthConfig: {
    ...(await original<typeof import("~/server/auth/traeger-config")>())
      .traegerAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));
vi.mock("~/server/auth/caterer-config", async (original) => ({
  catererAuthConfig: {
    ...(await original<typeof import("~/server/auth/caterer-config")>())
      .catererAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));
vi.mock("~/server/auth/parent-config", async (original) => ({
  parentAuthConfig: {
    ...(await original<typeof import("~/server/auth/parent-config")>())
      .parentAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));
vi.mock("~/server/auth/tenant-config", async (original) => ({
  tenantAuthConfig: {
    ...(await original<typeof import("~/server/auth/tenant-config")>())
      .tenantAuthConfig,
    secret: "synthetic-local-stage-three-session-secret",
  },
}));

import { createPortalLoginRoute } from "~/lib/portal-login-route.server";
import { POST as demo } from "../demo/access/sessions/route";
import { operatorHandlers } from "~/server/auth/operator";
import { schoolHandlers } from "~/server/auth/school";
import { traegerHandlers } from "~/server/auth/traeger";
import { catererHandlers } from "~/server/auth/caterer";
import { parentHandlers } from "~/server/auth/parent";
import { handlers as tenantHandlers } from "~/server/auth";

const backend = vi.fn();
const claims = (scope: string) => ({
  id: 42,
  sub: "local@example.test",
  tenant_id: 7,
  org_id: 9,
  scope,
  exp: Math.floor(Date.now() / 1000) + 900,
});
const pair = () => ({
  access_token: `e30.${Buffer.from(JSON.stringify(claims(""))).toString("base64url")}.synthetic`,
  refresh_token: "synthetic-server-refresh",
});
function request(
  portal: string,
  body: unknown = {},
  origin?: string | null,
  cookie = "",
) {
  state.host = `${portal}.localhost:3000`;
  state.cookie = cookie;
  state.jar = MutableRequestCookiesAdapter.wrap(
    new RequestCookies(new Headers({ cookie })),
  );
  return new NextRequest(`http://${state.host}/api/${portal}/auth/login`, {
    method: "POST",
    headers: {
      host: state.host,
      cookie,
      ...(origin === null ? {} : { Origin: origin ?? `http://${state.host}` }),
      Authorization: "Bearer enrollment-proof",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
}
function browserCookie(response: Response) {
  return response.headers
    .getSetCookie()
    .filter((c) => !c.includes("Max-Age=0"))
    .map((c) => c.split(";")[0])
    .join("; ");
}
beforeEach(() => {
  backend.mockReset();
  vi.stubGlobal("fetch", backend);
});
afterEach(() => vi.unstubAllGlobals());

const portals = [
  ["operator", "platform", operatorHandlers, true],
  ["school", "school", schoolHandlers, false],
  ["traeger", "org", traegerHandlers, false],
  ["caterer", "caterer", catererHandlers, true],
] as const;
describe.each(portals)(
  "%s BFF session",
  (portal, scope, handlers, envelope) => {
    const wrap = (data: unknown) =>
      envelope ? { status: "success", data } : data;
    const steps =
      portal === "caterer"
        ? (["login", "mfa/verify", "passkeys/login/verify"] as const)
        : portal === "operator"
          ? ([
              "login",
              "mfa/verify",
              "mfa/enroll/confirm",
              "passkeys/login/verify",
            ] as const)
          : (["login", "mfa/verify", "mfa/enroll/confirm"] as const);
    it.each(steps)(
      "%s installs only its own encrypted httpOnly cookie",
      async (step) => {
        // Operator passkey verification returns LoginResponse without an
        // envelope; so does the caterer's second factor (#4497).
        const wrapStep =
          step === "passkeys/login/verify" ||
          (portal === "caterer" && step !== "login")
            ? (data: unknown) => data
            : wrap;
        backend
          .mockResolvedValueOnce(
            Response.json(
              wrapStep({ ...pair(), extra_token: "must-not-leak" }),
              {
                headers: {
                  "Set-Cookie": "trusted_device=proof; Path=/; HttpOnly",
                },
              },
            ),
          )
          .mockResolvedValueOnce(Response.json(claims(scope)));
        const response = await createPortalLoginRoute(
          portal,
          step,
        )(
          request(
            portal,
            { organization_id: 9 },
            undefined,
            `${portal}.session-token.0=obsolete; ${portal}.session-token.1=obsolete`,
          ),
        );
        expect(response.status).toBe(200);
        expect(await response.json()).toEqual(
          wrapStep({ status: "authenticated" }),
        );
        expect(backend.mock.calls[0]?.[0]).toBe(
          `http://server:8080/${portal}/auth/${step}`,
        );
        expect(backend.mock.calls[0]?.[1].body).toBe(
          JSON.stringify({ organization_id: 9 }),
        );
        if (step === "mfa/enroll/confirm")
          expect(backend.mock.calls[0]?.[1].headers.Authorization).toBe(
            "Bearer enrollment-proof",
          );
        expect(JSON.parse(backend.mock.calls[1]?.[1].body).portal).toBe(scope);
        const cookies = response.headers.getSetCookie();
        expect(cookies).toContainEqual(
          expect.stringMatching(
            new RegExp(`${portal}\\.session-token=.*HttpOnly`),
          ),
        );
        expect(cookies).toContainEqual(
          expect.stringContaining("trusted_device=proof"),
        );
        expect(cookies).toContainEqual(
          expect.stringMatching(
            new RegExp(`${portal}\\.session-token\\.0=.*Max-Age=0`),
          ),
        );
        expect(cookies).toContainEqual(
          expect.stringMatching(
            new RegExp(`${portal}\\.session-token\\.1=.*Max-Age=0`),
          ),
        );
        expect(cookies.join(";")).not.toMatch(/Domain=/i);
        if (portal === "school")
          expect(cookies).toContainEqual(
            expect.stringMatching(/school\.session-token=.*SameSite=strict/i),
          );
        expect(response.headers.get("Cache-Control")).toBe("private, no-store");
        const cookie = browserCookie(response);
        const session = await handlers.GET(
          new NextRequest(`http://${state.host}/api/${portal}/auth/session`, {
            headers: { cookie },
          }),
        );
        expect((await session.json()).user).toMatchObject({ id: "42", scope });
        // Other portal configs cannot consume this portal's cookie.
        const other = portal === "operator" ? schoolHandlers : operatorHandlers;
        const otherPortal = portal === "operator" ? "school" : "operator";
        const foreign = await other.GET(
          new NextRequest(
            `http://${state.host}/api/${otherPortal}/auth/session`,
            { headers: { cookie } },
          ),
        );
        expect(await foreign.json()).toBeNull();
        expect(cookie).not.toContain(pair().access_token);
        expect(cookie).not.toContain(pair().refresh_token);
      },
    );
    it.each([null, "null", "http://other.localhost:3000"])(
      "rejects Origin %s before minting",
      async (origin) => {
        const response = await createPortalLoginRoute(
          portal,
          "login",
        )(request(portal, {}, origin));
        expect(response.status).toBe(403);
        expect(backend).not.toHaveBeenCalled();
        expect(response.headers.getSetCookie()).toEqual([]);
      },
    );
    it("preserves wrong-portal refusal from the backend", async () => {
      const problem = { code: "identity.use_parent_portal", status: 403 };
      backend.mockResolvedValueOnce(Response.json(problem, { status: 403 }));
      const response = await createPortalLoginRoute(
        portal,
        "login",
      )(request(portal));
      expect(response.status).toBe(403);
      expect(await response.json()).toEqual(problem);
      expect(response.headers.getSetCookie()).toEqual([]);
    });
    it("rolls back a failed handoff using the portal's logout endpoint", async () => {
      backend
        .mockResolvedValueOnce(Response.json(wrap(pair())))
        .mockResolvedValueOnce(Response.json(claims("wrong-portal")))
        .mockResolvedValueOnce(new Response(null, { status: 204 }));
      const response = await createPortalLoginRoute(
        portal,
        "login",
      )(request(portal));
      expect(response.status).toBe(500);
      expect(backend.mock.calls[2]?.[0]).toBe(
        `http://server:8080/${portal === "operator" ? "operator/" : ""}auth/logout`,
      );
      expect(backend.mock.calls[2]?.[1].headers.Authorization).toBe(
        `Bearer ${portal === "operator" ? pair().access_token : pair().refresh_token}`,
      );
      expect(response.headers.getSetCookie().join(";")).not.toContain(
        `${portal}.session-token=`,
      );
      expect(await response.text()).not.toContain(pair().refresh_token);
    });
    // The caterer's second factor is voluntary (#4497): no enrollment step.
    it.each(
      portal === "caterer"
        ? ["mfa_required"]
        : ["mfa_required", "mfa_enrollment_required"],
    )(
      "preserves only the limited %s proof and Träger binding",
      async (status) => {
        const proof =
          status === "mfa_required"
            ? { challenge_token: "challenge" }
            : { access_token: "enrollment", mfa_enrollment_required: true };
        const data = {
          status,
          ...proof,
          masked_email: "a***@example.test",
          ...(portal === "traeger" ? { organization_id: 9 } : {}),
        };
        backend.mockResolvedValueOnce(
          Response.json(wrap({ ...data, extra_token: "must-not-leak" })),
        );
        const response = await createPortalLoginRoute(
          portal,
          "login",
        )(request(portal));
        expect(await response.json()).toEqual(wrap(data));
        expect(response.headers.getSetCookie()).toEqual([]);
        expect(backend).toHaveBeenCalledOnce();
      },
    );
  },
);
it("allowlists Träger organization selection without disclosing credentials", async () => {
  backend.mockResolvedValueOnce(
    Response.json({
      status: "organization_selection_required",
      organizations: [{ id: 9, name: "Träger Nord", token: "must-not-leak" }],
      ...pair(),
    }),
  );
  const response = await createPortalLoginRoute(
    "traeger",
    "login",
  )(request("traeger"));
  expect(await response.json()).toEqual({
    status: "organization_selection_required",
    organizations: [{ id: 9, name: "Träger Nord" }],
  });
  expect(response.headers.getSetCookie()).toEqual([]);
});

describe.each([
  ["parents", "parent", "parent", "parent.session-token", parentHandlers],
  ["messe-demo", "lead", "", "authjs.session-token", tenantHandlers],
] as const)("%s demo session", (host, role, scope, cookieName, handlers) => {
  it("installs the host's real session and returns allowlisted visit metadata only", async () => {
    backend
      .mockResolvedValueOnce(
        Response.json({
          ...pair(),
          demo: {
            access_id: "4711",
            role,
            src: "fair",
            fixed_role: true,
            secret: "must-not-leak",
          },
        }),
      )
      .mockResolvedValueOnce(Response.json(claims(scope)));
    const response = await demo(
      request(host, { token: "demo-link-proof", role }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      session_created: true,
      demo: { access_id: "4711", role, src: "fair", fixed_role: true },
    });
    expect(response.headers.getSetCookie()).toContainEqual(
      expect.stringMatching(new RegExp(`${cookieName}=.*HttpOnly`)),
    );
    expect(response.headers.getSetCookie()).toContainEqual(
      expect.stringMatching(
        /moto-demo-token=demo-link-proof;.*HttpOnly;.*SameSite=strict/i,
      ),
    );
    const session = await handlers.GET(
      new NextRequest(
        `http://${state.host}/api/${host === "parents" ? "parent/" : ""}auth/session`,
        { headers: { cookie: browserCookie(response) } },
      ),
    );
    expect((await session.json()).user.id).toBe("42");
  });
  it("returns fixed-role handoff metadata without a wrong-portal session", async () => {
    const otherRole = role === "parent" ? "all" : "parent";
    backend
      .mockResolvedValueOnce(
        Response.json({
          ...pair(),
          demo: { access_id: "4711", role: otherRole, fixed_role: true },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const response = await demo(
      request(host, { token: "demo-link-proof", role }),
    );
    expect(await response.json()).toEqual({
      session_created: false,
      demo: { access_id: "4711", role: otherRole, fixed_role: true },
    });
    expect(backend.mock.calls[1]?.[0]).toBe("http://server:8080/auth/logout");
    expect(response.headers.getSetCookie().join(";")).not.toContain(
      "session-token=",
    );
  });
  it("rejects foreign origins before redeeming a link", async () => {
    const response = await demo(
      request(
        host,
        { token: "demo-link-proof", role },
        "http://foreign.localhost:3000",
      ),
    );
    expect(response.status).toBe(403);
    expect(backend).not.toHaveBeenCalled();
  });
});

it.each(["operator", "schule", "traeger", "caterer"])(
  "refuses demo session minting on the %s host",
  async (host) => {
    const response = await demo(
      request(host, { token: "demo-link-proof", role: "lead" }),
    );
    expect(response.status).toBe(403);
    expect(backend).not.toHaveBeenCalled();
  },
);
it("selects the parent demo cookie using the proxy-owned original host", async () => {
  backend
    .mockResolvedValueOnce(
      Response.json({ ...pair(), demo: { access_id: "4711", role: "parent" } }),
    )
    .mockResolvedValueOnce(Response.json(claims("parent")));
  const req = request("parents", { token: "demo-link-proof", role: "lead" });
  req.headers.set("host", "internal.localhost:3000");
  req.headers.set("x-moto-original-host", "parents.localhost:3000");
  const response = await demo(req);
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({
    session_created: true,
    demo: { access_id: "4711", role: "parent" },
  });
  expect(response.headers.getSetCookie()).toContainEqual(
    expect.stringMatching(/parent\.session-token=.*HttpOnly/),
  );
});
it("keeps demo session validation and rollback failures token-free", async () => {
  backend
    .mockResolvedValueOnce(
      Response.json({ ...pair(), demo: { access_id: "4711", role: "parent" } }),
    )
    .mockResolvedValueOnce(new Response(null, { status: 401 }))
    .mockRejectedValueOnce(new Error("offline"));
  const response = await demo(
    request("parents", { token: "demo-link-proof", role: "parent" }),
  );
  expect(response.status).toBe(500);
  expect(response.headers.getSetCookie()).toEqual([]);
  const body = await response.text();
  expect(body).toContain("general.server");
  expect(body).not.toContain(pair().access_token);
  expect(body).not.toContain(pair().refresh_token);
});
