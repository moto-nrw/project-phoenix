/**
 * Tests for multi-tenant subdomain middleware
 *
 * This file tests the middleware's core functionality:
 * - Subdomain extraction from hostname
 * - Reserved subdomain handling
 * - Path bypass logic
 * - Session validation flow
 * - Tenant header setting
 * - Console access control
 * - Protected path redirects
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { NextRequest } from "next/server";

// Mock environment for middleware tests
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NODE_ENV: "test",
  },
}));

// ============================================================================
// PART 1: Helper function unit tests (logic reimplemented for isolation)
// ============================================================================

describe("middleware helper functions", () => {
  describe("subdomain extraction logic", () => {
    // Testing the logic that would be in extractSubdomain
    it("extracts subdomain from localhost development URL", () => {
      // For localhost: slug.localhost:3000 -> slug
      const hostname = "ogs-test.localhost:3000";
      // baseDomain would be "localhost:3000" in the actual implementation

      const parts = hostname.split(".");
      const hasSubdomain =
        parts.length >= 2 && parts[parts.length - 1]?.startsWith("localhost");
      const subdomain = hasSubdomain ? parts.slice(0, -1).join(".") : null;

      expect(subdomain).toBe("ogs-test");
    });

    it("returns null for main localhost domain", () => {
      const hostname = "localhost:3000";
      const parts = hostname.split(".");

      // Only one part, no subdomain
      const hasSubdomain = parts.length >= 2;
      expect(hasSubdomain).toBe(false);
    });

    it("extracts subdomain from production URL", () => {
      // For production: slug.moto-app.de -> slug
      const hostname = "ogs-musterstadt.moto-app.de";
      const baseDomain = "moto-app.de";

      const baseParts = baseDomain.split(".");
      const hostParts = hostname.replace(/:\d+$/, "").split(".");

      // Hostname must have more parts than base domain
      const hasSubdomain = hostParts.length > baseParts.length;
      const subdomainParts = hostParts.slice(
        0,
        hostParts.length - baseParts.length,
      );
      const subdomain = subdomainParts.join(".");

      expect(hasSubdomain).toBe(true);
      expect(subdomain).toBe("ogs-musterstadt");
    });

    it("handles multiple subdomain levels", () => {
      const hostname = "sub1.sub2.moto-app.de";
      const baseDomain = "moto-app.de";

      const baseParts = baseDomain.split(".");
      const hostParts = hostname.replace(/:\d+$/, "").split(".");
      const subdomainParts = hostParts.slice(
        0,
        hostParts.length - baseParts.length,
      );
      const subdomain = subdomainParts.join(".");

      expect(subdomain).toBe("sub1.sub2");
    });

    it("returns null for exact base domain match", () => {
      const hostname = "moto-app.de";
      const baseDomain = "moto-app.de";

      const baseParts = baseDomain.split(".");
      const hostParts = hostname.replace(/:\d+$/, "").split(".");

      expect(hostParts.length).toBe(baseParts.length);
    });

    it("strips port from hostname when extracting subdomain", () => {
      const hostname = "ogs-test.moto-app.de:8080";
      const baseDomain = "moto-app.de";

      const baseParts = baseDomain.split(".");
      const hostParts = hostname.replace(/:\d+$/, "").split(".");
      const subdomainParts = hostParts.slice(
        0,
        hostParts.length - baseParts.length,
      );
      const subdomain = subdomainParts.join(".");

      expect(subdomain).toBe("ogs-test");
    });

    it("handles empty string subdomain", () => {
      // Edge case: ".moto-app.de" has empty first part
      const hostname = ".moto-app.de";
      const baseDomain = "moto-app.de";

      const baseParts = baseDomain.split(".");
      const hostParts = hostname.replace(/:\d+$/, "").split(".");
      const subdomainParts = hostParts.slice(
        0,
        hostParts.length - baseParts.length,
      );
      const subdomain = subdomainParts.join("") || null;

      expect(subdomain).toBe(null);
    });
  });

  describe("reserved subdomain detection", () => {
    const RESERVED_SUBDOMAINS = new Set([
      "www",
      "api",
      "auth",
      "admin",
      "app",
      "mail",
      "smtp",
      "ftp",
      "cdn",
      "static",
      "assets",
      "staging",
      "dev",
      "test",
      "demo",
      "beta",
      "preview",
    ]);

    it("detects all reserved subdomains", () => {
      const expected = [
        "www",
        "api",
        "auth",
        "admin",
        "app",
        "mail",
        "smtp",
        "ftp",
        "cdn",
        "static",
        "assets",
        "staging",
        "dev",
        "test",
        "demo",
        "beta",
        "preview",
      ];
      expected.forEach((subdomain) => {
        expect(RESERVED_SUBDOMAINS.has(subdomain)).toBe(true);
      });
    });

    it("allows non-reserved subdomains", () => {
      expect(RESERVED_SUBDOMAINS.has("ogs-musterstadt")).toBe(false);
      expect(RESERVED_SUBDOMAINS.has("my-organization")).toBe(false);
      expect(RESERVED_SUBDOMAINS.has("school-abc")).toBe(false);
      expect(RESERVED_SUBDOMAINS.has("customer-portal")).toBe(false);
    });

    it("is case-insensitive when checking", () => {
      expect(RESERVED_SUBDOMAINS.has("WWW".toLowerCase())).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("Api".toLowerCase())).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("STAGING".toLowerCase())).toBe(true);
    });
  });

  describe("path bypass logic", () => {
    const PUBLIC_PATHS = [
      "/api/",
      "/_next/",
      "/favicon.ico",
      "/images/",
      "/fonts/",
      "/manifest.json",
      "/robots.txt",
      "/sitemap.xml",
    ];

    function shouldBypass(pathname: string): boolean {
      return PUBLIC_PATHS.some((path) => pathname.startsWith(path));
    }

    it("bypasses API routes", () => {
      expect(shouldBypass("/api/auth/session")).toBe(true);
      expect(shouldBypass("/api/rooms")).toBe(true);
      expect(shouldBypass("/api/students/123")).toBe(true);
    });

    it("bypasses Next.js internal routes", () => {
      expect(shouldBypass("/_next/static/chunk.js")).toBe(true);
      expect(shouldBypass("/_next/image")).toBe(true);
      expect(shouldBypass("/_next/data/build-id/page.json")).toBe(true);
    });

    it("bypasses static files", () => {
      expect(shouldBypass("/favicon.ico")).toBe(true);
      expect(shouldBypass("/images/logo.png")).toBe(true);
      expect(shouldBypass("/fonts/inter.woff2")).toBe(true);
      expect(shouldBypass("/manifest.json")).toBe(true);
      expect(shouldBypass("/robots.txt")).toBe(true);
      expect(shouldBypass("/sitemap.xml")).toBe(true);
    });

    it("does not bypass protected routes", () => {
      expect(shouldBypass("/dashboard")).toBe(false);
      expect(shouldBypass("/ogs-groups")).toBe(false);
      expect(shouldBypass("/students")).toBe(false);
      expect(shouldBypass("/login")).toBe(false);
      expect(shouldBypass("/console")).toBe(false);
    });
  });

  describe("protected path detection", () => {
    const PROTECTED_PATHS = [
      "/ogs-groups",
      "/rooms",
      "/staff",
      "/students",
      "/activities",
      "/dashboard",
      "/database",
      "/invitations",
      "/settings",
      "/active-supervisions",
      "/substitutions",
    ];

    function isProtectedPath(pathname: string): boolean {
      return PROTECTED_PATHS.some(
        (path) => pathname === path || pathname.startsWith(path + "/"),
      );
    }

    it("identifies all protected paths", () => {
      PROTECTED_PATHS.forEach((path) => {
        expect(isProtectedPath(path)).toBe(true);
      });
    });

    it("identifies nested protected paths", () => {
      expect(isProtectedPath("/students/123")).toBe(true);
      expect(isProtectedPath("/dashboard/analytics")).toBe(true);
      expect(isProtectedPath("/database/groups/combined")).toBe(true);
      expect(isProtectedPath("/rooms/123/details")).toBe(true);
      expect(isProtectedPath("/activities/schedules")).toBe(true);
    });

    it("does not match public paths", () => {
      expect(isProtectedPath("/login")).toBe(false);
      expect(isProtectedPath("/signup")).toBe(false);
      expect(isProtectedPath("/")).toBe(false);
      expect(isProtectedPath("/invite")).toBe(false);
      expect(isProtectedPath("/reset-password")).toBe(false);
    });

    it("does not match partial path matches", () => {
      // /dashboardX should not match /dashboard
      expect(isProtectedPath("/dashboardX")).toBe(false);
      expect(isProtectedPath("/studentslist")).toBe(false);
      expect(isProtectedPath("/roomsview")).toBe(false);
    });
  });

  describe("SaaS admin email detection", () => {
    function getSaasAdminEmails(envValue: string | undefined): string[] {
      return (envValue ?? "admin@example.com")
        .split(",")
        .map((e) => e.trim().toLowerCase());
    }

    it("parses single admin email", () => {
      const emails = getSaasAdminEmails("admin@moto-app.de");
      expect(emails).toEqual(["admin@moto-app.de"]);
    });

    it("parses multiple admin emails", () => {
      const emails = getSaasAdminEmails(
        "admin1@moto-app.de,admin2@moto-app.de",
      );
      expect(emails).toEqual(["admin1@moto-app.de", "admin2@moto-app.de"]);
    });

    it("trims whitespace and normalizes to lowercase", () => {
      const emails = getSaasAdminEmails(" Admin@Example.com , USER@TEST.COM ");
      expect(emails).toEqual(["admin@example.com", "user@test.com"]);
    });

    it("uses default when env is undefined", () => {
      const emails = getSaasAdminEmails(undefined);
      expect(emails).toEqual(["admin@example.com"]);
    });

    it("handles three or more admin emails", () => {
      const emails = getSaasAdminEmails("a@x.com, b@x.com, c@x.com");
      expect(emails).toEqual(["a@x.com", "b@x.com", "c@x.com"]);
    });
  });

  describe("public subdomain paths", () => {
    const publicSubdomainPaths = [
      "/login",
      "/signup",
      "/invite",
      "/reset-password",
    ];

    function isPublicSubdomainPath(pathname: string): boolean {
      return publicSubdomainPaths.some(
        (path) => pathname === path || pathname.startsWith(path + "/"),
      );
    }

    it("identifies login as public", () => {
      expect(isPublicSubdomainPath("/login")).toBe(true);
    });

    it("identifies signup as public", () => {
      expect(isPublicSubdomainPath("/signup")).toBe(true);
    });

    it("identifies invite paths as public", () => {
      expect(isPublicSubdomainPath("/invite")).toBe(true);
      expect(isPublicSubdomainPath("/invite/abc123")).toBe(true);
    });

    it("identifies reset-password as public", () => {
      expect(isPublicSubdomainPath("/reset-password")).toBe(true);
      expect(isPublicSubdomainPath("/reset-password/confirm")).toBe(true);
    });

    it("does not match protected paths", () => {
      expect(isPublicSubdomainPath("/dashboard")).toBe(false);
      expect(isPublicSubdomainPath("/ogs-groups")).toBe(false);
    });

    it("does not match partial matches", () => {
      expect(isPublicSubdomainPath("/loginpage")).toBe(false);
      expect(isPublicSubdomainPath("/signupform")).toBe(false);
    });
  });
});

describe("console path handling", () => {
  function isConsolePath(pathname: string): boolean {
    return pathname === "/console" || pathname.startsWith("/console/");
  }

  it("identifies console login path", () => {
    expect(isConsolePath("/console/login")).toBe(true);
  });

  it("identifies console root path", () => {
    expect(isConsolePath("/console")).toBe(true);
  });

  it("identifies nested console paths", () => {
    expect(isConsolePath("/console/organizations")).toBe(true);
    expect(isConsolePath("/console/users")).toBe(true);
    expect(isConsolePath("/console/settings")).toBe(true);
  });

  it("does not match non-console paths", () => {
    expect(isConsolePath("/dashboard")).toBe(false);
    expect(isConsolePath("/consolepage")).toBe(false);
    expect(isConsolePath("/my-console")).toBe(false);
  });
});

describe("URL construction for redirects", () => {
  it("constructs main domain URL with protocol", () => {
    const baseDomain = "moto-app.de";
    const protocol = "https:";
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);

    expect(mainDomainUrl.href).toBe("https://moto-app.de/");
  });

  it("adds org_status query param for pending orgs", () => {
    const baseDomain = "moto-app.de";
    const protocol = "https:";
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);
    mainDomainUrl.searchParams.set("org_status", "pending");
    mainDomainUrl.searchParams.set("slug", "ogs-test");

    expect(mainDomainUrl.searchParams.get("org_status")).toBe("pending");
    expect(mainDomainUrl.searchParams.get("slug")).toBe("ogs-test");
  });

  it("adds org_status query param for suspended orgs", () => {
    const baseDomain = "moto-app.de";
    const protocol = "https:";
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);
    mainDomainUrl.searchParams.set("org_status", "suspended");

    expect(mainDomainUrl.searchParams.get("org_status")).toBe("suspended");
  });

  it("adds org_status query param for rejected orgs", () => {
    const baseDomain = "moto-app.de";
    const protocol = "https:";
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);
    mainDomainUrl.searchParams.set("org_status", "rejected");

    expect(mainDomainUrl.searchParams.get("org_status")).toBe("rejected");
  });

  it("adds org_status not_found for unknown orgs", () => {
    const baseDomain = "moto-app.de";
    const protocol = "https:";
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);
    mainDomainUrl.searchParams.set("org_status", "not_found");

    expect(mainDomainUrl.searchParams.get("org_status")).toBe("not_found");
  });
});

describe("tenant header management", () => {
  it("sets tenant headers for valid organization", () => {
    const headers = new Headers();
    headers.set("x-tenant-slug", "ogs-musterstadt");
    headers.set("x-tenant-id", "org-123");
    headers.set("x-tenant-name", "OGS Musterstadt");

    expect(headers.get("x-tenant-slug")).toBe("ogs-musterstadt");
    expect(headers.get("x-tenant-id")).toBe("org-123");
    expect(headers.get("x-tenant-name")).toBe("OGS Musterstadt");
  });

  it("clears tenant headers for main domain", () => {
    const headers = new Headers();
    headers.set("x-tenant-slug", "old-value");
    headers.delete("x-tenant-slug");
    headers.delete("x-tenant-id");
    headers.delete("x-tenant-name");

    expect(headers.get("x-tenant-slug")).toBeNull();
    expect(headers.get("x-tenant-id")).toBeNull();
    expect(headers.get("x-tenant-name")).toBeNull();
  });

  it("only sets id and name when available", () => {
    const headers = new Headers();
    const orgResult = {
      slug: "test-org",
      orgId: undefined,
      orgName: undefined,
    };

    headers.set("x-tenant-slug", orgResult.slug);
    if (orgResult.orgId) {
      headers.set("x-tenant-id", orgResult.orgId);
    }
    if (orgResult.orgName) {
      headers.set("x-tenant-name", orgResult.orgName);
    }

    expect(headers.get("x-tenant-slug")).toBe("test-org");
    expect(headers.get("x-tenant-id")).toBeNull();
    expect(headers.get("x-tenant-name")).toBeNull();
  });
});

// ============================================================================
// PART 2: Integration tests for actual middleware function
// ============================================================================

describe("middleware integration", () => {
  const originalFetch = global.fetch;
  const originalEnv = { ...process.env };

  // Helper to create mock NextRequest
  function createMockRequest(
    pathname: string,
    options: {
      host?: string;
      cookies?: string;
      protocol?: string;
    } = {},
  ): NextRequest {
    const { host = "localhost:3000", cookies, protocol = "https:" } = options;
    const url = new URL(pathname, `${protocol}//${host}`);

    const headers = new Headers();
    headers.set("host", host);
    if (cookies) {
      headers.set("Cookie", cookies);
    }

    return {
      nextUrl: url,
      headers,
      url: url.toString(),
    } as unknown as NextRequest;
  }

  // Create mock Response helper
  function createMockResponse(
    data: unknown,
    options: { status?: number; ok?: boolean } = {},
  ): Response {
    const { status = 200, ok = status >= 200 && status < 300 } = options;
    return {
      ok,
      status,
      json: () => Promise.resolve(data),
      text: () => Promise.resolve(JSON.stringify(data)),
    } as Response;
  }

  beforeEach(() => {
    vi.clearAllMocks();
    vi.resetModules();
    // Reset environment
    process.env.SAAS_ADMIN_EMAILS = "admin@example.com";
    process.env.NEXT_PUBLIC_BASE_DOMAIN = "localhost:3000";
    process.env.BETTERAUTH_INTERNAL_URL = "http://localhost:3001";
  });

  afterEach(() => {
    global.fetch = originalFetch;
    process.env = { ...originalEnv };
  });

  describe("public paths bypass", () => {
    it("should bypass middleware for /api/ routes", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/api/auth/session");
      const result = await middleware(request);

      // NextResponse.next() returns a response object (not a redirect)
      expect(result).toBeDefined();
      expect(result.status).not.toBe(307); // Not a redirect
    });

    it("should bypass middleware for /_next/ routes", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/_next/static/chunk.js");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for favicon.ico", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/favicon.ico");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for /images/ routes", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/images/logo.png");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for /fonts/ routes", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/fonts/inter.woff2");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for manifest.json", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/manifest.json");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for robots.txt", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/robots.txt");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });

    it("should bypass middleware for sitemap.xml", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/sitemap.xml");
      const result = await middleware(request);

      expect(result).toBeDefined();
    });
  });

  describe("main domain - login redirect", () => {
    it("should redirect /login to / on main domain", async () => {
      const { middleware } = await import("./middleware");
      const request = createMockRequest("/login", { host: "localhost:3000" });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      expect(result.headers.get("location")).toContain("/");
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/");
    });
  });

  describe("main domain - console access", () => {
    it("should allow unauthenticated access to /console/login", async () => {
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(null));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console/login", {
        host: "localhost:3000",
      });
      const result = await middleware(request);

      // Should NOT be a redirect - unauthenticated users can view login page
      expect(result.status).not.toBe(307);
    });

    it("should redirect unauthenticated users from /console to /console/login", async () => {
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(null));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", { host: "localhost:3000" });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/console/login");
    });

    it("should redirect unauthenticated users from /console/orgs to /console/login", async () => {
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(null));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console/orgs", {
        host: "localhost:3000",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/console/login");
    });

    it("should redirect authenticated SaaS admin from /console/login to /console", async () => {
      const sessionData = {
        user: { id: "user-1", email: "admin@example.com" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console/login", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/console");
    });

    it("should allow authenticated SaaS admin access to /console", async () => {
      const sessionData = {
        user: { id: "user-1", email: "admin@example.com" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect
      expect(result.status).not.toBe(307);
    });

    it("should redirect non-SaaS admin from /console to /console/login with error", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/console/login");
      expect(locationUrl.searchParams.get("error")).toBe("Unauthorized");
    });

    it("should handle case-insensitive email matching for SaaS admins", async () => {
      process.env.SAAS_ADMIN_EMAILS = "admin@example.com";
      const sessionData = {
        user: { id: "user-1", email: "ADMIN@EXAMPLE.COM" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect - case-insensitive match
      expect(result.status).not.toBe(307);
    });

    it("should support multiple SaaS admin emails", async () => {
      process.env.SAAS_ADMIN_EMAILS =
        "admin1@example.com, admin2@example.com, superadmin@example.com";
      const sessionData = {
        user: { id: "user-1", email: "admin2@example.com" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect - second admin email should work
      expect(result.status).not.toBe(307);
    });
  });

  describe("main domain - protected paths with org status", () => {
    it("should redirect to /signup/pending for pending org on protected path", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };
      const orgData = [{ id: "org-1", slug: "test-org", status: "pending" }];

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/signup/pending");
    });

    it("should redirect to /?org_status=rejected for rejected org", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };
      const orgData = [{ id: "org-1", slug: "test-org", status: "rejected" }];

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/");
      expect(locationUrl.searchParams.get("org_status")).toBe("rejected");
    });

    it("should redirect to /?org_status=suspended for suspended org", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };
      const orgData = [{ id: "org-1", slug: "test-org", status: "suspended" }];

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/");
      expect(locationUrl.searchParams.get("org_status")).toBe("suspended");
    });

    it("should allow access for active org on protected path", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };
      const orgData = [{ id: "org-1", slug: "test-org", status: "active" }];

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect
      expect(result.status).not.toBe(307);
    });

    it("should allow access when user has no org", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: null },
      };
      const orgData: unknown[] = [];

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect (no org = no status to check)
      expect(result.status).not.toBe(307);
    });
  });

  describe("reserved subdomains", () => {
    const reservedSubdomains = [
      "www",
      "api",
      "auth",
      "admin",
      "app",
      "mail",
      "cdn",
      "static",
      "staging",
      "dev",
      "test",
      "demo",
      "beta",
      "preview",
    ];

    it.each(reservedSubdomains)(
      "should treat %s subdomain as main domain",
      async (subdomain) => {
        process.env.NEXT_PUBLIC_BASE_DOMAIN = "moto-app.de";
        global.fetch = vi.fn().mockResolvedValue(createMockResponse(null));

        const { middleware } = await import("./middleware");
        const request = createMockRequest("/", {
          host: `${subdomain}.moto-app.de`,
        });
        const result = await middleware(request);

        // Should NOT call org validation (no redirect to main domain with org_status)
        expect(result.status).not.toBe(307);
      },
    );
  });

  describe("org subdomain validation", () => {
    beforeEach(() => {
      process.env.NEXT_PUBLIC_BASE_DOMAIN = "moto-app.de";
    });

    it("should redirect to main domain for non-existent org", async () => {
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(
            createMockResponse({ error: "Not found" }, { status: 404 }),
          );
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/", {
        host: "unknown-org.moto-app.de",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.hostname).toBe("moto-app.de");
      expect(locationUrl.searchParams.get("org_status")).toBe("not_found");
    });

    it("should redirect with pending status for pending org", async () => {
      const orgData = {
        id: "org-1",
        name: "Test Org",
        slug: "pending-org",
        status: "pending",
      };
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/", {
        host: "pending-org.moto-app.de",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.searchParams.get("org_status")).toBe("pending");
      expect(locationUrl.searchParams.get("slug")).toBe("pending-org");
    });

    it("should redirect with suspended status for suspended org", async () => {
      const orgData = {
        id: "org-1",
        name: "Test Org",
        slug: "suspended-org",
        status: "suspended",
      };
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/", {
        host: "suspended-org.moto-app.de",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.searchParams.get("org_status")).toBe("suspended");
    });

    it("should allow access to public paths on valid org subdomain", async () => {
      const orgData = {
        id: "org-1",
        name: "Test OGS",
        slug: "test-ogs",
        status: "active",
      };
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/login", {
        host: "test-ogs.moto-app.de",
      });
      const result = await middleware(request);

      // Should NOT be a redirect - public path
      expect(result.status).not.toBe(307);
    });

    it("should redirect to login for protected path without session on subdomain", async () => {
      const orgData = {
        id: "org-1",
        name: "Test OGS",
        slug: "test-ogs",
        status: "active",
      };
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(null));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "test-ogs.moto-app.de",
      });
      const result = await middleware(request);

      expect(result.status).toBe(307);
      const locationUrl = new URL(
        result.headers.get("location") ?? "",
        "http://localhost",
      );
      expect(locationUrl.pathname).toBe("/login");
    });

    it("should allow access to protected path with session on subdomain", async () => {
      const orgData = {
        id: "org-1",
        name: "Test OGS",
        slug: "test-ogs",
        status: "active",
      };
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("by-slug")) {
          return Promise.resolve(createMockResponse(orgData));
        }
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "test-ogs.moto-app.de",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should NOT be a redirect
      expect(result.status).not.toBe(307);
    });
  });

  describe("session handling edge cases", () => {
    it("should handle missing cookies gracefully", async () => {
      const { middleware } = await import("./middleware");
      // No cookies set
      const request = createMockRequest("/", { host: "localhost:3000" });
      const result = await middleware(request);

      // Should continue without error
      expect(result).toBeDefined();
    });

    it("should handle session API error gracefully", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        // Suppress console output
      });

      global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should redirect to login when session check fails
      expect(result.status).toBe(307);

      consoleSpy.mockRestore();
    });

    it("should handle non-200 session response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse({ error: "Unauthorized" }, { status: 401 }),
        );

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should redirect when session check fails
      expect(result.status).toBe(307);
    });

    it("should handle session response without user.id", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse({ user: { email: "test@test.com" } }),
        );

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should redirect when session is invalid
      expect(result.status).toBe(307);
    });
  });

  describe("org status handling edge cases", () => {
    it("should handle org list API error gracefully", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        // Suppress console output
      });

      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.reject(new Error("API error"));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should continue without blocking when org status check fails
      expect(result.status).not.toBe(307);

      consoleSpy.mockRestore();
    });

    it("should handle null org list response", async () => {
      const sessionData = {
        user: { id: "user-1", email: "user@example.com" },
        session: { activeOrganizationId: "org-1" },
      };

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes("get-session")) {
          return Promise.resolve(createMockResponse(sessionData));
        }
        if (url.includes("list-organizations")) {
          return Promise.resolve(createMockResponse(null));
        }
        return Promise.resolve(createMockResponse(null));
      });

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/dashboard", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Should continue when org list is null
      expect(result.status).not.toBe(307);
    });
  });

  describe("environment defaults", () => {
    it("should use default SAAS_ADMIN_EMAILS when env not set", async () => {
      delete process.env.SAAS_ADMIN_EMAILS;

      const sessionData = {
        user: { id: "user-1", email: "admin@example.com" },
        session: { activeOrganizationId: null },
      };
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(sessionData));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      const result = await middleware(request);

      // Default admin email should work
      expect(result.status).not.toBe(307);
    });

    it("should use default BETTERAUTH_INTERNAL_URL when env not set", async () => {
      delete process.env.BETTERAUTH_INTERNAL_URL;

      global.fetch = vi.fn().mockResolvedValue(createMockResponse(null));

      const { middleware } = await import("./middleware");
      const request = createMockRequest("/console", {
        host: "localhost:3000",
        cookies: "session=test",
      });
      await middleware(request);

      // Should call localhost:3001 (default)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining("localhost:3001"),
        expect.any(Object),
      );
    });
  });

  describe("config matcher", () => {
    it("should have correct matcher pattern", async () => {
      const { config } = await import("./middleware");
      expect(config.matcher).toEqual([
        "/((?!_next/static|_next/image|favicon.ico|images/|fonts/).*)",
      ]);
    });
  });
});
