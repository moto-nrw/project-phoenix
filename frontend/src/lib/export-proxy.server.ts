import type { NextRequest } from "next/server";
import { handleApiError, problemResponse } from "./api-response-error.server";
import { tenant } from "./route-wrapper.server";

/**
 * Tenant export routes that answer with a file. The route wraps them in
 * withTenantAuth and withPrivateDownloadNoStore. The file is read whole and
 * leaves with its Content-Length.
 */

type RouteParams = Record<string, string>;

/** POST export: forwards the JSON request body to the backend. */
export function createFileExportRoute<
  P extends RouteParams = RouteParams,
>(options: {
  /** Backend path, or built from the route params. Encode each segment. */
  backendPath: string | ((params: P) => string);
  /**
   * Optional body rewrite between client and backend. Throwing rejects the
   * request as a 400, which is what a malformed body is.
   */
  normalizeBody?: (raw: unknown) => unknown;
}) {
  return async (request: NextRequest, context?: { params: Promise<P> }) => {
    try {
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
      const params = ((await context?.params) ?? {}) as P;
      const path =
        typeof options.backendPath === "string"
          ? options.backendPath
          : options.backendPath(params);
      return await tenant.downloadFile(request, path, {
        method: "POST",
        body,
        contentType: "application/json",
        buffer: true,
        fallbackType: "application/octet-stream",
      });
    } catch (error) {
      return handleApiError(error, request);
    }
  };
}

/**
 * GET export whose backend path carries the object id
 * (`/api/surveys/{id}/results/export`). The query is not forwarded.
 */
export function createFileDownloadRoute<P extends RouteParams>(options: {
  /** Backend path built from the route params. Encode each segment. */
  backendPath: (params: P) => string;
}) {
  return async (request: NextRequest, context: { params: Promise<P> }) => {
    try {
      return await tenant.downloadFile(
        request,
        options.backendPath(await context.params),
        { buffer: true, fallbackType: "application/octet-stream" },
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  };
}
