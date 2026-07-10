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

const dnsLabelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

/**
 * Parses a host authority and returns a canonical hostname without its port.
 * Malformed or ambiguous authorities are rejected instead of being repaired:
 * callers use this value for portal isolation, where a false match is unsafe.
 */
export function hostnameFromAuthority(authority: string): string | null {
  if (
    authority === "" ||
    /[\s/@\\?#,%]/.test(authority) ||
    authority.endsWith(":")
  ) {
    return null;
  }

  try {
    const parsed = new URL(`http://${authority}`);
    if (
      parsed.username !== "" ||
      parsed.password !== "" ||
      parsed.pathname !== "/" ||
      parsed.search !== "" ||
      parsed.hash !== ""
    ) {
      return null;
    }

    let hostname = parsed.hostname.toLowerCase();
    if (hostname.startsWith("[") && hostname.endsWith("]")) {
      hostname = hostname.slice(1, -1);
      return isIP(hostname) === 6 ? hostname : null;
    }
    if (isIP(hostname) === 4) return hostname;

    hostname = hostname.replace(/\.$/, "");
    if (
      hostname === "" ||
      hostname.length > 253 ||
      !hostname.split(".").every((label) => dnsLabelPattern.test(label))
    ) {
      return null;
    }
    return hostname;
  } catch {
    return null;
  }
}

/**
 * Returns the browser-facing host captured by the application proxy. Route
 * handlers normally see an internal Next.js request target after rewrites,
 * so security boundaries must prefer the proxy-owned original-host header.
 */
export function getOriginalRequestHost(request: NextRequest): string {
  return (
    firstHeaderValue(request.headers.get("x-moto-original-host")) ||
    firstHeaderValue(request.headers.get("host")) ||
    firstHeaderValue(request.headers.get("x-forwarded-host"))
  );
}

function frontendOrigin(request: NextRequest): string {
  const host = getOriginalRequestHost(request);

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
