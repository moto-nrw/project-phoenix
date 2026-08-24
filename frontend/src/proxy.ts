import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { LOCALE_SCOPE_HEADER } from "~/i18n/locales";

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

const PARENTS_HOSTNAME = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
if (!PARENTS_HOSTNAME) {
  throw new Error(
    "NEXT_PUBLIC_PARENTS_HOSTNAME is not set. " +
      "Add it to your .env.local or docker-compose environment.",
  );
}

const SCHOOL_HOSTNAME = process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME;
if (!SCHOOL_HOSTNAME) {
  throw new Error(
    "NEXT_PUBLIC_SCHOOL_HOSTNAME is not set. " +
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

const POSTHOG_INGESTION_ORIGIN: string | null = (() => {
  const configuredKey = process.env.NEXT_PUBLIC_POSTHOG_KEY;
  const configuredHost = process.env.NEXT_PUBLIC_POSTHOG_HOST;
  if (!configuredKey) return null;
  if (!configuredHost) {
    throw new Error(
      "NEXT_PUBLIC_POSTHOG_HOST is required when NEXT_PUBLIC_POSTHOG_KEY is set.",
    );
  }

  const url = new URL(configuredHost);
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(
      "NEXT_PUBLIC_POSTHOG_HOST must use the http or https protocol.",
    );
  }
  return url.origin;
})();

// --- CSP Headers ---

const CSP_HEADER = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  `connect-src 'self'${POSTHOG_INGESTION_ORIGIN ? ` ${POSTHOG_INGESTION_ORIGIN}` : ""}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join("; ");

const ORIGINAL_HOST_HEADER = "X-Moto-Original-Host";
const ORIGINAL_PROTO_HEADER = "X-Moto-Original-Proto";

function originalHost(request: NextRequest): string {
  return (
    request.headers.get("x-forwarded-host") ?? request.headers.get("host") ?? ""
  );
}

function originalProtocol(request: NextRequest): string {
  const forwardedProto = request.headers
    .get("x-forwarded-proto")
    ?.split(",")[0]
    ?.trim();
  if (forwardedProto === "http" || forwardedProto === "https") {
    return forwardedProto;
  }
  return request.nextUrl.protocol.replace(/:$/, "");
}

function preserveOriginalRequestTarget(
  headers: Headers,
  request: NextRequest,
): Headers {
  const host = originalHost(request);
  if (host) headers.set(ORIGINAL_HOST_HEADER, host);
  headers.set(ORIGINAL_PROTO_HEADER, originalProtocol(request));
  return headers;
}

/** Attach security headers (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy) to a response. */
function withSecurityHeaders(response: NextResponse): NextResponse {
  response.headers.set("Content-Security-Policy", CSP_HEADER);
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  return response;
}

/** Clone the incoming request headers and mark this request as localized, so
 * request.ts resolves the parent locale instead of forcing German. */
function localizedHeaders(request: NextRequest): Headers {
  const headers = new Headers(request.headers);
  headers.set(LOCALE_SCOPE_HEADER, "1");
  return preserveOriginalRequestTarget(headers, request);
}

/** Like NextResponse.rewrite, but flags the request as a localized surface. */
function rewriteLocalized(request: NextRequest, url: URL): NextResponse {
  return withSecurityHeaders(
    NextResponse.rewrite(url, {
      request: { headers: localizedHeaders(request) },
    }),
  );
}

/** Like NextResponse.next, but flags the request as a localized surface. */
function nextLocalized(request: NextRequest): NextResponse {
  return withSecurityHeaders(
    NextResponse.next({ request: { headers: localizedHeaders(request) } }),
  );
}

/** Clone the incoming headers and strip the localize signal. x-phoenix-localize
 * is an internal flag the proxy sets only on parent-facing surfaces; a client
 * could otherwise forge it to make the German-only staff/operator portals
 * render in another language. Every non-localized response goes through this so
 * request.ts never sees a client-supplied value. */
function sanitizedHeaders(request: NextRequest): Headers {
  const headers = new Headers(request.headers);
  headers.delete(LOCALE_SCOPE_HEADER);
  return preserveOriginalRequestTarget(headers, request);
}

/** Like NextResponse.next, but strips any client-forged localize header. */
function secureNext(request: NextRequest): NextResponse {
  return withSecurityHeaders(
    NextResponse.next({ request: { headers: sanitizedHeaders(request) } }),
  );
}

/** Like NextResponse.rewrite, but strips any client-forged localize header. */
function secureRewrite(request: NextRequest, url: URL): NextResponse {
  return withSecurityHeaders(
    NextResponse.rewrite(url, {
      request: { headers: sanitizedHeaders(request) },
    }),
  );
}

// --- Operator subdomain handling (from development) ---

/** Paths that the operator subdomain serves (without /operator prefix) */
const OPERATOR_PUBLIC_PATHS = [
  "/login",
  "/email-confirm",
  "/announcements",
  "/settings",
  "/provisioning",
  "/organizations",
  "/schools",
  "/accounts",
  "/devices",
  "/persons",
  "/unregistered-tags",
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

  if (pathname === "/help" || pathname.startsWith("/help/")) {
    return secureNext(request);
  }

  // Block tenant auth endpoints on the operator host. Tenant session cookies
  // are domain-scoped and sent to subdomains, so allowing /api/auth/* here
  // would expose tenant session data on operator.moto-app.de.
  if (pathname.startsWith("/api/auth/")) {
    return withSecurityHeaders(new NextResponse(null, { status: 404 }));
  }

  // Pass through: operator API routes, static assets, Sentry tunnel
  // Note: favicon.ico, favicon.png, apple-touch-icon.png, manifests, sw.js
  // (the Web Push service worker must be same-origin and redirect-free on
  // every host), favicons/, icons/, and images/ are excluded from the proxy
  // matcher entirely.
  if (
    pathname.startsWith("/api/") ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/monitoring")
  ) {
    return secureNext(request);
  }

  // Root → rewrite to /operator (which server-redirects to /operator/organizations)
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = "/operator";
    return secureRewrite(request, url);
  }

  // Known operator paths → rewrite to /operator/* internally
  if (
    OPERATOR_PUBLIC_PATHS.some(
      (p) => pathname === p || pathname.startsWith(`${p}/`),
    )
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/operator${pathname}`;
    return secureRewrite(request, url);
  }

  // Already prefixed with /operator → pass through (handles direct /operator/* access)
  if (pathname.startsWith("/operator")) {
    return secureNext(request);
  }

  // Everything else (e.g. /dashboard, /database/*) → redirect to root
  const url = request.nextUrl.clone();
  url.pathname = "/";
  return withSecurityHeaders(NextResponse.redirect(url));
}

