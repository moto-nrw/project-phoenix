/**
 * The Vorgangskennung of one request (CONTEXT.md). The proxy assigns a fresh
 * one to every request it passes on, the BFF forwards it to the backend,
 * which keeps a caller-supplied UUID, and both put it on their Sentry events
 * as `request_id`. So one ID finds the same request in every layer.
 */
export const REQUEST_ID_HEADER = "X-Request-ID";

/** Returns the proxy-assigned Vorgangskennung of the current request. */
export function requestInstance(
  request?: Pick<Request, "headers">,
): string | undefined {
  return request?.headers?.get(REQUEST_ID_HEADER) || undefined;
}
