import { NextResponse, type NextRequest } from "next/server";

// Read directly from process.env — t3-env is incompatible with Edge runtime
const OPERATOR_HOSTNAME = process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME;
if (!OPERATOR_HOSTNAME) {
  throw new Error(
    "NEXT_PUBLIC_OPERATOR_HOSTNAME is not set. " +
      "Add it to your .env.local or docker-compose environment.",
  );
}

/** Paths that the operator subdomain serves (without /operator prefix) */
const OPERATOR_PUBLIC_PATHS = [
  "/login",
  "/suggestions",
  "/announcements",
  "/settings",
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

  // Pass through: all API routes, static assets
  if (
    pathname.startsWith("/api/") ||
    pathname.startsWith("/_next") ||
    pathname === "/favicon.ico" ||
    pathname.startsWith("/images")
  ) {
    return NextResponse.next();
  }

  // Root → rewrite to /operator (which server-redirects to /operator/suggestions)
  if (pathname === "/") {
    const url = request.nextUrl.clone();
    url.pathname = "/operator";
    return NextResponse.rewrite(url);
  }

  // Known operator paths → rewrite to /operator/* internally
  if (
    OPERATOR_PUBLIC_PATHS.some(
      (p) => pathname === p || pathname.startsWith(`${p}/`),
    )
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/operator${pathname}`;
    return NextResponse.rewrite(url);
  }

  // Already prefixed with /operator → pass through (handles direct /operator/* access)
  if (pathname.startsWith("/operator")) {
    return NextResponse.next();
  }

  // Everything else (e.g. /dashboard, /database/*) → redirect to root
  const url = request.nextUrl.clone();
  url.pathname = "/";
  return NextResponse.redirect(url);
}

function handleTenantSubdomain(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;

  // /operator/* on tenant subdomain → redirect to operator subdomain with clean path
  if (pathname.startsWith("/operator")) {
    const cleanPath = pathname.replace(/^\/operator/, "") || "/";
    const protocol = request.nextUrl.protocol;
    const redirectUrl = `${protocol}//${OPERATOR_HOSTNAME}${cleanPath}`;
    return NextResponse.redirect(redirectUrl);
  }

  return NextResponse.next();
}

export function middleware(request: NextRequest): NextResponse {
  const hostname = getHostname(request);

  if (isOperatorHost(hostname)) {
    return handleOperatorSubdomain(request);
  }

  return handleTenantSubdomain(request);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|images/).*)"],
};