// --- Parents subdomain handling (cross-tenant guardian portal) ---

/** Paths the parents subdomain serves (without /parents prefix).
 * Mirrors OPERATOR_PUBLIC_PATHS in shape. The proxy rewrites these to
 * /parents/* internally so the App Router routes them under app/parent/.
 *
 * The "/parents" prefix is the path namespace inside the App Router,
 * NOT the URL path the user sees. On the configured parents host the user
 * sees /login → internally rewritten to /parents/login.
 */
const PARENTS_PUBLIC_PATHS = [
  "/login",
  "/reset-password",
  "/email-confirm",
  "/children",
  "/messages",
  "/news",
  "/meal-plan",
  "/calendar",
  "/settings",
  "/enroll",
  "/accept-guardian-invite",
];

function isParentsHost(hostname: string): boolean {
  return hostname === PARENTS_HOSTNAME;
}

function hostnameWithoutPort(hostname: string): string {
  const [host = ""] = hostname.split(":");
  return host;
}

function isLegacyParentsHost(hostname: string): boolean {
  if (!PARENTS_HOSTNAME) return false;
  const legacyParentsHost = `parents.${TENANT_DOMAIN}`;
  return (
    hostnameWithoutPort(hostname) === legacyParentsHost &&
    hostnameWithoutPort(PARENTS_HOSTNAME) !== legacyParentsHost
  );
}

