/**
 * Utilities for school subdomain URL handling ("moto schule", #2207).
 *
 * On the school subdomain (e.g. schule.moto-app.de), URLs use clean paths
 * like /login instead of /school/login. The proxy rewrites these to the
 * actual /school/* routes internally.
 *
 * Mirrors parent-url.ts; see operator-url.ts for the reference shape.
 */

// Cache the result — window.location.host and the env var never change at runtime
let _isSchool: boolean | undefined;

function isSchoolSubdomain(): boolean {
  if (typeof window === "undefined") return false;
  if (_isSchool !== undefined) return _isSchool;

  const schoolHostname = process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME;
  if (!schoolHostname) {
    throw new Error(
      "NEXT_PUBLIC_SCHOOL_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  _isSchool = window.location.host === schoolHostname;
  return _isSchool;
}

/**
 * Returns the correct path for school-portal navigation:
 * - On school subdomain: strips /school prefix (clean URLs)
 * - On other hosts: keeps /school prefix (so the proxy can redirect)
 */
export function schoolPath(path: string): string {
  if (isSchoolSubdomain()) {
    const stripped = path.replace(/^\/school/, "");
    // Nach dem Abschneiden muss ein Pfad übrig bleiben. Bleibt nichts oder
    // nur ein Query- bzw. Fragment-Teil ("/school?tag=..."), wäre das
    // Ergebnis relativ zur aktuellen Seite: der Link führte dann nirgendwo
    // hin statt auf die Startseite des Portals (#2294).
    if (
      stripped === "" ||
      stripped.startsWith("?") ||
      stripped.startsWith("#")
    ) {
      return `/${stripped}`;
    }
    return stripped;
  }
  return path.startsWith("/school") ? path : `/school${path}`;
}

/**
 * Absolute URL of the school-portal login page on its OWN host.
 *
 * Different from schoolAbsoluteUrl(), which stays on the current origin.
 * This one crosses hosts: the tenant (staff) login uses it to send a
 * school-portal-only account to the portal it actually belongs to — the
 * mirror of parentsPortalLoginUrl in parent-url.ts.
 *
 * NEXT_PUBLIC_SCHOOL_HOSTNAME is a host authority and already carries the
 * port (e.g. "schule.localhost:3000"), so no port is appended. The scheme
 * follows the current page so local http stays http.
 *
 * Client-side only — throws on server.
 */
export function schoolPortalLoginUrl(search?: string): string {
  if (typeof window === "undefined") {
    throw new Error("schoolPortalLoginUrl() is client-only.");
  }

  const schoolHostname = process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME;
  if (!schoolHostname) {
    throw new Error(
      "NEXT_PUBLIC_SCHOOL_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  return `${window.location.protocol}//${schoolHostname}/login${search ?? ""}`;
}

/**
 * Returns an absolute URL for school paths.
 * Use for NextAuth callbackUrl where relative paths resolve against
 * NEXTAUTH_URL. Client-side only — throws on server.
 */
export function schoolAbsoluteUrl(path: string): string {
  if (typeof window === "undefined") {
    throw new Error(
      "schoolAbsoluteUrl() is client-only. Use schoolPath() on the server.",
    );
  }
  return `${window.location.origin}${schoolPath(path)}`;
}
