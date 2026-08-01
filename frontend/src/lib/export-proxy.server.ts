import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

import { auth, uncachedAuth } from "~/server/auth";
import { createLogger } from "~/lib/logger";

/**
 * Route handler for the export endpoints that answer with a file.
 *
 * These bypass the JSON route wrapper (it cannot stream bytes) and so each
 * one had grown the same forty lines: authenticate, POST, retry once on a
 * 401 with a refreshed token, pass the bytes and the Content-Disposition
 * through. That is here once.
 *
 * Server-only — import it dynamically from anything that could reach a
 * client bundle (route handlers are safe to import it statically).
 */
export function createFileExportRoute(options: {
  /** Backend path, e.g. "/api/staff-shifts/export". */
  backendPath: string;
  /** Logger component name, e.g. "DienstplanExportRoute". */
  component: string;
  /**
   * Optional body rewrite between client and backend. Throwing rejects the
   * request as a 400, which is what a malformed body is — the outer catch
   * would otherwise report the client's mistake as a server failure.
   */
  normalizeBody?: (raw: unknown) => unknown;
}) {
  const logger = createLogger({ component: options.component });

  async function proxy(token: string, body: string) {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const response = await fetch(`${getServerApiUrl()}${options.backendPath}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body,
      cache: "no-store",
    });

    if (!response.ok) {
      return NextResponse.json(
        { error: await response.text() },
        { status: response.status },
      );
    }

    const contentType =
      response.headers.get("content-type") ?? "application/octet-stream";
    const disposition = response.headers.get("content-disposition");
    const data = await response.arrayBuffer();
    const headers = new Headers({
      "Content-Type": contentType,
      "Content-Length": String(data.byteLength),
    });
    if (disposition) headers.set("Content-Disposition", disposition);
    return new NextResponse(data, { status: 200, headers });
  }

  return async function handler(request: NextRequest) {
    try {
      const session = await auth();
      if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
      }

      let body = await request.text();
      if (options.normalizeBody) {
        try {
          body = JSON.stringify(
            options.normalizeBody(body ? JSON.parse(body) : {}),
          );
        } catch {
          return NextResponse.json(
            { error: "Ungültiger Anfrage-Text." },
            { status: 400 },
          );
        }
      }

      const response = await proxy(session.user.token, body);
      if (response.status !== 401) return response;

      // The access token lives 15 minutes; a single refreshed retry keeps a
      // long-open planning tab from failing an export it is allowed to run.
      const refreshed = await uncachedAuth();
      if (
        !refreshed?.user?.token ||
        refreshed.user.token === session.user.token
      ) {
        return response;
      }
      return proxy(refreshed.user.token, body);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Export failed";
      logger.error("file_export_route_failed", { error: message });
      return NextResponse.json({ error: message }, { status: 500 });
    }
  };
}