function redirectLegacyParentsHost(request: NextRequest): NextResponse {
  if (!PARENTS_HOSTNAME) {
    throw new Error("NEXT_PUBLIC_PARENTS_HOSTNAME is not set.");
  }
  const url = new URL(`${originalProtocol(request)}://${PARENTS_HOSTNAME}`);
  url.pathname = request.nextUrl.pathname;
  url.search = request.nextUrl.search;
  return withSecurityHeaders(NextResponse.redirect(url, 302));
}

function handleParentsSubdomain(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  if (pathname === "/help" || pathname.startsWith("/help/")) {
    return secureNext(request);
  }

  // Block tenant + operator auth endpoints on the parents host. Tenant
  // and operator session cookies are strictly host-only on their own
  // subdomains, but we 404 here as defense-in-depth in case a misconfig
  // ever leaked them.
  if (
    pathname.startsWith("/api/auth/") ||
    pathname.startsWith("/api/operator/auth/")
  ) {
    return withSecurityHeaders(new NextResponse(null, { status: 404 }));
  }

  // Pass through: parents API routes, static assets, Sentry tunnel.
  if (
    pathname.startsWith("/api/") ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/monitoring")
  ) {
    return secureNext(request);
  }

  // Root → /parents (which server-redirects to the dashboard).
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = "/parents";
    return rewriteLocalized(request, url);
  }

  // Known parents paths → rewrite to /parents/* internally.
  if (
    PARENTS_PUBLIC_PATHS.some(
      (p) => pathname === p || pathname.startsWith(`${p}/`),
    )
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/parents${pathname}`;
    return rewriteLocalized(request, url);
  }

  // Already prefixed with /parents → pass through (handles direct
  // /parents/* access during dev).
  if (pathname.startsWith("/parents")) {
    return nextLocalized(request);
  }

  // Anything else → redirect to root. Keeps the parents host from
  // leaking access to tenant or operator paths.
  const url = request.nextUrl.clone();
  url.pathname = "/";
  return withSecurityHeaders(NextResponse.redirect(url));
}

// --- School subdomain handling ("moto schule" teacher portal, #2207) ---

/** Paths the school subdomain serves (without /school prefix).
 * Mirrors PARENTS_PUBLIC_PATHS in shape: the proxy rewrites these to
 * /school/* internally so the App Router routes them under app/school/.
 * The Klassenansicht itself lives at the root ("/" → /school).
 */
const SCHOOL_PUBLIC_PATHS = ["/login", "/invite", "/reset-password"];
const SCHOOL_INVITATION_API_PATHS = [
  "/api/invitations/validate",
  "/api/invitations/accept",
];

function isSchoolHost(hostname: string): boolean {
  return hostname === SCHOOL_HOSTNAME;
}

function handleSchoolSubdomain(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  if (pathname === "/help" || pathname.startsWith("/help/")) {
    return secureNext(request);
  }

  // Block the other portals' auth endpoints on the school host. Their
  // session cookies are host-only (tenant: domain-scoped), so this is
  // defense-in-depth against a misconfig ever leaking them here.
  if (
    pathname.startsWith("/api/auth/") ||
    pathname.startsWith("/api/operator/auth/") ||
    pathname.startsWith("/api/parent/auth/")
  ) {
    return withSecurityHeaders(new NextResponse(null, { status: 404 }));
  }

  // Only the school portal's BFF routes may run on this host. In particular,
  // do not pass tenant API routes through: tenant session cookies are
  // domain-scoped and would otherwise be sent to this subdomain as well.
  if (
    pathname.startsWith("/api/school/") ||
    SCHOOL_INVITATION_API_PATHS.includes(pathname) ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/monitoring")
  ) {
    return secureNext(request);
  }

  if (pathname.startsWith("/api/")) {
    return withSecurityHeaders(new NextResponse(null, { status: 404 }));
  }

  // Root → /school (the Klassenansicht). The staff/school surfaces are
  // German-only, so no localized rewrite here.
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = "/school";
    return secureRewrite(request, url);
  }

  // Known school paths → rewrite to /school/* internally.
  if (
    SCHOOL_PUBLIC_PATHS.some(
      (p) => pathname === p || pathname.startsWith(`${p}/`),
    )
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/school${pathname}`;
    return secureRewrite(request, url);
  }

  // Already prefixed with /school → pass through (direct /school/* access
  // during dev).
  if (pathname === "/school" || pathname.startsWith("/school/")) {
    return secureNext(request);
  }

  // Anything else → redirect to root. Keeps the school host from leaking
  // access to tenant, operator, or parents paths.
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

