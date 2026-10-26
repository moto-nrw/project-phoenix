/**
 * Utilities for operator subdomain URL handling.
 *
 * On the operator subdomain (e.g. operator.moto-app.de), URLs use clean paths
 * like /organizations instead of /operator/organizations. The proxy rewrites
 * these to the actual /operator/* routes internally.
 */

// Cache the result — window.location.host and the env var never change at runtime
let _isOperator: boolean | undefined;

export function isOperatorSubdomain(): boolean {
  // Server rendering of this portal only happens on its own host: the
  // proxy redirects /operator/* on every other host. Answering "own host"
  // keeps server HTML and hydration on the same paths, now that the
  // layout renders the signed-in portal on the server (#4991).
  if (typeof window === "undefined") return true;
  if (_isOperator !== undefined) return _isOperator;

  const operatorHostname = process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME;
  if (!operatorHostname) {
    throw new Error(
      "NEXT_PUBLIC_OPERATOR_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  _isOperator = window.location.host === operatorHostname;
  return _isOperator;
}

/**
 * Returns the correct path for operator navigation:
 * - On operator subdomain: strips /operator prefix (clean URLs)
 * - On tenant subdomain: keeps /operator prefix (legacy paths)
 */
export function operatorPath(path: string): string {
  if (isOperatorSubdomain()) {
    return path.replace(/^\/operator/, "") || "/";
  }
  return path.startsWith("/operator") ? path : `/operator${path}`;
}

/**
 * Returns an absolute URL for operator paths.
 * Use for NextAuth callbackUrl where relative paths resolve against NEXTAUTH_URL.
 * Client-side only — throws on server.
 */
export function operatorAbsoluteUrl(path: string): string {
  if (typeof window === "undefined") {
    throw new Error(
      "operatorAbsoluteUrl() is client-only. Use operatorPath() on the server.",
    );
  }
  return `${window.location.origin}${operatorPath(path)}`;
}
