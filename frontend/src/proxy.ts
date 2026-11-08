import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { REQUEST_ID_HEADER } from "~/lib/request-id";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { normalizeHostname, tenantSlugFromHost } from "~/lib/tenant-host";
import { LOCALE_SCOPE_HEADER } from "~/i18n/locales";
import { berlinTodayISO } from "~/lib/date-helpers";
import {
  resolvePlanungRedirect,
  type PlanungRedirectTarget,
} from "~/components/timetable/planung-redirect";
import {
  HOSTED_PORTAL_KEYS,
  PORTAL_ROUTES,
  type HostedPortalKey,
} from "~/lib/portal-paths";
import { PORTAL_PAGES } from "./proxy-pages.generated";

/**
 * Routes by host (portal hosts from PORTAL_HOSTS, `{slug}.TENANT_DOMAIN`,
 * path mode on the bare domain) and sets the security headers.
 */

const isDev = process.env.NODE_ENV === "development";

// The proxy runs without t3-env, so it checks its configuration itself. Static
// property access: Next.js inlines NEXT_PUBLIC_* values only in that form.
function requiredEnv(name: string, value: string | undefined): string {
  if (!value) {
    throw new Error(
      `${name} is not set. Add it to your .env.local or docker-compose environment.`,
    );
  }
  return value;
}

/** Every portal host, checked at start-up. */
const HOSTNAMES = Object.fromEntries(
  HOSTED_PORTAL_KEYS.map((key) => {
    const portal = PORTAL_ROUTES[key];
    return [key, requiredEnv(portal.hostEnv, portal.hostname())];
  }),
) as Record<HostedPortalKey, string>;
const TENANT_DOMAIN = requiredEnv("TENANT_DOMAIN", process.env.TENANT_DOMAIN);

// --- CSP Headers ---

// PostHog runs behind the same-origin /ingest rewrite (next.config.js), so
// the policy names no analytics host.
// Keep Next.js inline bootstrap/hydration scripts working without a nonce
// migration. This is not a strict XSS policy; see docs/operations/browser-csp-3842.md.
const BLOB_FRAME_SRC = "frame-src 'self' blob:";
const CSP_HEADER = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "connect-src 'self'",
  "object-src 'none'",
  // A PDF the page loaded through the signed-in API shows in a frame from
  // its object URL (#5692); no other origin may be framed.
  BLOB_FRAME_SRC,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join("; ");

// Only the public tenant phase form mounts Turnstile. Parent-authenticated,
// preview, status and staff pages keep the same-origin policy.
const TURNSTILE_ORIGIN = "https://challenges.cloudflare.com";
const TURNSTILE_CSP_HEADER = CSP_HEADER.replace(
  "script-src 'self'",
  `script-src 'self' ${TURNSTILE_ORIGIN}`,
).replace(BLOB_FRAME_SRC, `frame-src ${TURNSTILE_ORIGIN}`);

function isPublicEnrollmentForm(pathname: string): boolean {
  const [, tenant, surface, phase, extra] = pathname.split("/");
  return (
    !!tenant &&
    !RESERVED_SLUGS.has(tenant) &&
    surface === "anmeldung" &&
    !!phase &&
    !["preview", "submitted", "status"].includes(phase) &&
    extra === undefined
  );
}

/** The public holiday registration (#4661) mounts Turnstile on the parents
 * host: /ferienanmeldung/{token}, rewritten to /parents/ferienanmeldung/…. */
function isPublicHolidayRegistration(pathname: string): boolean {
  const [, portal, surface, token, extra] = pathname.split("/");
  return (
    portal === "parents" &&
    surface === "ferienanmeldung" &&
    !!token &&
    extra === undefined
  );
}

// --- PostHog reverse proxy (#3601) ---

const POSTHOG_PROXY_PREFIX = "/ingest";
const POSTHOG_INGESTION_ORIGIN = "https://eu.i.posthog.com";
const POSTHOG_ASSETS_ORIGIN = "https://eu-assets.i.posthog.com";

