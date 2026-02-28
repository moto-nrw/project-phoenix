import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * Subdomain-based tenant routing middleware.
 *
 * Extracts the tenant slug from the subdomain (e.g., school-a.localhost:3000 -> "school-a")
 * and rewrites the request to the /[tenant]/... path segment so Next.js App Router can
 * resolve it via the dynamic [tenant] route.
 *
 * Stateless: no DB calls, no auth checks. The [tenant]/layout.tsx validates the slug.
 */

const isDev = process.env.NODE_ENV === "development";

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

/** Attach security headers (CSP, X-Content-Type-Options, X-Frame-Options) to a response. */
function withSecurityHeaders(response: NextResponse): NextResponse {
  response.headers.set("Content-Security-Policy", CSP_HEADER);
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  return response;
}

const TENANT_DOMAIN = process.env.TENANT_DOMAIN ?? "localhost";

/** Subdomains reserved for infrastructure — never treated as tenant slugs. */
const RESERVED_SUBDOMAINS = new Set(["www", "api", "admin", "operator", "app"]);

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

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // /api/* — Next.js API proxy routes; still attach security headers.
  // /_next/* — Next.js internals; skip entirely (static assets).
  if (pathname.startsWith("/_next")) {
    return NextResponse.next();
  }
  if (pathname.startsWith("/api")) {
    return withSecurityHeaders(NextResponse.next());
  }

  // Operator dashboard auth guard (merged from proxy.ts — Next.js 16 forbids
  // having both middleware.ts and proxy.ts). Protect all /operator/* routes
  // except /operator/login by requiring the operator session cookie.
  if (pathname.startsWith("/operator")) {
    if (
      !pathname.startsWith("/operator/login") &&
      !request.cookies.get("phoenix-operator-token")?.value
    ) {
      return NextResponse.redirect(new URL("/operator/login", request.url));
    }
    return withSecurityHeaders(NextResponse.next());
  }

  const host = request.headers.get("host") ?? "";
  const tenantSlug = extractTenantSlug(host);

  // No subdomain (bare domain) — pass through without rewrite.
  // The root app/page.tsx can handle tenant selection or redirect.
  if (!tenantSlug) {
    return withSecurityHeaders(NextResponse.next());
  }

  // "operator" subdomain routes to operator dashboard without tenant rewrite
  if (tenantSlug === "operator") {
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
    /*
     * Match all paths except static assets:
     * - _next/static, _next/image (Next.js built-in)
     * - Static files by extension (svg, png, jpg, ico, etc.)
     */
    "/((?!_next/static|_next/image|favicon\\.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico|webmanifest)$).*)",
  ],
};
