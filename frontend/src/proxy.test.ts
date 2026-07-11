import { describe, it, expect, vi } from "vitest";
import { NextRequest } from "next/server";
import { LOCALE_SCOPE_HEADER } from "~/i18n/locales";

const OPERATOR_HOSTNAME = "operator.localhost:3000";
const PARENTS_HOSTNAME = "parents.localhost:3000";

vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", PARENTS_HOSTNAME);
vi.stubEnv("TENANT_DOMAIN", "localhost");

// Import after env is stubbed , proxy reads the env var at module load
const { proxy } = await import("./proxy");

function makeRequest(url: string, host?: string): NextRequest {
  const req = new NextRequest(url);
  if (host) {
    req.headers.set("host", host);
  }
  return req;
}

function getForwardedRequestHeader(
  res: Response,
  headerName: string,
): string | null {
  return res.headers.get(`x-middleware-request-${headerName}`);
}

describe("proxy env validation", () => {
  it("throws when NEXT_PUBLIC_OPERATOR_HOSTNAME is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "");
    vi.stubEnv("TENANT_DOMAIN", "localhost");

    await expect(
      // @ts-expect-error , query string forces fresh module evaluation
      import("./proxy?missing-operator"),
    ).rejects.toThrow("NEXT_PUBLIC_OPERATOR_HOSTNAME is not set");

    // Restore for subsequent imports
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", PARENTS_HOSTNAME);
  });

  it("throws when TENANT_DOMAIN is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", PARENTS_HOSTNAME);
    vi.stubEnv("TENANT_DOMAIN", "");

    await expect(
      // @ts-expect-error , query string forces fresh module evaluation
      import("./proxy?missing-tenant"),
    ).rejects.toThrow("TENANT_DOMAIN is not set");

    // Restore
    vi.stubEnv("TENANT_DOMAIN", "localhost");
  });
});