// An external rewrite forwards every request header unless the proxy names
// the ones it keeps. Cookies carry the session tokens, Referer the raw URL
// with IDs, X-Forwarded-For the client IP: none of them may reach PostHog.
const POSTHOG_FORWARDED_HEADERS = [
  "accept",
  "accept-encoding",
  "content-length",
  "content-type",
] as const;

function isPostHogProxyPath(pathname: string): boolean {
  return (
    pathname === POSTHOG_PROXY_PREFIX ||
    pathname.startsWith(`${POSTHOG_PROXY_PREFIX}/`)
  );
}

function proxyPostHog(request: NextRequest): NextResponse {
  const path =
    request.nextUrl.pathname.slice(POSTHOG_PROXY_PREFIX.length) || "/";
  const origin = /^\/(static|array)\//.test(path)
    ? POSTHOG_ASSETS_ORIGIN
    : POSTHOG_INGESTION_ORIGIN;
  const target = new URL(`${origin}${path}${request.nextUrl.search}`);
  if (target.origin !== origin) {
    return new NextResponse(null, { status: 404 });
  }

  const headers = new Headers();
  for (const name of POSTHOG_FORWARDED_HEADERS) {
    const value = request.headers.get(name);
    if (value !== null) headers.set(name, value);
  }
  return NextResponse.rewrite(target, { request: { headers } });
}

const ORIGINAL_HOST_HEADER = "X-Moto-Original-Host";
const ORIGINAL_PROTO_HEADER = "X-Moto-Original-Proto";
// Path and query the browser asked for; the tenant layout needs them to send
// path mode to the school's subdomain (#5130).
const ORIGINAL_PATH_HEADER = "X-Moto-Original-Path";

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
  headers.set(
    ORIGINAL_PATH_HEADER,
    `${request.nextUrl.pathname}${request.nextUrl.search}`,
  );
  // A fresh Vorgangskennung per request, never the client's value: the BFF
  // tags its Sentry events with it and forwards it to the backend.
  headers.set(REQUEST_ID_HEADER, crypto.randomUUID());
  return headers;
}

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

/**
 * The sandbox of a file a person uploaded, as the backend sends it with an
 * inline file (#5275): under the app CSP an embedded script of a PDF would
 * run on the app's origin. Next sets the proxy's headers on the response
 * before the route runs and keeps them over the route's own, so this list
 * alone decides.
 */
const SANDBOX_CSP_HEADER =
  "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'";

/** API routes that stream an uploaded file the browser may show inline. */
const SANDBOXED_FILE_ROUTES: readonly RegExp[] = [
  // Proofs parents attach to an enrollment request (#4783).
  /^\/api\/enrollment\/admin\/requests\/[^/]+\/uploads\/[^/]+$/,
  // Proofs of benefits of a holiday registration (#5238).
  /^\/api\/traeger\/holiday-periods\/[^/]+\/registrations\/[^/]+\/proofs\/[^/]+$/,
  // Managed files and announcement attachments, staff and parents.
  /^\/api\/files\/folders\/[^/]+\/files\/[^/]+\/download$/,
  /^\/api\/announcement-attachments\/[^/]+\/[^/]+\/download$/,
  /^\/api\/parent\/me\/news\/[^/]+\/attachments\/[^/]+\/download$/,
  // Documents staff attach to a child (#777) or a staff member (#1424).
  /^\/api\/(?:students|staff)\/[^/]+\/documents\/[^/]+\/download$/,
];

/** Like NextResponse.next, but strips any client-forged localize header. */
function secureNext(request: NextRequest): NextResponse {
  const response = withSecurityHeaders(
    NextResponse.next({ request: { headers: sanitizedHeaders(request) } }),
  );
  const { pathname } = request.nextUrl;
  if (SANDBOXED_FILE_ROUTES.some((route) => route.test(pathname))) {
    response.headers.set("Content-Security-Policy", SANDBOX_CSP_HEADER);
  }
  return response;
}

/** Like NextResponse.rewrite, but strips any client-forged localize header. */
function secureRewrite(request: NextRequest, url: URL): NextResponse {
  return withSecurityHeaders(
    NextResponse.rewrite(url, {
      request: { headers: sanitizedHeaders(request) },
    }),
  );
}

// --- Portal hosts ---

