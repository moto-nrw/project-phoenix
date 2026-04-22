import { describe, it, expect, vi } from "vitest";
import { NextRequest } from "next/server";

const OPERATOR_HOSTNAME = "operator.localhost:3000";

vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
vi.stubEnv("TENANT_DOMAIN", "localhost");

// Import after env is stubbed — proxy reads the env var at module load
const { proxy } = await import("./proxy");

function makeRequest(url: string, host?: string): NextRequest {
  const req = new NextRequest(url);
  if (host) {
    req.headers.set("host", host);
  }
  return req;
}

describe("proxy env validation", () => {
  it("throws when NEXT_PUBLIC_OPERATOR_HOSTNAME is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "");
    vi.stubEnv("TENANT_DOMAIN", "localhost");

    await expect(
      // @ts-expect-error — query string forces fresh module evaluation
      import("./proxy?missing-operator"),
    ).rejects.toThrow("NEXT_PUBLIC_OPERATOR_HOSTNAME is not set");

    // Restore for subsequent imports
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
  });

  it("throws when TENANT_DOMAIN is missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
    vi.stubEnv("TENANT_DOMAIN", "");

    await expect(
      // @ts-expect-error — query string forces fresh module evaluation
      import("./proxy?missing-tenant"),
    ).rejects.toThrow("TENANT_DOMAIN is not set");

    // Restore
    vi.stubEnv("TENANT_DOMAIN", "localhost");
  });
});

describe("proxy", () => {
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

    // Note: favicon.ico, favicon.png, apple-touch-icon.png, site.webmanifest,
    // icons/, and images/ are excluded from proxy via the matcher config.
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
    ])("rewrites %s to /operator%s", (path) => {
      const res = proxy(
        makeRequest(`http://${OPERATOR_HOSTNAME}${path}`, OPERATOR_HOSTNAME),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain(`/operator${path}`);
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
      // "www" is in RESERVED_SLUGS — extractTenantSlug returns null,
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
  });
});
