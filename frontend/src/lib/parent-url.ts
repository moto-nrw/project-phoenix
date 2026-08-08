/**
 * Utilities for parents subdomain URL handling.
 *
 * On the parents subdomain (e.g. eltern.moto-app.de), URLs use clean
 * paths like /children/123 instead of /parents/children/123. The proxy
 * rewrites these to the actual /parents/* routes internally.
 *
 * Mirrors operator-url.ts; see that file for the reference shape.
 */

// Cache the result — window.location.host and the env var never change at runtime
let _isParents: boolean | undefined;

function isParentsSubdomain(): boolean {
  if (typeof window === "undefined") return false;
  if (_isParents !== undefined) return _isParents;

  const parentsHostname = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  if (!parentsHostname) {
    throw new Error(
      "NEXT_PUBLIC_PARENTS_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  _isParents = window.location.host === parentsHostname;
  return _isParents;
}

/**
 * Returns the correct path for parent navigation:
 * - On parents subdomain: strips /parents prefix (clean URLs)
 * - On other hosts: keeps /parents prefix (so the proxy can redirect)
 */
export function parentPath(path: string): string {
  if (isParentsSubdomain()) {
    return path.replace(/^\/parents/, "") || "/";
  }
  return path.startsWith("/parents") ? path : `/parents${path}`;
}

/**
 * Absolute URL of the parents-portal login page on its OWN host.
 *
 * Different from parentAbsoluteUrl(), which stays on the current origin.
 * This one crosses hosts: it is used from the tenant (staff) login to send
 * a guardian-only account to the portal it actually belongs to.
 *
 * NEXT_PUBLIC_PARENTS_HOSTNAME is a host authority and already carries the
 * port (e.g. "parents.localhost:3000"), so no port is appended. The scheme
 * follows the current page so local http stays http.
 *
 * Client-side only — throws on server.
 */
export function parentsPortalLoginUrl(search?: string): string {
  if (typeof window === "undefined") {
    throw new Error("parentsPortalLoginUrl() is client-only.");
  }

  const parentsHostname = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  if (!parentsHostname) {
    throw new Error(
      "NEXT_PUBLIC_PARENTS_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  return `${window.location.protocol}//${parentsHostname}/login${search ?? ""}`;
}

/**
 * Returns an absolute URL for parent paths.
 * Use for NextAuth callbackUrl where relative paths resolve against NEXTAUTH_URL.
 * Client-side only — throws on server.
 */
export function parentAbsoluteUrl(path: string): string {
  if (typeof window === "undefined") {
    throw new Error(
      "parentAbsoluteUrl() is client-only. Use parentPath() on the server.",
    );
  }
  return `${window.location.origin}${parentPath(path)}`;
}