/** A portal on its own host. The host serves the portal's pages without the
 * path namespace (`/login` → `/operator/login`) and only the BFF routes its
 * pages call: the tenant session cookie is domain-scoped and reaches every
 * portal host, so a tenant route must not run there. */
interface PortalHostRules {
  /** BFF routes the host serves: a string ending in "/" is a prefix, any
   * other string an exact path. Every other /api/ path answers 404. The
   * portal's own prefix also carries its client log route (`portalLogPath`). */
  readonly api: readonly (string | RegExp)[];
  /** Parent-facing: the request may render in the parent's language. */
  readonly localized: boolean;
  /** Serves the public /help pages. */
  readonly help: boolean;
  /** Retired pages answered with 308 to their new place (#5685). */
  readonly movedPaths?: ReadonlyArray<readonly [from: string, to: string]>;
  /** An API namespace that runs on this host only; every other host answers
   * 404, so it never runs beside another portal's cookies (ADR 0077). */
  readonly exclusiveApi?: string;
}

interface PortalHost extends PortalHostRules {
  readonly hostname: string;
  /** Path namespace of the portal's pages inside the App Router. */
  readonly appPrefix: `/${string}`;
  /** First segments of the pages under app/<portal>/ (generated). */
  readonly pages: readonly string[];
}

const HOST_RULES: Record<HostedPortalKey, PortalHostRules> = {
  operator: {
    api: ["/api/operator/"],
    localized: false,
    help: true,
  },
  parent: {
    api: [
      "/api/parent/",
      // Anonymous routes of the pages a link opens without a parent session:
      // invitations, enrollment status and its uploads, holiday registration,
      // contract signing, the demo entry and the school's login image.
      "/api/public/",
      "/api/tenant/resolve",
      "/api/invitations/validate",
      "/api/invitations/accept",
      "/api/guardian-invitations/",
      "/api/holiday-registrations/",
      "/api/enrollment/requests/",
      /^\/api\/enrollment\/[^/]+\/uploads$/,
      "/api/demo/access/",
    ],
    localized: true,
    help: true,
  },
  school: {
    api: [
      "/api/school/",
      "/api/invitations/validate",
      "/api/invitations/accept",
    ],
    localized: false,
    help: true,
  },
  traeger: {
    api: ["/api/traeger/"],
    localized: false,
    help: true,
    // Bookmarks, help links and e-mails name these pages, also with /traeger.
    movedPaths: [
      ["/accounts", "/einstellungen/mitglieder"],
      ["/access-log", "/einstellungen/zugriffsprotokoll"],
      ["/briefkopf", "/einstellungen/briefkopf"],
      ["/beitraege", "/einstellungen/abrechnung"],
    ],
  },
  caterer: {
    api: ["/api/caterer/"],
    localized: false,
    // The help has no caterer role and the portal links to no article.
    help: false,
    exclusiveApi: "/api/caterer",
  },
};

const PORTAL_HOSTS: readonly PortalHost[] = HOSTED_PORTAL_KEYS.map((key) => {
  const { appPrefix } = PORTAL_ROUTES[key];
  const pages = (PORTAL_PAGES as Record<string, readonly string[]>)[
    appPrefix.slice(1)
  ];
  // scripts/generate-portal-pages.mjs lists the same folders.
  if (!pages) throw new Error(`No generated pages for app${appPrefix}`);
  return { ...HOST_RULES[key], hostname: HOSTNAMES[key], appPrefix, pages };
});

