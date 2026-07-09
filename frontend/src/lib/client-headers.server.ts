import { isIP } from "node:net";
import type { NextRequest } from "next/server";

type HeaderReader = Pick<Headers, "get">;

function firstHeaderValue(value: string | null | undefined): string {
  return (
    value
      ?.split(",")
      .map((part) => part.trim())
      .find(Boolean) ?? ""
  );
}

function lastHeaderValue(value: string | null | undefined): string {
  const parts = value?.split(",") ?? [];
  for (let index = parts.length - 1; index >= 0; index -= 1) {
    const part = parts[index]?.trim();
    if (part) return part;
  }
  return "";
}

function isValidHeaderIP(value: string): boolean {
  return isIP(value) !== 0;
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
  const forwardedFor = canonicalForwardedFor(request.headers);
  const userAgent = request.headers.get("user-agent") ?? "unknown";

  return {
    ...(forwardedFor && { "X-Forwarded-For": forwardedFor }),
    "X-Moto-Frontend-Origin": frontendOrigin(request),
    "User-Agent": userAgent,
  };
}

export function canonicalForwardedFor(headers: HeaderReader | null): string {
  const forwardedFor = lastHeaderValue(headers?.get("x-forwarded-for"));
  if (forwardedFor) {
    return isValidHeaderIP(forwardedFor) ? forwardedFor : "";
  }
  const realIp = firstHeaderValue(headers?.get("x-real-ip"));
  return isValidHeaderIP(realIp) ? realIp : "";
}
