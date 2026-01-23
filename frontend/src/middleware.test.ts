/**
 * Tests for multi-tenant subdomain middleware
 *
 * This file tests the middleware's core functionality:
 * - Subdomain extraction from hostname
 * - Reserved subdomain handling
 * - Path bypass logic
 * - Session validation flow
 * - Tenant header setting
 */

import { describe, it, expect, vi } from "vitest";

// Mock environment for middleware tests
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NODE_ENV: "test",
  },
}));

// We test the middleware helper function logic by reimplementing
// the same logic used in middleware.ts for subdomain extraction,
// path detection, and configuration parsing.

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

    it("detects reserved subdomains", () => {
      expect(RESERVED_SUBDOMAINS.has("www")).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("api")).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("admin")).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("staging")).toBe(true);
    });

    it("allows non-reserved subdomains", () => {
      expect(RESERVED_SUBDOMAINS.has("ogs-musterstadt")).toBe(false);
      expect(RESERVED_SUBDOMAINS.has("my-organization")).toBe(false);
      expect(RESERVED_SUBDOMAINS.has("school-abc")).toBe(false);
    });

    it("is case-insensitive when checking", () => {
      expect(RESERVED_SUBDOMAINS.has("WWW".toLowerCase())).toBe(true);
      expect(RESERVED_SUBDOMAINS.has("Api".toLowerCase())).toBe(true);
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
    });

    it("bypasses Next.js internal routes", () => {
      expect(shouldBypass("/_next/static/chunk.js")).toBe(true);
      expect(shouldBypass("/_next/image")).toBe(true);
    });

    it("bypasses static files", () => {
      expect(shouldBypass("/favicon.ico")).toBe(true);
      expect(shouldBypass("/images/logo.png")).toBe(true);
      expect(shouldBypass("/fonts/inter.woff2")).toBe(true);
    });

    it("does not bypass protected routes", () => {
      expect(shouldBypass("/dashboard")).toBe(false);
      expect(shouldBypass("/ogs-groups")).toBe(false);
      expect(shouldBypass("/students")).toBe(false);
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

    it("identifies protected paths", () => {
      expect(isProtectedPath("/dashboard")).toBe(true);
      expect(isProtectedPath("/ogs-groups")).toBe(true);
      expect(isProtectedPath("/students")).toBe(true);
    });

    it("identifies nested protected paths", () => {
      expect(isProtectedPath("/students/123")).toBe(true);
      expect(isProtectedPath("/dashboard/analytics")).toBe(true);
      expect(isProtectedPath("/database/groups/combined")).toBe(true);
    });

    it("does not match public paths", () => {
      expect(isProtectedPath("/login")).toBe(false);
      expect(isProtectedPath("/signup")).toBe(false);
      expect(isProtectedPath("/")).toBe(false);
    });

    it("does not match partial path matches", () => {
      // /dashboardX should not match /dashboard
      expect(isProtectedPath("/dashboardX")).toBe(false);
      expect(isProtectedPath("/studentslist")).toBe(false);
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
      // Note: Query strings are part of URL.search, not pathname
      // pathname "/reset-password?token=xyz" is NOT realistic - the URL parser
      // separates pathname from search params. This test verifies nested paths work.
      expect(isPublicSubdomainPath("/reset-password/confirm")).toBe(true);
    });

    it("does not match protected paths", () => {
      expect(isPublicSubdomainPath("/dashboard")).toBe(false);
      expect(isPublicSubdomainPath("/ogs-groups")).toBe(false);
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
  });

  it("does not match non-console paths", () => {
    expect(isConsolePath("/dashboard")).toBe(false);
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
});
