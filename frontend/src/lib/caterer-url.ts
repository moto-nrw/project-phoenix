/**
 * Paths of the caterer portal (ADR 0074). On the caterer host the browser
 * shows clean paths (/login); the proxy rewrites them to /caterer/*. Mirrors
 * school-url.ts.
 */

let isCatererHost: boolean | undefined;

function onCatererHost(): boolean {
  if (typeof window === "undefined") return false;
  if (isCatererHost !== undefined) return isCatererHost;
  const hostname = process.env.NEXT_PUBLIC_CATERER_HOSTNAME;
  if (!hostname) {
    throw new Error(
      "NEXT_PUBLIC_CATERER_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }
  isCatererHost = window.location.host === hostname;
  return isCatererHost;
}

/** The path of a caterer page, without /caterer on the caterer host. */
export function catererPath(path: string): string {
  if (onCatererHost()) {
    const stripped = path.replace(/^\/caterer/, "");
    return stripped === "" || /^[?#]/.test(stripped)
      ? `/${stripped}`
      : stripped;
  }
  return path.startsWith("/caterer") ? path : `/caterer${path}`;
}

/** Absolute URL of a caterer page, for NextAuth's callbackUrl. */
export function catererAbsoluteUrl(path: string): string {
  if (typeof window === "undefined") {
    throw new Error("catererAbsoluteUrl() is client-only.");
  }
  return `${window.location.origin}${catererPath(path)}`;
}
