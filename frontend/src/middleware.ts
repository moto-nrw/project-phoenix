import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * Multi-tenant subdomain middleware for Project Phoenix.
 *
 * Detects organization subdomains (e.g., ogs-musterstadt.moto-app.de) and:
 * 1. Validates the org exists and is active via BetterAuth API
 * 2. Sets tenant context headers for downstream use
 * 3. Redirects to main domain if org is invalid/pending/suspended
 *
 * Flow:
 * - Main domain (moto-app.de): No tenant context, show landing/login
 * - Org subdomain (slug.moto-app.de): Validate org, set context, or redirect
 */

// Reserved subdomains that should never be treated as org slugs
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

// Paths that should bypass tenant validation entirely
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

// Get base domain from environment (e.g., "moto-app.de" or "localhost:3000")
function getBaseDomain(): string {
  return process.env.NEXT_PUBLIC_BASE_DOMAIN ?? "localhost:3000";
}

// Extract subdomain from hostname
function extractSubdomain(hostname: string): string | null {
  const baseDomain = getBaseDomain();

  // Handle localhost development (e.g., "slug.localhost:3000")
  if (baseDomain.startsWith("localhost")) {
    const parts = hostname.split(".");
    if (parts.length >= 2 && parts[parts.length - 1]?.startsWith("localhost")) {
      const subdomain = parts.slice(0, -1).join(".");
      return subdomain || null;
    }
    return null;
  }

  // Production: extract subdomain from "slug.moto-app.de"
  const baseParts = baseDomain.split(".");
  const hostParts = hostname.replace(/:\d+$/, "").split("."); // Remove port if present

  // Hostname must have more parts than base domain to have a subdomain
  if (hostParts.length <= baseParts.length) {
    return null;
  }

  // Extract subdomain (everything before the base domain)
  const subdomainParts = hostParts.slice(
    0,
    hostParts.length - baseParts.length,
  );
  const subdomain = subdomainParts.join(".");

  return subdomain || null;
}

// Check if path should bypass middleware
function shouldBypass(pathname: string): boolean {
  return PUBLIC_PATHS.some((path) => pathname.startsWith(path));
}

// Validate organization via BetterAuth API
async function validateOrganization(
  slug: string,
  _request: NextRequest,
): Promise<{
  valid: boolean;
  status?: string;
  orgId?: string;
  orgName?: string;
}> {
  try {
    // Call the BetterAuth org lookup endpoint directly (not through Next.js proxy)
    // This avoids self-referential calls from middleware
    const betterAuthInternalUrl =
      process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";
    const response = await fetch(
      `${betterAuthInternalUrl}/api/auth/org/by-slug/${encodeURIComponent(slug)}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (!response.ok) {
      if (response.status === 404) {
        return { valid: false };
      }
      console.error(
        `Org lookup failed for slug "${slug}":`,
        response.status,
        await response.text(),
      );
      return { valid: false };
    }

    const org = (await response.json()) as {
      id: string;
      name: string;
      slug: string;
      status: string;
    };

    // Only allow active organizations
    if (org.status !== "active") {
      return { valid: false, status: org.status };
    }

    return {
      valid: true,
      status: org.status,
      orgId: org.id,
      orgName: org.name,
    };
  } catch (error) {
    console.error(`Failed to validate org "${slug}":`, error);
    return { valid: false };
  }
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hostname = request.headers.get("host") ?? "";

  // Bypass for static assets and API routes
  if (shouldBypass(pathname)) {
    return NextResponse.next();
  }

  // Extract subdomain
  const subdomain = extractSubdomain(hostname);

  // No subdomain - main domain request
  if (!subdomain) {
    // Clear any stale tenant headers and continue
    const response = NextResponse.next();
    response.headers.delete("x-tenant-slug");
    response.headers.delete("x-tenant-id");
    response.headers.delete("x-tenant-name");
    return response;
  }

  // Reserved subdomain - treat as main domain
  if (RESERVED_SUBDOMAINS.has(subdomain.toLowerCase())) {
    const response = NextResponse.next();
    response.headers.delete("x-tenant-slug");
    return response;
  }

  // Validate organization for the subdomain
  const orgResult = await validateOrganization(subdomain, request);

  if (!orgResult.valid) {
    // Invalid or non-active org - redirect to main domain
    const baseDomain = getBaseDomain();
    const protocol = request.nextUrl.protocol;
    const mainDomainUrl = new URL(`${protocol}//${baseDomain}`);

    // Add a query param to show appropriate message
    if (orgResult.status === "pending") {
      mainDomainUrl.searchParams.set("org_status", "pending");
      mainDomainUrl.searchParams.set("slug", subdomain);
    } else if (orgResult.status === "suspended") {
      mainDomainUrl.searchParams.set("org_status", "suspended");
    } else {
      mainDomainUrl.searchParams.set("org_status", "not_found");
    }

    return NextResponse.redirect(mainDomainUrl);
  }

  // Valid organization - set tenant context headers
  const response = NextResponse.next();
  response.headers.set("x-tenant-slug", subdomain);
  if (orgResult.orgId) {
    response.headers.set("x-tenant-id", orgResult.orgId);
  }
  if (orgResult.orgName) {
    response.headers.set("x-tenant-name", orgResult.orgName);
  }

  return response;
}

// Configure which paths the middleware runs on
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public folder files
     */
    "/((?!_next/static|_next/image|favicon.ico|images/|fonts/).*)",
  ],
};
