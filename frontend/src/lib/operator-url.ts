/**
 * Utilities for operator subdomain URL handling.
 *
 * On the operator subdomain (e.g. operator.moto-app.de), URLs use clean paths
 * like /suggestions instead of /operator/suggestions. The middleware rewrites
 * these to the actual /operator/* routes internally.
 */

export function isOperatorSubdomain(): boolean {
  if (typeof window === "undefined") return false;
  const operatorHostname =
    process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME ?? "operator.localhost:3000";
  return window.location.host === operatorHostname;
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
 * Use this for NextAuth callbackUrl and other cases where a relative path
 * would be resolved against NEXTAUTH_URL instead of the current origin.
 */
export function operatorAbsoluteUrl(path: string): string {
  if (typeof window === "undefined") return operatorPath(path);
  return `${window.location.origin}${operatorPath(path)}`;
}
