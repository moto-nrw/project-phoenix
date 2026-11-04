/**
 * Cache policy for BFF answers that carry session- or token-bound data
 * (#4999). Next sets no Cache-Control on dynamic route handlers, so without
 * it a shared school device may keep children's data in its disk cache.
 */
const PRIVATE_NO_STORE = "private, no-store";

const JSON_CONTENT_TYPE = /^application\/(?:[\w.+-]+\+)?json\s*(?:;|$)/i;

function needsPrivateNoStore(headers: Headers): boolean {
  if (headers.has("Cache-Control")) return false;
  const contentType = headers.get("Content-Type");
  return contentType === null || JSON_CONTENT_TYPE.test(contentType);
}

/**
 * Marks a JSON or bodyless answer (204, redirect) as `private, no-store`.
 * An answer that already sets Cache-Control keeps it, and files, images and
 * event streams keep their own headers.
 */
export function applyPrivateNoStore(headers: Headers): void {
  if (needsPrivateNoStore(headers)) {
    headers.set("Cache-Control", PRIVATE_NO_STORE);
  }
}

/**
 * Applies {@link applyPrivateNoStore} to a route response. A response with
 * immutable headers (a fetch result handed through as is) is copied first.
 */
function withPrivateNoStoreResponse<R extends Response>(
  response: R,
  download = false,
): R {
  if (!download && !needsPrivateNoStore(response.headers)) return response;
  try {
    response.headers.set("Cache-Control", PRIVATE_NO_STORE);
    return response;
  } catch {
    const copy = new Response(response.body, response);
    copy.headers.set("Cache-Control", PRIVATE_NO_STORE);
    return copy as R;
  }
}

/**
 * Route-handler wrapper for BFF routes that run through no portal wrapper or
 * proxy factory (login, MFA, passkeys, demo access, invitations, Auth.js
 * endpoints). `route-cache-policy.test.ts` requires one of them per route.
 */
export function withPrivateNoStore<A extends unknown[], R extends Response>(
  handler: (...args: A) => R | Promise<R>,
): (...args: A) => Promise<R> {
  return async (...args) => withPrivateNoStoreResponse(await handler(...args));
}

/**
 * Explicit policy for private downloads, including their error responses.
 * Unlike the JSON policy, it covers every MIME type and overrides upstream
 * caching. Wrap the portal handler, not just its success callback, so auth
 * failures and refreshed session cookies leave through the same boundary.
 * The body is never read or buffered; public assets, feeds and SSE do not
 * use this wrapper.
 */
export function withPrivateDownloadNoStore<
  A extends unknown[],
  R extends Response,
>(handler: (...args: A) => R | Promise<R>): (...args: A) => Promise<R> {
  return async (...args) =>
    withPrivateNoStoreResponse(await handler(...args), true);
}
