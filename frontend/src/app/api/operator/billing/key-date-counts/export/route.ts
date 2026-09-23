import type { NextRequest } from "next/server";
import { createLogger } from "~/lib/logger";
import { getServerApiUrl } from "~/lib/server-api-url";
import {
  operatorAuth,
  uncachedOperatorAuth,
  withOperatorAuth,
} from "~/server/auth/operator";

const logger = createLogger({ component: "OperatorBillingExportRoute" });

/**
 * Streams the billing key-date CSV (#2791) from the backend as a download.
 * It bypasses the JSON route wrapper because the body is a file with its own
 * Content-Disposition, not an envelope.
 */
function exportQuery(request: NextRequest): string {
  const month = request.nextUrl.searchParams.get("month");
  return month ? `?month=${encodeURIComponent(month)}` : "";
}

async function proxyExport(query: string, token: string): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}/operator/billing/key-date-counts/export${query}`,
    {
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
    },
  );

  if (!backendResponse.ok) {
    const body = await backendResponse.text().catch(() => "");
    return new Response(body || "Export failed", {
      status: backendResponse.status,
      headers: { "Cache-Control": "no-store" },
    });
  }
  if (!backendResponse.body) {
    return new Response("No response body from backend", { status: 502 });
  }

  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const name of [
    "Content-Type",
    "Content-Disposition",
    "Content-Length",
  ]) {
    const value = backendResponse.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(backendResponse.body, {
    status: backendResponse.status,
    headers,
  });
}

async function GETHandler(request: NextRequest) {
  try {
    const session = await operatorAuth();
    if (!session?.user?.token) {
      return new Response("Unauthorized", { status: 401 });
    }

    const query = exportQuery(request);
    const response = await proxyExport(query, session.user.token);
    if (response.status !== 401) return response;

    // The access token may have expired between page load and click; one
    // retry with a refreshed session, as the JSON routes do.
    const refreshed = await uncachedOperatorAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return await proxyExport(query, refreshed.user.token);
  } catch (error) {
    logger.error("operator billing export proxy failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}

export const GET = withOperatorAuth(GETHandler);
