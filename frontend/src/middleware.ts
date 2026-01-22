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
 * Also handles:
 * - SaaS admin redirects (main domain, admin email)
 * - Pending org protection (user logged in but org not approved)
 *
 * Flow:
 * - Main domain (moto-app.de): Check session, redirect SaaS admins or pending users
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

// Paths that should bypass all middleware checks
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

// Protected paths that require an active org (on main domain)
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

// SaaS admin emails (comma-separated in env)
function getSaasAdminEmails(): string[] {
  return (process.env.SAAS_ADMIN_EMAILS ?? "admin@example.com")
    .split(",")
    .map((e) => e.trim().toLowerCase());
}

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

// Get BetterAuth internal URL
function getBetterAuthUrl(): string {
  return process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";
}

// Get user session from BetterAuth
async function getUserSession(request: NextRequest): Promise<{
  authenticated: boolean;
  email?: string;
  userId?: string;
  activeOrgId?: string;
} | null> {
  try {
    const cookies = request.headers.get("Cookie");
    if (!cookies) {
      return null;
    }

    const response = await fetch(`${getBetterAuthUrl()}/api/auth/get-session`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Cookie: cookies,
      },
    });

    if (!response.ok) {
      return null;
    }

    const session = (await response.json()) as {
      user?: { id?: string; email?: string };
      session?: { activeOrganizationId?: string };
    } | null;

    if (!session?.user?.id) {
      return null;
    }

    return {
      authenticated: true,
      email: session.user.email,
      userId: session.user.id,
      activeOrgId: session.session?.activeOrganizationId,
    };
  } catch (error) {
    console.error("Failed to get user session:", error);
    return null;
  }
}

// Get user's active organization status
async function getUserOrgStatus(
  userId: string,
  request: NextRequest,
): Promise<{
  hasOrg: boolean;
  orgStatus?: string;
  orgSlug?: string;
} | null> {
  try {
    const cookies = request.headers.get("Cookie");

    // Get the user's organization memberships via BetterAuth
    // We need a custom endpoint for this - for now, use a simple approach
    const response = await fetch(
      `${getBetterAuthUrl()}/api/auth/organization/list-organizations`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          ...(cookies ? { Cookie: cookies } : {}),
        },
      },
    );

    if (!response.ok) {
      return null;
    }

    const data = (await response.json()) as Array<{
      id: string;
      slug: string;
      status?: string;
    }> | null;

    if (!data || data.length === 0) {
      return { hasOrg: false };
    }

    // Return the first org's status (user typically has one org after signup)
    const firstOrg = data[0];
    return {
      hasOrg: true,
      orgStatus: firstOrg?.status,
      orgSlug: firstOrg?.slug,
    };
  } catch (error) {
    console.error(`Failed to get org status for user ${userId}:`, error);
    return null;
  }
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
    const response = await fetch(
      `${getBetterAuthUrl()}/api/auth/org/by-slug/${encodeURIComponent(slug)}`,
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

// Check if path is protected (requires active org)
function isProtectedPath(pathname: string): boolean {
  return PROTECTED_PATHS.some(
    (path) => pathname === path || pathname.startsWith(path + "/"),
  );
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
    // Main domain: Check for pending org protection and SaaS admin access
    const session = await getUserSession(request);

    if (session?.authenticated && session.userId) {
      const saasAdminEmails = getSaasAdminEmails();
      const isSaasAdmin =
        session.email && saasAdminEmails.includes(session.email.toLowerCase());

      // SaaS admins can access /console
      if (pathname === "/console" || pathname.startsWith("/console/")) {
        if (isSaasAdmin) {
          // Allow access
          const response = NextResponse.next();
          response.headers.delete("x-tenant-slug");
          return response;
        } else {
          // Not a SaaS admin - redirect to home
          return NextResponse.redirect(new URL("/", request.url));
        }
      }

      // Check if user has a pending org and is trying to access protected routes
      if (isProtectedPath(pathname)) {
        const orgStatus = await getUserOrgStatus(session.userId, request);

        if (orgStatus?.hasOrg && orgStatus.orgStatus === "pending") {
          // Redirect to pending page
          return NextResponse.redirect(new URL("/signup/pending", request.url));
        }

        if (orgStatus?.hasOrg && orgStatus.orgStatus === "rejected") {
          // Redirect to home with rejection message
          const url = new URL("/", request.url);
          url.searchParams.set("org_status", "rejected");
          return NextResponse.redirect(url);
        }

        if (orgStatus?.hasOrg && orgStatus.orgStatus === "suspended") {
          // Redirect to home with suspension message
          const url = new URL("/", request.url);
          url.searchParams.set("org_status", "suspended");
          return NextResponse.redirect(url);
        }
      }
    }

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

  // Valid organization - check session for protected routes
  // Public paths on subdomain: /login, /signup, /invite, /reset-password
  const publicSubdomainPaths = [
    "/login",
    "/signup",
    "/invite",
    "/reset-password",
  ];
  const isPublicSubdomainPath = publicSubdomainPaths.some(
    (path) => pathname === path || pathname.startsWith(path + "/"),
  );

  // If not on a public path, check for session
  if (!isPublicSubdomainPath) {
    const session = await getUserSession(request);

    // No session on subdomain - redirect to login
    if (!session?.authenticated) {
      const loginUrl = new URL("/login", request.url);
      return NextResponse.redirect(loginUrl);
    }

    // Root path with session - allow (SmartRedirect handles in frontend)
    // Other protected paths - continue with tenant headers
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