function isUnder(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

function servesApi(host: PortalHost, pathname: string): boolean {
  return host.api.some((rule) => {
    if (typeof rule !== "string") return rule.test(pathname);
    return rule.endsWith("/") ? pathname.startsWith(rule) : pathname === rule;
  });
}

/** Portal hosts serve only their own allow-list; this keeps an exclusive
 * API off the tenant hosts and the bare domain too. */
function isExclusivePortalApi(pathname: string): boolean {
  return PORTAL_HOSTS.some(
    (host) =>
      host.exclusiveApi !== undefined && isUnder(pathname, host.exclusiveApi),
  );
}

function movedTarget(host: PortalHost, pathname: string): string | null {
  if (!host.movedPaths) return null;
  const page = (
    isUnder(pathname, host.appPrefix)
      ? pathname.slice(host.appPrefix.length)
      : pathname
  ).replace(/\/$/, "");
  return host.movedPaths.find(([from]) => page === from)?.[1] ?? null;
}

function notFound(): NextResponse {
  return withSecurityHeaders(new NextResponse(null, { status: 404 }));
}

function handlePortalHost(
  request: NextRequest,
  host: PortalHost,
): NextResponse {
  const { pathname } = request.nextUrl;

  if (host.help && isUnder(pathname, "/help")) return secureNext(request);
  if (isUnder(pathname, "/monitoring")) return secureNext(request);
  if (pathname.startsWith("/api/")) {
    return servesApi(host, pathname) ? secureNext(request) : notFound();
  }

  const rewrite = host.localized ? rewriteLocalized : secureRewrite;
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = host.appPrefix;
    return rewrite(request, url);
  }

  const moved = movedTarget(host, pathname);
  if (moved) {
    const url = new URL(request.url);
    url.pathname = moved;
    return withSecurityHeaders(NextResponse.redirect(url, 308));
  }

  if (host.pages.some((page) => isUnder(pathname, page))) {
    const url = request.nextUrl.clone();
    url.pathname = `${host.appPrefix}${pathname}`;
    return rewrite(request, url);
  }

  if (isUnder(pathname, host.appPrefix)) {
    return host.localized ? nextLocalized(request) : secureNext(request);
  }

  // Anything else goes to the portal's start: a portal host serves no other
  // portal's pages.
  const url = request.nextUrl.clone();
  url.pathname = "/";
  return withSecurityHeaders(NextResponse.redirect(url));
}

/** A portal's pages on any other host go to the portal's own host. */
function redirectToPortalHost(request: NextRequest): NextResponse | null {
  const { pathname, protocol, search } = request.nextUrl;
  const host = PORTAL_HOSTS.find((h) => isUnder(pathname, h.appPrefix));
  if (!host) return null;
  const page = pathname.slice(host.appPrefix.length) || "/";
  return withSecurityHeaders(
    NextResponse.redirect(`${protocol}//${host.hostname}${page}${search}`),
  );
}

function isLegacyParentsHost(hostname: string): boolean {
  const legacyParentsHost = `parents.${TENANT_DOMAIN}`;
  return (
    normalizeHostname(hostname) === legacyParentsHost &&
    normalizeHostname(HOSTNAMES.parent) !== legacyParentsHost
  );
}

function redirectLegacyParentsHost(request: NextRequest): NextResponse {
  const url = new URL(`${originalProtocol(request)}://${HOSTNAMES.parent}`);
  url.pathname = request.nextUrl.pathname;
  url.search = request.nextUrl.search;
  return withSecurityHeaders(NextResponse.redirect(url, 302));
}

// --- Tenant hosts ---

/** Public parent-enrollment surface. German path since #2829; the English
 * `/enroll` lives on only as the legacy redirect below, because it is
 * printed in parent letters that schools already handed out. */
const ENROLLMENT_PATH = "/anmeldung";
const LEGACY_ENROLLMENT_PATH = "/enroll";

function isEnrollmentPath(pathname: string): boolean {
  return (
    pathname === ENROLLMENT_PATH || pathname.startsWith(`${ENROLLMENT_PATH}/`)
  );
}

function withEnrollmentPath(prefix: string, rest: string): string {
  return `${prefix}${ENROLLMENT_PATH}${rest}`;
}

/** Maps a legacy `/enroll…` URL onto its German successor, or null when the
 * path is unrelated. Covers the parents host (`/enroll/status/x`), a tenant
 * subdomain (`/enroll/phase-1`), path mode on the bare domain
 * (`/school-a/enroll/phase-1`) and parent-portal links rendered before the
 * route rename (`/parents/enroll/status/x`). */