function isEnrollPath(pathname: string): boolean {
  return pathname === "/enroll" || pathname.startsWith("/enroll/");
}

function getBareTenantPrefixedPath(pathname: string): string | null {
  const match = pathname.match(/^\/([^/]+)(\/.*)?$/);
  if (!match) return null;

  const slug = match[1];
  if (!slug || RESERVED_SUBDOMAINS.has(slug)) return null;

  return match[2] ?? "/";
}

// --- Main proxy ---

export function proxy(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  // /_next/*: Next.js internals; skip entirely (static assets).
  if (pathname.startsWith("/_next")) {
    return NextResponse.next();
  }

  const hostname = getHostname(request);

  // 1a. Operator subdomain gets its own routing
  if (isOperatorHost(hostname)) {
    return handleOperatorSubdomain(request);
  }

  // 1b. Legacy deployed parents host redirects to the configured canonical
  // parents portal host. Local dev stays on parents.localhost:3000 because it
  // is already the canonical PARENTS_HOSTNAME there.
  if (isLegacyParentsHost(hostname)) {
    return redirectLegacyParentsHost(request);
  }

  // 1c. Parents subdomain gets its own routing (mirrors operator)
  if (isParentsHost(hostname)) {
    return handleParentsSubdomain(request);
  }

  // 1d. School subdomain ("moto schule") gets its own routing (#2207)
  if (isSchoolHost(hostname)) {
    return handleSchoolSubdomain(request);
  }

  // 2. /api/* and /monitoring (Sentry tunnel): pass through with security headers.
  if (pathname.startsWith("/api") || pathname.startsWith("/monitoring")) {
    return secureNext(request);
  }

  // Public documentation must stay host-agnostic. Without this exception,
  // tenant subdomains rewrite /help to /{tenant}/help, where no
  // App Router route exists.
  if (pathname === "/help" || pathname.startsWith("/help/")) {
    return secureNext(request);
  }

  // 3a. Operator auth guard on non-operator hosts:
  //    Protect all /operator/* routes except /operator/login by requiring the
  //    operator session cookie. Redirect /operator/* to operator subdomain if possible.
  if (pathname.startsWith("/operator")) {
    const cleanPath = pathname.replace(/^\/operator/, "") || "/";
    const protocol = request.nextUrl.protocol;
    const search = request.nextUrl.search;
    const redirectUrl = `${protocol}//${OPERATOR_HOSTNAME}${cleanPath}${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  // 3b. Parents auth guard on non-parents hosts: same pattern as
  // operator. /parents/* hit on a tenant subdomain or the bare domain
  // gets redirected to the configured parents host. Defense-in-depth so
  // a stray link or bookmark can't accidentally serve parent UI from
  // a tenant context.
  if (pathname.startsWith("/parents")) {
    const cleanPath = pathname.replace(/^\/parents/, "") || "/";
    const protocol = request.nextUrl.protocol;
    const search = request.nextUrl.search;
    const redirectUrl = `${protocol}//${PARENTS_HOSTNAME}${cleanPath}${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  // 3c. School guard on non-school hosts: /school/* hit on a tenant
  // subdomain or the bare domain gets redirected to the configured school
  // host. Exact-segment match — a tenant slug like "school-a" must NOT be
  // hijacked here ("/school-a/dashboard" is a legitimate tenant path).
  if (pathname === "/school" || pathname.startsWith("/school/")) {
    const cleanPath = pathname.replace(/^\/school/, "") || "/";
    const protocol = request.nextUrl.protocol;
    const search = request.nextUrl.search;
    const redirectUrl = `${protocol}//${SCHOOL_HOSTNAME}${cleanPath}${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  // 3d. Bookmark bridge for the old Lehrkraft-Klassenansicht (#2207 PR 3).
  // Until the cutover the view lived at {slug}.{TENANT_DOMAIN}/klassen; the
  // page is gone and a Lehrkraft-only account can no longer log in here at
  // all. A 404 would look like the school lost its access, so send the
  // bookmark to moto schule instead. Remove once the schools have settled in.
  const tenantSlug = extractTenantSlug(hostname);
  const normalizedPathname = pathname.replace(/\/+$/, "") || "/";
  const bareTenantPath = getBareTenantPrefixedPath(pathname);
  const bareTenantSlug =
    bareTenantPath && (bareTenantPath.replace(/\/+$/, "") || "/") === "/klassen"
      ? pathname.split("/")[1]
      : null;
  const tenantPath = tenantSlug
    ? pathname.startsWith(`/${tenantSlug}`)
      ? pathname.slice(tenantSlug.length + 1) || "/"
      : pathname
    : null;
  const isLegacyClassBookmark =
    normalizedPathname === "/klassen" ||
    (tenantPath && (tenantPath.replace(/\/+$/, "") || "/") === "/klassen") ||
    bareTenantSlug !== null;
  if (isLegacyClassBookmark) {
    const protocol = request.nextUrl.protocol;
    const selectedTenantSlug = tenantSlug ?? bareTenantSlug;
    const search = selectedTenantSlug
      ? `?from=staff&tenant=${encodeURIComponent(selectedTenantSlug)}`
      : "?from=staff";
    const redirectUrl = `${protocol}//${SCHOOL_HOSTNAME}/login${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  // 4. Tenant subdomain routing
  // No subdomain (bare domain): pass through without rewrite.
  // The root app/page.tsx can handle tenant selection or redirect.
  if (!tenantSlug) {
    const appPath = getBareTenantPrefixedPath(pathname);
    return appPath && isEnrollPath(appPath)
      ? nextLocalized(request)
      : secureNext(request);
  }

  // Public enrollment is the only parent-facing surface on a tenant subdomain,
  // so it's the only path we localize here. The staff portal stays German.
  const appPath = pathname.startsWith(`/${tenantSlug}`)
    ? pathname.slice(tenantSlug.length + 1) || "/"
    : pathname;
  const normalizedAppPath = appPath.startsWith("/") ? appPath : `/${appPath}`;
  const isEnroll = isEnrollPath(normalizedAppPath);

  // Already has tenant prefix. useTenantRouter().push() adds the slug explicitly,
  // so skip rewriting to avoid double-prefixing (e.g. /school-a/school-a/dashboard).
  if (pathname.startsWith(`/${tenantSlug}/`) || pathname === `/${tenantSlug}`) {
    return isEnroll ? nextLocalized(request) : secureNext(request);
  }

  if (pathname === "/login") {
    const url = request.nextUrl.clone();
    url.pathname = `/${tenantSlug}`;
    return secureRewrite(request, url);
  }

  // Rewrite: school-a.localhost:3000/dashboard -> internal /school-a/dashboard
  // This lets the [tenant] dynamic segment in the App Router capture the slug.
  const url = request.nextUrl.clone();
  url.pathname = `/${tenantSlug}${pathname}`;
  return isEnroll
    ? rewriteLocalized(request, url)
    : secureRewrite(request, url);
}

export const config = {
  matcher: [
    // Next.js requires a literal so it can statically analyze the matcher.
    "/((?!_next/static|_next/image|favicon\\.ico|favicon\\.png|apple-touch-icon\\.png|site\\.webmanifest|manifest\\.webmanifest|sw\\.js|favicons/|icons/|images/).*)", // NOSONAR
  ],
};
