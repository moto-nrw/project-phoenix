import type { NextRequest } from "next/server";

function firstHeaderValue(value: string | null): string {
  return value?.split(",")[0]?.trim() ?? "";
}

function isSafeForwardedHost(value: string): boolean {
  return value !== "" && !value.includes("/") && !value.includes("\\");
}

function frontendOrigin(request: NextRequest): string {
  const host =
    firstHeaderValue(request.headers.get("x-moto-original-host")) ||
    firstHeaderValue(request.headers.get("host")) ||
    firstHeaderValue(request.headers.get("x-forwarded-host"));

  if (!isSafeForwardedHost(host)) {
    return request.nextUrl.origin;
  }

  const forwardedProto =
    firstHeaderValue(request.headers.get("x-moto-original-proto")) ||
    firstHeaderValue(request.headers.get("x-forwarded-proto"));
  const protocol =
    forwardedProto === "http" || forwardedProto === "https"
      ? forwardedProto
      : request.nextUrl.protocol.replace(/:$/, "");

  return `${protocol}://${host}`;
}

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
    "X-Moto-Frontend-Origin": frontendOrigin(request),
    "User-Agent": userAgent,
  };
}
