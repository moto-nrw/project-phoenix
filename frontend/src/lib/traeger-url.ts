/**
 * URL helpers for the Träger subdomain ("moto träger", #4649).
 *
 * On the Träger host (e.g. traeger.moto-app.de) the browser shows clean
 * paths like /login; the proxy rewrites them to the /traeger/* routes
 * internally. Mirrors school-url.ts.
 */

let _isTraeger: boolean | undefined;

function isTraegerSubdomain(): boolean {
  // Server rendering of this portal only happens on its own host: the
  // proxy redirects /traeger/* on every other host. Answering "own host"
  // keeps server HTML and hydration on the same paths, now that the
  // layout renders the signed-in portal on the server (#4991).
  if (typeof window === "undefined") return true;
  if (_isTraeger !== undefined) return _isTraeger;

  const traegerHostname = process.env.NEXT_PUBLIC_TRAEGER_HOSTNAME;
  if (!traegerHostname) {
    throw new Error(
      "NEXT_PUBLIC_TRAEGER_HOSTNAME is not set. " +
        "Add it to your .env.local or docker-compose environment.",
    );
  }

  _isTraeger = window.location.host === traegerHostname;
  return _isTraeger;
}

/**
 * Path for Träger-portal navigation: without the /traeger prefix on the
 * Träger host, with it elsewhere (so the proxy can redirect).
 */
export function traegerPath(path: string): string {
  if (isTraegerSubdomain()) {
    const stripped = path.replace(/^\/traeger/, "");
    if (
      stripped === "" ||
      stripped.startsWith("?") ||
      stripped.startsWith("#")
    ) {
      return `/${stripped}`;
    }
    return stripped;
  }
  return path.startsWith("/traeger") ? path : `/traeger${path}`;
}

/** Test hook: forget the cached host decision. */
export function _resetTraegerHostCache(): void {
  _isTraeger = undefined;
}
