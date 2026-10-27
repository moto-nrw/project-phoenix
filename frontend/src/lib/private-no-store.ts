/**
 * Cache policy for BFF answers that carry session- or token-bound data
 * (#4999). Next sets no Cache-Control on dynamic route handlers, so without
 * it a shared school device may keep children's data in its disk cache.
 */
const PRIVATE_NO_STORE = "private, no-store";

const JSON_CONTENT_TYPE = /^application\/(?:[\w.+-]+\+)?json\s*(?:;|$)/i;

/**
 * Marks a JSON or bodyless answer (204, redirect) as `private, no-store`.
 * An answer that already sets Cache-Control keeps it, and files, images and
 * event streams keep their own headers.
 */
export function applyPrivateNoStore(headers: Headers): void {
  if (headers.has("Cache-Control")) return;
  const contentType = headers.get("Content-Type");
  if (contentType !== null && !JSON_CONTENT_TYPE.test(contentType)) return;
  headers.set("Cache-Control", PRIVATE_NO_STORE);
}
