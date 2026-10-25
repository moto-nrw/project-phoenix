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
  buildQueryString,
  createTokenExpiredResponse,
  createUnauthorizedResponse,
  wrapInApiResponse,
} from "../route-wrapper-utils.server";

/**
 * Träger-portal route wrapper ("moto träger", #4649). Mirrors
 * lib/school/route-wrapper.server.ts, but reads the Träger NextAuth session
 * (traeger.session-token) so a Träger route can never forward a tenant,
 * parent, or school token. Only reads exist so far: the portal shows the
 * schools of the Träger and writes nothing.
 */

async function traegerServerGet<T>(
  endpoint: string,
  token: string,
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(`${getServerApiUrl()}${endpoint}`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(await incomingAnalyticsSessionHeaders()),
    },
  });

  if (!response.ok) {
    throw await backendResponseError(response);
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

/** Retries once with a refreshed Träger session after a backend 401. */
async function retryWithRefreshedToken<T>(
  originalToken: string,
  retry: (token: string) => Promise<T>,
): Promise<T | null> {
  const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
  const updated = await uncachedTraegerAuth();
  if (!updated?.user?.token || updated.user.token === originalToken) {
    return null;
  }
  return retry(updated.user.token);
}

/** GET proxy for a Träger route: forwards the query, unwraps `data`. */
export function proxyGet<T>(endpoint: string) {
  return withTraegerAuth(async (request: NextRequest) => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);

      const target = `${endpoint}${buildQueryString(request)}`;
      const load = (current: string) => traegerServerGet<T>(target, current);
      try {
        return NextResponse.json(wrapInApiResponse(await load(token)));
      } catch (error) {
        if (!isApiResponseError(error, 401)) throw error;
        try {
          const retried = await retryWithRefreshedToken(token, load);
          if (retried !== null) {
            return NextResponse.json(wrapInApiResponse(retried));
          }
        } catch {
          // The retry failed as well: the session counts as expired.
        }
        return createTokenExpiredResponse(request);
      }
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}
