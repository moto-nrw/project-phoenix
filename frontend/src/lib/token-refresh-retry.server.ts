import type { NextResponse } from "next/server";
import type { ProblemBody } from "./api-error";
import { isApiResponseError } from "./api-response-error.server";
import { createTokenExpiredResponse } from "./route-wrapper-utils.server";

/**
 * Reads the portal's session past the request cache. Every portal has its
 * own session (cookie, scope); the wrapper passes its getter in.
 */
export type FreshSessionReader = () => Promise<
  { user?: { token?: string | null } | null } | null | undefined
>;

/** The session's token when it differs from the one that failed, else null. */
async function refreshedToken(
  originalToken: string,
  readFreshSession: FreshSessionReader,
): Promise<string | null> {
  const token = (await readFreshSession())?.user?.token;
  return token && token !== originalToken ? token : null;
}

/**
 * Runs a handler that throws on backend errors and, after a backend 401,
 * once more with the refreshed session token. Only a failed or missing
 * refresh or a second 401 counts as an expired session; any other answer of
 * the retry, including a null result (a DELETE's 204), is the answer of the
 * request.
 */
export async function executeWithTokenRefresh<T, R>(
  request: Request,
  token: string,
  execute: (token: string) => Promise<T>,
  formatResponse: (data: T) => R,
  readFreshSession: FreshSessionReader,
): Promise<R | NextResponse<ProblemBody>> {
  try {
    return formatResponse(await execute(token));
  } catch (error) {
    if (!isApiResponseError(error, 401)) throw error;
  }
  let fresh: string | null = null;
  try {
    fresh = await refreshedToken(token, readFreshSession);
  } catch {
    // The refresh itself failed: the session counts as expired.
  }
  if (fresh === null) return createTokenExpiredResponse(request);
  try {
    return formatResponse(await execute(fresh));
  } catch (retryError) {
    if (isApiResponseError(retryError, 401)) {
      return createTokenExpiredResponse(request);
    }
    throw retryError;
  }
}

/**
 * Sends a backend request and repeats it once with the refreshed token when
 * the backend answers 401 and the session now holds a newer token. For
 * handlers that hold the raw Response (files, uploads, verbatim proxies).
 */
export async function sendWithTokenRefresh(
  token: string,
  send: (bearer: string) => Promise<Response>,
  readFreshSession: FreshSessionReader,
): Promise<Response> {
  const response = await send(token);
  if (response.status !== 401) return response;
  const fresh = await refreshedToken(token, readFreshSession);
  return fresh === null ? response : send(fresh);
}