describe("proxy", () => {
  it("preserves x-forwarded-host as the original request host", () => {
    const req = new NextRequest(
      "http://next-internal:3000/api/auth/passkeys/login/options",
    );
    req.headers.set("host", "next-internal:3000");
    req.headers.set("x-forwarded-host", "school.localhost:3000");

    const res = proxy(req);

    expect(getForwardedRequestHeader(res, "x-moto-original-host")).toBe(
      "school.localhost:3000",
    );
  });

  it("overwrites a client-forged original host with the resolved tenant host", () => {
    const req = new NextRequest(
      "http://school.localhost:3000/api/enrollment/form-bootstrap/public/school/5",
    );
    req.headers.set("host", "school.localhost:3000");
    req.headers.set("x-moto-original-host", PARENTS_HOSTNAME);

    const res = proxy(req);

    expect(getForwardedRequestHeader(res, "x-moto-original-host")).toBe(
      "school.localhost:3000",
    );
  });

  describe("operator subdomain", () => {
    it("rewrites / to /operator", () => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}/`, OPERATOR_HOSTNAME),
      );

      // Rewrites set x-middleware-rewrite header
      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator");
    });

    it("rewrites /login to /operator/login", () => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}/login`, OPERATOR_HOSTNAME),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/login");
    });

    it("rewrites /suggestions to /operator/suggestions", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/suggestions`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/suggestions");
    });

    it("rewrites nested operator paths like /suggestions/123", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/suggestions/123`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/suggestions/123");
    });

    it("returns 404 for tenant /api/auth/* routes on operator host", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/api/auth/session`,
          OPERATOR_HOSTNAME,
        ),
      );

      expect(res.status).toBe(404);
      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through /api/operator/auth/* routes without rewriting", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/api/operator/auth/session`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(res.status).toBe(200);
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    // Note: favicon.ico, favicon.png, apple-touch-icon.png, manifests,
    // favicons/, icons/, and images/ are excluded from proxy via the matcher config.
    // They never reach the proxy function in production.

    it("uses x-forwarded-host header when present", () => {
      const req = new NextRequest(`http://internal-host/suggestions`);
      req.headers.set("x-forwarded-host", OPERATOR_HOSTNAME);

      const res = proxy(req);

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/suggestions");
    });

    it("rewrites /announcements to /operator/announcements", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/announcements`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/announcements");
    });

    it("rewrites /settings to /operator/settings", () => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}/settings`, OPERATOR_HOSTNAME),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/settings");
    });

    it("rewrites /provisioning to /operator/provisioning", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/provisioning`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/provisioning");
    });

    it.each([
      "/organizations",
      "/schools",
      "/accounts",
      "/devices",
      "/persons",
      "/unregistered-tags",
    ])("rewrites %s to /operator%s", (path) => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}${path}`, OPERATOR_HOSTNAME),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain(`/operator${path}`);
    });

    it("keeps unregistered tag filter query params on clean operator URLs", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/unregistered-tags?resolved=all&organization_id=7`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain(
        "/operator/unregistered-tags?resolved=all&organization_id=7",
      );
    });

    it("passes through /_next routes", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/_next/static/chunk.js`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through paths already prefixed with /operator", () => {
      const res = proxy(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/operator/suggestions`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("redirects unknown paths like /dashboard to /", () => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}/dashboard`, OPERATOR_HOSTNAME),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(new URL(redirect!).pathname).toBe("/");
    });
  });

  describe("parents subdomain", () => {
    it("redirects the legacy deployed parents host to the canonical parents host", async () => {
      vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.moto-app.de");
      vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
      vi.stubEnv("TENANT_DOMAIN", "moto-app.de");

      try {
        const { proxy: deployedProxy } = await import(
          // @ts-expect-error query string forces fresh module evaluation
          "./proxy?legacy-parents-host"
        );

        const res = deployedProxy(
          makeRequest(
            "https://parents.moto-app.de:3000/accept-guardian-invite/abc?source=email",
            "parents.moto-app.de",
          ),
        );

        expect(res.status).toBe(302);
        expect(res.headers.get("location")).toBe(
          "https://eltern.moto-app.de/accept-guardian-invite/abc?source=email",
        );
        expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
      } finally {
        vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
        vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", PARENTS_HOSTNAME);
        vi.stubEnv("TENANT_DOMAIN", "localhost");
      }
    });

    it("keeps local parents.localhost on the normal parents host path", () => {
      const res = proxy(
        makeRequest(`http://${PARENTS_HOSTNAME}/login`, PARENTS_HOSTNAME),
      );

      expect(res.headers.get("location")).toBeNull();
      expect(res.headers.get("x-middleware-rewrite")).toContain(
        "/parents/login",
      );
    });

    it("rewrites clean parent password reset links to the parents app", () => {
      const res = proxy(
        makeRequest(
          `http://${PARENTS_HOSTNAME}/reset-password?token=abc`,
          PARENTS_HOSTNAME,
        ),
      );

      expect(res.headers.get("location")).toBeNull();
      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/parents/reset-password");
      expect(rewrite).toContain("token=abc");
      expect(getForwardedRequestHeader(res, LOCALE_SCOPE_HEADER)).toBe("1");
    });

    it("rewrites the clean /meal-plan path to the parents app", () => {
      const res = proxy(
        makeRequest(`http://${PARENTS_HOSTNAME}/meal-plan`, PARENTS_HOSTNAME),
      );

      // Without /meal-plan in the parents allowlist this path falls through to
      // the "redirect to root" branch and the meal plan becomes unreachable
      // from the desktop sidebar (which links to the clean /meal-plan).
      expect(res.headers.get("location")).toBeNull();
      expect(res.headers.get("x-middleware-rewrite")).toContain(
        "/parents/meal-plan",
      );
    });
  });

  describe("tenant subdomain", () => {
    const TENANT_HOST = "localhost:3000";

    it("redirects /operator/* to operator subdomain with clean path", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_HOST}/operator/suggestions`, TENANT_HOST),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(redirect).toContain("/suggestions");
      expect(redirect).not.toContain("/operator/suggestions");
    });

    it("redirects /operator to operator subdomain root /", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_HOST}/operator`, TENANT_HOST),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(new URL(redirect!).pathname).toBe("/");
    });

    it("passes through non-operator paths", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_HOST}/dashboard`, TENANT_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through /api/* routes", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_HOST}/api/students`, TENANT_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("skips /_next routes without security headers on non-operator host", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_HOST}/_next/static/chunk.js`, TENANT_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
      // /_next early return skips withSecurityHeaders
      expect(res.headers.get("Content-Security-Policy")).toBeNull();
    });
  });

  describe("tenant subdomain routing", () => {
    const TENANT_SUBDOMAIN_HOST = "school-a.localhost:3000";

    it("rewrites /dashboard to /school-a/dashboard on tenant subdomain", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/dashboard`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/school-a/dashboard");
    });

    it("rewrites root / to /school-a on tenant subdomain", () => {
      const res = proxy(
        makeRequest(`http://${TENANT_SUBDOMAIN_HOST}/`, TENANT_SUBDOMAIN_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/school-a");
    });

    it("rewrites /login to the tenant root login page", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/login`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(new URL(rewrite!).pathname).toBe("/school-a");
    });

    it("skips rewrite when path already has tenant prefix with slash", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/school-a/dashboard`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toBeNull();
      // Still gets security headers
      expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
    });

    it("localizes tenant-prefixed enrollment paths on tenant subdomains", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/school-a/enroll/phase-1`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
      expect(getForwardedRequestHeader(res, LOCALE_SCOPE_HEADER)).toBe("1");
    });

    it("skips rewrite when path is exactly the tenant slug", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/school-a`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toBeNull();
    });

    it("returns null slug for reserved subdomains", () => {
      // "www" is in RESERVED_SLUGS , extractTenantSlug returns null,
      // so it passes through as a bare domain (no rewrite).
      const res = proxy(
        makeRequest(
          `http://www.localhost:3000/dashboard`,
          "www.localhost:3000",
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toBeNull();
    });

    it("returns null slug for the reserved 'help' subdomain so it never rewrites to /help/*", () => {
      // A tenant slug of "help" would collide with the public /help docs route:
      // help.localhost/dashboard would rewrite to /help/dashboard, where the
      // static app/help segment shadows [tenant]. Reserving "help" prevents it.
      const res = proxy(
        makeRequest(
          `http://help.localhost:3000/dashboard`,
          "help.localhost:3000",
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toBeNull();
    });

    it("attaches security headers to rewritten tenant responses", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/dashboard`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
      expect(res.headers.get("X-Content-Type-Options")).toBe("nosniff");
      expect(res.headers.get("X-Frame-Options")).toBe("DENY");
    });

    it("handles bare domain without subdomain as passthrough", () => {
      const res = proxy(
        makeRequest(`http://localhost:3000/some-page`, "localhost:3000"),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("localizes tenant-prefixed enrollment paths on the bare domain", () => {
      const res = proxy(
        makeRequest(
          `http://localhost:3000/school-a/enroll/phase-1`,
          "localhost:3000",
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
      expect(getForwardedRequestHeader(res, LOCALE_SCOPE_HEADER)).toBe("1");
    });

    it("does not localize tenant-prefixed staff paths on the bare domain", () => {
      const res = proxy(
        makeRequest(
          `http://localhost:3000/school-a/dashboard`,
          "localhost:3000",
        ),
      );

      expect(getForwardedRequestHeader(res, LOCALE_SCOPE_HEADER)).toBeNull();
    });
  });

  // The public /help guide must stay host-agnostic: served as-is on every host
  // with security headers, never rewritten to a tenant/operator/parents segment
  // (no such route exists there) and never blocked.
  describe("public help docs", () => {
    const TENANT_SUBDOMAIN_HOST = "school-a.localhost:3000";

    it("serves /help on a tenant subdomain without rewriting to /{tenant}/help", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/help`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      expect(res.headers.get("x-middleware-rewrite")).toBeNull();
      expect(res.headers.get("location")).toBeNull();
      expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
    });

    it("serves nested /help/* on a tenant subdomain without rewriting", () => {
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/help/setup`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      expect(res.headers.get("x-middleware-rewrite")).toBeNull();
      expect(res.headers.get("location")).toBeNull();
    });

    it("serves /help on the operator host without rewriting to /operator or blocking", () => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}/help`, OPERATOR_HOSTNAME),
      );

      expect(res.status).not.toBe(404);
      expect(res.headers.get("x-middleware-rewrite")).toBeNull();
      expect(res.headers.get("location")).toBeNull();
      expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
    });

    it("serves /help/nfc on the parents host without rewriting to /parents or blocking", () => {
      const res = proxy(
        makeRequest(`http://${PARENTS_HOSTNAME}/help/nfc`, PARENTS_HOSTNAME),
      );

      expect(res.status).not.toBe(404);
      expect(res.headers.get("x-middleware-rewrite")).toBeNull();
      expect(res.headers.get("location")).toBeNull();
      expect(res.headers.get("Content-Security-Policy")).toBeTruthy();
    });

    it("does not treat a non-/help path containing 'help' as public docs", () => {
      // Guards against a `.includes('help')`-style mistake: /helpdesk on a
      // tenant subdomain must still rewrite to the tenant segment.
      const res = proxy(
        makeRequest(
          `http://${TENANT_SUBDOMAIN_HOST}/helpdesk`,
          TENANT_SUBDOMAIN_HOST,
        ),
      );

      expect(res.headers.get("x-middleware-rewrite")).toContain(
        "/school-a/helpdesk",
      );
    });
  });
});
