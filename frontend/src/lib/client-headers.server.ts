import type { NextRequest } from "next/server";

/**
 * Extracts the real client IP and User-Agent from an incoming request
 * and returns headers that should be forwarded to the backend.
 *
 * When Next.js proxies requests to the Go backend over the Docker network,
 * the backend only sees the Docker-internal IP (e.g. 172.20.0.4) and
 * Node.js as the User-Agent. This helper preserves the original values
 * so the backend can log them for security auditing.
 */
export function getClientForwardHeaders(
  request: NextRequest,
): Record<string, string> {
  const ip =
    request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ??
    request.headers.get("x-real-ip") ??
    "unknown";

  const userAgent = request.headers.get("user-agent") ?? "unknown";

  return {
    "X-Forwarded-For": ip,
    "X-Real-IP": ip,
    "X-Moto-Frontend-Origin": request.nextUrl.origin,
    "User-Agent": userAgent,
  };
}
