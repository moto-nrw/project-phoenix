/**
 * BetterAuth Proxy Route
 *
 * This catch-all route proxies authentication requests to the BetterAuth service.
 * Using a proxy eliminates CORS issues since all requests are same-origin from
 * the browser's perspective.
 *
 * Flow: Browser → Next.js (/api/auth/*) → BetterAuth (localhost:3001)
 *
 * This route handles all BetterAuth endpoints:
 * - POST /api/auth/sign-in/email
 * - POST /api/auth/sign-up/email
 * - POST /api/auth/sign-out
 * - GET  /api/auth/session
 * - Organization endpoints (from plugin)
 * - etc.
 *
 * Note: Specific routes like /api/auth/login take precedence and proxy to
 * the Go backend instead.
 */

import { type NextRequest, NextResponse } from "next/server";

// BetterAuth service URL (internal, not exposed to client)
const BETTERAUTH_URL =
  process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";

/**
 * Proxy a request to the BetterAuth service.
 */
async function proxyToBetterAuth(
  request: NextRequest,
  method: string,
): Promise<NextResponse> {
  try {
    // Get the path after /api/auth/
    const url = new URL(request.url);
    const targetUrl = `${BETTERAUTH_URL}${url.pathname}${url.search}`;

    // Prepare headers - forward relevant ones
    const headers = new Headers();
    headers.set(
      "Content-Type",
      request.headers.get("Content-Type") ?? "application/json",
    );

    // Forward Origin header for CSRF protection
    // BetterAuth requires Origin to match trustedOrigins
    const origin = request.headers.get("Origin");
    if (origin) {
      headers.set("Origin", origin);
    }

    // Forward Referer header (some auth flows check this)
    const referer = request.headers.get("Referer");
    if (referer) {
      headers.set("Referer", referer);
    }

    // Forward cookies for session management
    const cookies = request.headers.get("Cookie");
    if (cookies) {
      headers.set("Cookie", cookies);
    }

    // Forward the request body for POST/PUT/PATCH
    let body: string | undefined;
    if (method !== "GET" && method !== "HEAD") {
      try {
        body = await request.text();
      } catch {
        // No body
      }
    }

    // Make the proxied request
    const response = await fetch(targetUrl, {
      method,
      headers,
      body: body ?? undefined,
      // Don't follow redirects - let the client handle them
      redirect: "manual",
    });

    // Create response with BetterAuth's response
    const responseHeaders = new Headers();

    // Forward content type
    const contentType = response.headers.get("Content-Type");
    if (contentType) {
      responseHeaders.set("Content-Type", contentType);
    }

    // Forward Set-Cookie headers for session cookies
    // BetterAuth sets cookies for session management
    const setCookies = response.headers.getSetCookie();
    for (const cookie of setCookies) {
      responseHeaders.append("Set-Cookie", cookie);
    }

    // Forward Location header for redirects
    const location = response.headers.get("Location");
    if (location) {
      responseHeaders.set("Location", location);
    }

    // Get response body
    const responseBody = await response.text();

    return new NextResponse(responseBody || null, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });
  } catch (error) {
    console.error("BetterAuth proxy error:", error);
    return NextResponse.json(
      { error: "Authentication service unavailable" },
      { status: 503 },
    );
  }
}

// Handle all HTTP methods
export async function GET(request: NextRequest) {
  return proxyToBetterAuth(request, "GET");
}

export async function POST(request: NextRequest) {
  return proxyToBetterAuth(request, "POST");
}

export async function PUT(request: NextRequest) {
  return proxyToBetterAuth(request, "PUT");
}

export async function PATCH(request: NextRequest) {
  return proxyToBetterAuth(request, "PATCH");
}

export async function DELETE(request: NextRequest) {
  return proxyToBetterAuth(request, "DELETE");
}

export async function OPTIONS(request: NextRequest) {
  return proxyToBetterAuth(request, "OPTIONS");
}