function legacyEnrollmentTarget(pathname: string): string | null {
  if (
    pathname === LEGACY_ENROLLMENT_PATH ||
    pathname.startsWith(`${LEGACY_ENROLLMENT_PATH}/`)
  ) {
    return withEnrollmentPath(
      "",
      pathname.slice(LEGACY_ENROLLMENT_PATH.length),
    );
  }

  const legacyParentsPath = `/parents${LEGACY_ENROLLMENT_PATH}`;
  if (
    pathname === legacyParentsPath ||
    pathname.startsWith(`${legacyParentsPath}/`)
  ) {
    return withEnrollmentPath(
      "/parents",
      pathname.slice(legacyParentsPath.length),
    );
  }

  const match = /^\/([^/]+)(\/enroll(?:\/.*)?)$/.exec(pathname);
  const slug = match?.[1];
  const tail = match?.[2];
  if (!slug || !tail || RESERVED_SLUGS.has(slug)) return null;

  return withEnrollmentPath(
    `/${slug}`,
    tail.slice(LEGACY_ENROLLMENT_PATH.length),
  );
}

function redirectLegacyEnrollment(
  request: NextRequest,
  target: string,
): NextResponse {
  const url = request.nextUrl.clone();
  url.pathname = target;
  // 308 keeps method and body, and lets browsers cache the move.
  return withSecurityHeaders(NextResponse.redirect(url, 308));
}

function getBareTenantPrefixedPath(pathname: string): string | null {
  const match = pathname.match(/^\/([^/]+)(\/.*)?$/);
  if (!match) return null;

  const slug = match[1];
  if (!slug || RESERVED_SLUGS.has(slug)) return null;

  return match[2] ?? "/";
}

type LegacySelectionSource = {
  readonly collectionPath: string;
  readonly selectionKey: string;
  readonly target: (selection: string) => string;
  readonly tab?: string;
};

const LEGACY_SELECTION_SOURCES: readonly LegacySelectionSource[] = [
  {
    collectionPath: "/rooms",
    selectionKey: "room",
    target: (selection: string) =>
      selection === "__transit__"
        ? "/rooms/unterwegs"
        : `/rooms/${encodePathSegment(selection)}`,
  },
  {
    collectionPath: "/database/rooms",
    selectionKey: "room",
    target: (selection: string) => `/rooms/${encodePathSegment(selection)}`,
    tab: "stammdaten",
  },
  {
    collectionPath: "/database/students",
    selectionKey: "student",
    target: (selection: string) => `/students/${encodePathSegment(selection)}`,
  },
  {
    collectionPath: "/database/personal",
    selectionKey: "staff",
    target: (selection: string) => `/staff/${encodePathSegment(selection)}`,
    tab: "konto",
  },
];

/** Dots must stay encoded: assigning a literal `..` to URL.pathname would
 * normalize it into a different route. */
function encodePathSegment(value: string): string {
  return encodeURIComponent(value).replaceAll(".", "%2E");
}

/**
 * The old collection panes selected their object through a query parameter.
 * Next's static redirects forward extra query parameters beside the fixed
 * destination query, but cannot nest them in `from`. This request-aware
 * redirect keeps search and grouping on the originating collection instead.
 */
function redirectLegacySelection(request: NextRequest): NextResponse | null {
  const { pathname, searchParams } = request.nextUrl;
  let collectionPath = pathname;
  let tenantPrefix = "";

  if (
    !LEGACY_SELECTION_SOURCES.some((rule) => rule.collectionPath === pathname)
  ) {
    const match =
      /^\/([^/]+)(\/(?:rooms|database\/(?:rooms|students|personal)))$/.exec(
        pathname,
      );
    const tenant = match?.[1];
    const nestedCollectionPath = match?.[2];
    // These are fixed first path segments, never tenant slugs. The same
    // exclusion made the previous next.config redirect unambiguous.
    if (
      !tenant ||
      !nestedCollectionPath ||
      ["api", "database", "rooms", "students", "staff"].includes(tenant)
    ) {
      return null;
    }
    collectionPath = nestedCollectionPath;
    tenantPrefix = `/${tenant}`;
  }

  const rule = LEGACY_SELECTION_SOURCES.find(
    (entry) => entry.collectionPath === collectionPath,
  );
  if (!rule) return null;

  const selection = searchParams.get(rule.selectionKey);
  if (!selection) return null;

  const collectionQuery = new URLSearchParams(searchParams);
  collectionQuery.delete(rule.selectionKey);
  const query = collectionQuery.toString();
  const referrer = query ? `${collectionPath}?${query}` : collectionPath;
  const url = request.nextUrl.clone();
  url.pathname = `${tenantPrefix}${rule.target(selection)}`;
  url.search = "";
  if (rule.tab) url.searchParams.set("tab", rule.tab);
  url.searchParams.set("from", referrer);

  return withSecurityHeaders(NextResponse.redirect(url));
}

