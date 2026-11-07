import type { NextRequest } from "next/server";
import { problemResponse } from "~/lib/api-response-error.server";
import { NextResponse } from "next/server";

import { auth, uncachedAuth } from "~/server/auth";
import { incomingClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";

/**
 * Route handlers for the export endpoints that answer with a file.
 *
 * These bypass the JSON route wrapper (it cannot stream bytes): authenticate,
 * send, repeat once with the refreshed session after a 401, pass the bytes
 * and the Content-Disposition through.
 *
 * Server-only — import it dynamically from anything that could reach a
 * client bundle (route handlers are safe to import it statically).
 */

type RouteParams = Record<string, string>;

async function bufferedFileResponse(
  response: Response,
  request: NextRequest,
): Promise<Response> {
  if (!response.ok) return forwardBackendResponse(response, request);
  const data = await response.arrayBuffer();
  const headers = new Headers({
    "Content-Type":
      response.headers.get("content-type") ?? "application/octet-stream",
    "Content-Length": String(data.byteLength),
  });
  const disposition = response.headers.get("content-disposition");
  if (disposition) headers.set("Content-Disposition", disposition);
  return new NextResponse(data, { status: 200, headers });
}

function createExportHandler<P extends RouteParams>(
  component: string,
  failureEvent: string,
  prepare: (
    request: NextRequest,
    params: P,
  ) => Promise<{ path: string; init: RequestInit } | Response>,
) {
  const logger = createLogger({ component });
  return async function handler(
    request: NextRequest,
    context?: { params: Promise<P> },
  ) {
    try {
      const session = await auth();
      const token = session?.user?.token;
      if (!token) return problemResponse(401, "Unauthorized", request);

      const prepared = await prepare(
        request,
        ((await context?.params) ?? {}) as P,
      );
      if (prepared instanceof Response) return prepared;

      const { getServerApiUrl } = await import("~/lib/server-api-url");
      const forwardHeaders = await incomingClientForwardHeaders();
      const send = (bearer: string) =>
        fetch(`${getServerApiUrl()}${prepared.path}`, {
          ...prepared.init,
          headers: {
            Authorization: `Bearer ${bearer}`,
            ...(prepared.init.headers as Record<string, string> | undefined),
            ...forwardHeaders,
          },
          cache: "no-store",
        });

      return await bufferedFileResponse(
        await sendWithTokenRefresh(token, send, () => uncachedAuth()),
        request,
      );
    } catch (error) {
      captureBffException(error, request);
      const message = error instanceof Error ? error.message : "Export failed";
      logger.error(failureEvent, { error: message });
      return problemResponse(500, message, request);
    }
  };
}

/** POST export: forwards the JSON request body to the backend. */
export function createFileExportRoute<
  P extends RouteParams = RouteParams,
>(options: {
  /** Backend path, e.g. "/api/staff-shifts/export", or built from the route params. Encode each segment. */
  backendPath: string | ((params: P) => string);
  /** Logger component name, e.g. "DienstplanExportRoute". */
  component: string;
  /**
   * Optional body rewrite between client and backend. Throwing rejects the
   * request as a 400, which is what a malformed body is — the outer catch
   * would otherwise report the client's mistake as a server failure.
   */
  normalizeBody?: (raw: unknown) => unknown;
}) {
  return createExportHandler<P>(
    options.component,
    "file_export_route_failed",
    async (request, params) => {
      let body = await request.text();
      if (options.normalizeBody) {
        try {
          body = JSON.stringify(
            options.normalizeBody(body ? JSON.parse(body) : {}),
          );
        } catch {
          return problemResponse(
            400,
            "request body is not valid JSON",
            request,
          );
        }
      }
      const path =
        typeof options.backendPath === "string"
          ? options.backendPath
          : options.backendPath(params);
      return {
        path,
        init: {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body,
        },
      };
    },
  );
}

/**
 * GET export whose backend path carries the object id
 * (`/api/surveys/{id}/results/export`). Query strings are not forwarded; the
 * path is the whole request. The file leaves with `Cache-Control: no-store`.
 */
export function createFileDownloadRoute<P extends RouteParams>(options: {
  /** Backend path built from the route params. Encode each segment. */
  backendPath: (params: P) => string;
  /** Logger component name, e.g. "SurveyResultsExportRoute". */
  component: string;
}) {
  const handler = createExportHandler<P>(
    options.component,
    "file_download_route_failed",
    async (_request, params) => ({
      path: options.backendPath(params),
      init: { method: "GET" },
    }),
  );
  return async (request: NextRequest, context: { params: Promise<P> }) => {
    const response = await handler(request, context);
    if (response.ok) response.headers.set("Cache-Control", "no-store");
    return response;
  };
}
