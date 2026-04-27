import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";

/**
 * Combined proxy for:
 * 1. Operator subdomain separation (from development)
 * 2. Subdomain-based tenant routing (from multi-tenancy)
 * 3. CSP security headers
 */

const isDev = process.env.NODE_ENV === "development";

// --- Fail-fast environment checks (no t3-env at proxy level) ---

const OPERATOR_HOSTNAME = process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME;
if (!OPERATOR_HOSTNAME) {
  throw new Error(
    "NEXT_PUBLIC_OPERATOR_HOSTNAME is not set. " +
      "Add it to your .env.local or docker-compose environment.",
  );
}

const TENANT_DOMAIN: string = (() => {
  const val = process.env.TENANT_DOMAIN;
  if (!val) {
    throw new Error(
      "TENANT_DOMAIN is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }
  return val;
})();

// --- CSP Headers ---

const CSP_HEADER = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "connect-src 'self'",
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join("; ");

/** Attach security headers (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy) to a response. */
function withSecurityHeaders(response: NextResponse): NextResponse {
  response.headers.set("Content-Security-Policy", CSP_HEADER);
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  return response;
}

// --- Operator subdomain handling (from development) ---

/** Paths that the operator subdomain serves (without /operator prefix) */
const OPERATOR_PUBLIC_PATHS = [
  "/login",
  "/email-confirm",
  "/suggestions",
  "/announcements",
  "/settings",
  "/provisioning",
  "/organizations",
  "/schools",
  "/accounts",
  "/devices",
  "/persons",
  "/operators",
  "/invite",
];

function getHostname(request: NextRequest): string {
  return (
    request.headers.get("x-forwarded-host") ?? request.headers.get("host") ?? ""
  );
}

function isOperatorHost(hostname: string): boolean {
  return hostname === OPERATOR_HOSTNAME;
}

function handleOperatorSubdomain(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  // Block tenant auth endpoints on the operator host. Tenant session cookies
  // are domain-scoped and sent to subdomains, so allowing /api/auth/* here
  // would expose tenant session data on operator.moto-app.de.
  if (pathname.startsWith("/api/auth/")) {
    return withSecurityHeaders(new NextResponse(null, { status: 404 }));
  }

  // Pass through: operator API routes, static assets, Sentry tunnel
  // Note: favicon.ico, favicon.png, apple-touch-icon.png, site.webmanifest,
  // icons/, and images/ are excluded from the proxy matcher entirely.
  if (
    pathname.startsWith("/api/") ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/monitoring")
  ) {
    return withSecurityHeaders(NextResponse.next());
  }

  // Root → rewrite to /operator (which server-redirects to /operator/suggestions)
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = "/operator";
    return withSecurityHeaders(NextResponse.rewrite(url));
  }

  // Known operator paths → rewrite to /operator/* internally
  if (
    OPERATOR_PUBLIC_PATHS.some(
      (p) => pathname === p || pathname.startsWith(`${p}/`),
    )
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/operator${pathname}`;
    return withSecurityHeaders(NextResponse.rewrite(url));
  }

  // Already prefixed with /operator → pass through (handles direct /operator/* access)
  if (pathname.startsWith("/operator")) {
    return withSecurityHeaders(NextResponse.next());
  }

  // Everything else (e.g. /dashboard, /database/*) → redirect to root
  const url = request.nextUrl.clone();
  url.pathname = "/";
  return withSecurityHeaders(NextResponse.redirect(url));
}

// --- Tenant subdomain handling (from multi-tenancy) ---

// Alias for use in subdomain extraction below.
// Single source of truth: ~/lib/reserved-slugs.ts
const RESERVED_SUBDOMAINS = RESERVED_SLUGS;

function extractTenantSlug(host: string): string | null {
  // Strip port (e.g., "school-a.localhost:3000" -> "school-a.localhost")
  const hostname = host.split(":")[0] ?? "";

  // Check if hostname has a subdomain under TENANT_DOMAIN
  if (hostname !== TENANT_DOMAIN && hostname.endsWith(`.${TENANT_DOMAIN}`)) {
    const slug = hostname.slice(0, -(TENANT_DOMAIN.length + 1));

    // Block reserved subdomains from tenant resolution
    if (RESERVED_SUBDOMAINS.has(slug)) {
      return null;
    }

    return slug;
  }

  return null;
}

// --- Main proxy ---

export function proxy(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  // /_next/* — Next.js internals; skip entirely (static assets).
  if (pathname.startsWith("/_next")) {
    return NextResponse.next();
  }

  const hostname = getHostname(request);

  // 1. Operator subdomain gets its own routing
  if (isOperatorHost(hostname)) {
    return handleOperatorSubdomain(request);
  }

  // 2. /api/* and /monitoring (Sentry tunnel) — pass through with security headers.
  if (pathname.startsWith("/api") || pathname.startsWith("/monitoring")) {
    return withSecurityHeaders(NextResponse.next());
  }

  // 3. Operator auth guard on non-operator hosts:
  //    Protect all /operator/* routes except /operator/login by requiring the
  //    operator session cookie. Redirect /operator/* to operator subdomain if possible.
  if (pathname.startsWith("/operator")) {
    const cleanPath = pathname.replace(/^\/operator/, "") || "/";
    const protocol = request.nextUrl.protocol;
    const search = request.nextUrl.search;
    const redirectUrl = `${protocol}//${OPERATOR_HOSTNAME}${cleanPath}${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  // 4. Tenant subdomain routing
  const host = request.headers.get("host") ?? "";
  const tenantSlug = extractTenantSlug(host);

  // No subdomain (bare domain) — pass through without rewrite.
  // The root app/page.tsx can handle tenant selection or redirect.
  if (!tenantSlug) {
    return withSecurityHeaders(NextResponse.next());
  }

  // Already has tenant prefix — useTenantRouter().push() adds the slug explicitly,
  // so skip rewriting to avoid double-prefixing (e.g. /school-a/school-a/dashboard).
  if (pathname.startsWith(`/${tenantSlug}/`) || pathname === `/${tenantSlug}`) {
    return withSecurityHeaders(NextResponse.next());
  }

  // Rewrite: school-a.localhost:3000/dashboard -> internal /school-a/dashboard
  // This lets the [tenant] dynamic segment in the App Router capture the slug.
  const url = request.nextUrl.clone();
  url.pathname = `/${tenantSlug}${pathname}`;
  return withSecurityHeaders(NextResponse.rewrite(url));
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon\\.ico|favicon\\.png|apple-touch-icon\\.png|site\\.webmanifest|icons/|images/).*)",
  ],
};