/**
 * Retired tenant pages that live on in bookmarks and old links. They used to
 * be client pages that loaded a chunk and an RSC payload only to redirect;
 * answering here sends the browser on before any page renders (#5001).
 * `/planung` derives its area from `?tab=`; the other planning routes have a
 * fixed one.
 */
const RETIRED_PLANNING_ROUTES: Readonly<
  Record<string, PlanungRedirectTarget | undefined>
> = {
  "/planung": undefined,
  "/timetables": "betreuungsplan",
  "/vertretungsplan": "vertretung",
  "/staff/dienstplan": "dienstplan",
};

function retiredTenantRouteTarget(
  appPath: string,
  searchParams: URLSearchParams,
): string | null {
  if (appPath === "/invitations") return "/database/personal";
  if (!Object.hasOwn(RETIRED_PLANNING_ROUTES, appPath)) return null;
  // Berlin calendar day (docs/07 §1), so the `d` anchor matches the
  // backend's timezone.TodayDate().
  return resolvePlanungRedirect(
    searchParams,
    berlinTodayISO(),
    RETIRED_PLANNING_ROUTES[appPath],
  );
}

/**
 * Covers a tenant subdomain (bare or slug-prefixed path) and path mode on
 * the bare domain; the tenant prefix of the request stays on the target.
 */
function redirectRetiredTenantRoute(
  request: NextRequest,
  tenantSlug: string | null,
): NextResponse | null {
  const { pathname, searchParams } = request.nextUrl;
  let prefix = "";
  let appPath = pathname;
  if (tenantSlug) {
    if (pathname.startsWith(`/${tenantSlug}/`)) {
      prefix = `/${tenantSlug}`;
      appPath = pathname.slice(prefix.length);
    }
  } else {
    const bareAppPath = getBareTenantPrefixedPath(pathname);
    if (!bareAppPath) return null;
    prefix = `/${pathname.split("/")[1]}`;
    appPath = bareAppPath;
  }

  const target = retiredTenantRouteTarget(appPath, searchParams);
  if (target === null) return null;

  const url = request.nextUrl.clone();
  const queryStart = target.indexOf("?");
  url.pathname = `${prefix}${queryStart === -1 ? target : target.slice(0, queryStart)}`;
  url.search = queryStart === -1 ? "" : target.slice(queryStart);
  return withSecurityHeaders(NextResponse.redirect(url));
}

// --- Main proxy ---

export function proxy(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  if (isPostHogProxyPath(pathname)) {
    return proxyPostHog(request);
  }

  const response = routeRequest(request);

  // next.config.js sets skipTrailingSlashRedirect so the PostHog proxy
  // keeps the trailing slash its endpoints need (/ingest/e/). A path the
  // routing would serve as it is keeps the Next.js default and loses its
  // trailing slash; routing redirects such as old bookmarks win. Relative
  // Location: the browser resolves it against the host it asked.
  const servedAsIs =
    !pathname.startsWith("/_next") &&
    (response.headers.has("x-middleware-next") ||
      response.headers.has("x-middleware-rewrite"));
  if (servedAsIs && pathname.length > 1 && pathname.endsWith("/")) {
    const target = `${pathname.replace(/\/+$/, "") || "/"}${request.nextUrl.search}`;
    return withSecurityHeaders(
      new NextResponse(null, { status: 308, headers: { Location: target } }),
    );
  }
  const pagePath = new URL(
    response.headers.get("x-middleware-rewrite") ?? request.url,
  ).pathname;
  if (
    servedAsIs &&
    (isPublicEnrollmentForm(pagePath) || isPublicHolidayRegistration(pagePath))
  ) {
    response.headers.set("Content-Security-Policy", TURNSTILE_CSP_HEADER);
  }
  return response;
}

