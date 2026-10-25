import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { traegerAuth } from "~/server/auth/traeger";
import { withTraegerAuth } from "~/server/auth/traeger-route";
import { incomingAnalyticsSessionHeaders } from "../analytics-session-header.server";
import { handleApiError } from "../api-helpers.server";
import {
  backendResponseError,
  isApiResponseError,
} from "../api-response-error.server";
import {
  type RouteContext,
  buildQueryString,
  createTokenExpiredResponse,
  createUnauthorizedResponse,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
} from "../route-wrapper-utils.server";

/**
 * Träger-portal route wrapper ("moto träger", #4649). Mirrors
 * lib/school/route-wrapper.server.ts, but reads the Träger NextAuth session
 * (traeger.session-token) so a Träger route can never forward a tenant,
 * parent, or school token. Reads the schools and settings of the Träger and
 * writes its settings (#4652).
 */

type Params = Record<string, unknown>;

async function traegerServerFetch<T>(
  endpoint: string,
  token: string,
  options: { readonly method: string; readonly body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(`${getServerApiUrl()}${endpoint}`, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(await incomingAnalyticsSessionHeaders()),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    throw await backendResponseError(response);
  }
  if (response.status === 204) {
    return null as T;
  }

  const json: unknown = await response.json();
  // Unwrap the backend envelope { status, data, message } of common.Respond().
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "status" in json
  ) {
    return (json as { data: T }).data;
  }
  return json as T;
}

/** PUT to the backend with the Träger token; unwraps `data`. */
export function traegerApiPut<T>(
  endpoint: string,
  token: string,
  body: unknown,
): Promise<T> {
  return traegerServerFetch<T>(endpoint, token, { method: "PUT", body });
}

/** DELETE at the backend with the Träger token. */
export function traegerApiDelete(
  endpoint: string,
  token: string,
): Promise<unknown> {
  return traegerServerFetch<unknown>(endpoint, token, { method: "DELETE" });
}

/**
 * Retries once with a refreshed Träger session after a backend 401. Null
 * means there is no fresh session; a retried answer may itself be null (a
 * write without content), so it travels wrapped.
 */
async function retryWithRefreshedToken<T>(
  originalToken: string,
  retry: (token: string) => Promise<T>,
): Promise<{ readonly value: T } | null> {
  const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
  const updated = await uncachedTraegerAuth();
  if (!updated?.user?.token || updated.user.token === originalToken) {
    return null;
  }
  return { value: await retry(updated.user.token) };
}

/**
 * Runs a Träger route: requires the Träger session, retries once after a
 * backend 401 with a refreshed session, and answers every failure with the
 * shared error envelope. `readBody` parses the JSON body once per request,
 * before the first attempt, so the retry sends the same body.
 */
function traegerRoute<T, B>(
  readBody: (request: NextRequest) => Promise<B>,
  run: (
    request: NextRequest,
    body: B,
    token: string,
    params: Params,
  ) => Promise<T>,
  respond: (data: T) => Response,
) {
  return withTraegerAuth(
    async (request: NextRequest, context: RouteContext) => {
      try {
        const session = await traegerAuth();
        const token = session?.user?.token;
        if (!token) return createUnauthorizedResponse(request);

        const params = await extractParams(request, context);
        const body = await readBody(request);
        const load = (current: string) => run(request, body, current, params);
        try {
          return respond(await load(token));
        } catch (error) {
          if (!isApiResponseError(error, 401)) throw error;
          try {
            const retried = await retryWithRefreshedToken(token, load);
            if (retried !== null) return respond(retried.value);
          } catch {
            // The retry failed as well: the session counts as expired.
          }
          return createTokenExpiredResponse(request);
        }
      } catch (error) {
        return handleApiError(error, request);
      }
    },
  );
}

const noBody = async (): Promise<undefined> => undefined;

const jsonResponse = <T>(data: T) => NextResponse.json(wrapInApiResponse(data));

/** GET proxy for a Träger route: forwards the query, unwraps `data`. */
export function proxyGet<T>(endpoint: string) {
  return traegerRoute(
    noBody,
    (request, _body, token) =>
      traegerServerFetch<T>(`${endpoint}${buildQueryString(request)}`, token, {
        method: "GET",
      }),
    jsonResponse,
  );
}

/** PUT handler for a Träger route; the body is parsed JSON. */
export function createTraegerPutHandler<T, B = unknown>(
  handler: (
    request: NextRequest,
    body: B,
    token: string,
    params: Params,
  ) => Promise<T>,
) {
  return traegerRoute(
    (request) => parseRequestBody<B>(request),
    handler,
    jsonResponse,
  );
}

/** DELETE handler for a Träger route; answers 204 without content. */
export function createTraegerDeleteHandler(
  handler: (
    request: NextRequest,
    token: string,
    params: Params,
  ) => Promise<unknown>,
) {
  return traegerRoute(
    noBody,
    (request, _body, token, params) => handler(request, token, params),
    () => new NextResponse(null, { status: 204 }),
  );
}