function routeRequest(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  if (pathname.startsWith("/_next")) {
    return NextResponse.next();
  }

  const hostname = originalHost(request);

  // Parent letters and e-mails sent before the rename (#2829) still point at
  // /enroll, on the parents host, tenant subdomains and in path mode alike.
  const legacyEnrollment = legacyEnrollmentTarget(pathname);
  if (legacyEnrollment) {
    return redirectLegacyEnrollment(request, legacyEnrollment);
  }

  // Local dev already uses parents.localhost as the canonical parents host.
  if (isLegacyParentsHost(hostname)) {
    return redirectLegacyParentsHost(request);
  }

  const portalHost = PORTAL_HOSTS.find((host) => host.hostname === hostname);
  if (portalHost) return handlePortalHost(request, portalHost);

  // Object links from panes that existed before #3115 need the whole
  // collection query nested in `from`, which static next.config redirects
  // cannot express.
  const legacySelection = redirectLegacySelection(request);
  if (legacySelection) return legacySelection;

  if (isExclusivePortalApi(pathname)) return notFound();
  if (pathname.startsWith("/api") || pathname.startsWith("/monitoring")) {
    return secureNext(request);
  }

  // Without this, a tenant subdomain would rewrite /help to /{tenant}/help,
  // where no route exists.
  if (isUnder(pathname, "/help")) {
    return secureNext(request);
  }

  const portalRedirect = redirectToPortalHost(request);
  if (portalRedirect) return portalRedirect;

  // Bookmark bridge for the old Lehrkraft-Klassenansicht (#2207 PR 3).
  // Until the cutover the view lived at {slug}.{TENANT_DOMAIN}/klassen; the
  // page is gone and a Lehrkraft-only account can no longer log in here at
  // all. A 404 would look like the school lost its access, so send the
  // bookmark to moto schule instead. Remove once the schools have settled in.
  const tenantSlug = tenantSlugFromHost(hostname);
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
    const redirectUrl = `${protocol}//${HOSTNAMES.school}/login${search}`;
    return withSecurityHeaders(NextResponse.redirect(redirectUrl));
  }

  const retiredRoute = redirectRetiredTenantRoute(request, tenantSlug);
  if (retiredRoute) return retiredRoute;

  // The bare domain is path mode: the root page picks the school.
  if (!tenantSlug) {
    const appPath = getBareTenantPrefixedPath(pathname);
    return appPath && isEnrollmentPath(appPath)
      ? nextLocalized(request)
      : secureNext(request);
  }

  // Public enrollment is the only parent-facing surface on a tenant subdomain,
  // so it's the only path we localize here. The staff portal stays German.
  const appPath = pathname.startsWith(`/${tenantSlug}`)
    ? pathname.slice(tenantSlug.length + 1) || "/"
    : pathname;
  const normalizedAppPath = appPath.startsWith("/") ? appPath : `/${appPath}`;
  const isEnroll = isEnrollmentPath(normalizedAppPath);

  // useTenantRouter().push() adds the slug itself; a second prefix would
  // make /school-a/school-a/dashboard.
  if (pathname.startsWith(`/${tenantSlug}/`) || pathname === `/${tenantSlug}`) {
    return isEnroll ? nextLocalized(request) : secureNext(request);
  }

  if (pathname === "/login") {
    const url = request.nextUrl.clone();
    url.pathname = `/${tenantSlug}`;
    return secureRewrite(request, url);
  }

  const url = request.nextUrl.clone();
  url.pathname = `/${tenantSlug}${pathname}`;
  return isEnroll
    ? rewriteLocalized(request, url)
    : secureRewrite(request, url);
}

export const config = {
  matcher: [
    // Next.js requires a literal so it can statically analyze the matcher.
    // downloads/ holds public files that mails link on every host, such as
    // the Elterninfo on the parents host (#3534).
    "/((?!_next/static|_next/image|favicon\\.ico|favicon\\.png|apple-touch-icon\\.png|site\\.webmanifest|manifest\\.webmanifest|sw\\.js|favicons/|icons/|images/|downloads/).*)",
  ],
};
