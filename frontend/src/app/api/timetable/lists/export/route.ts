import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";
import { normalizeSlotListRequestBody } from "../request-normalizer";

const logger = createLogger({ component: "SlotListExportRoute" });

// File download proxy — bypasses route-wrapper (JSON-only) like the
// emergency snapshot export, streaming the PDF/XLSX bytes through.
async function proxyExport(token: string, body: string) {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(
    `${getServerApiUrl()}/api/timetable/lists/export`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body,
      cache: "no-store",
    },
  );

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

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }
    const rawBody = await request.text();
    let parsedBody: unknown;
    try {
      parsedBody = rawBody ? JSON.parse(rawBody) : {};
    } catch {
      // Malformed client JSON is a bad request, not a server failure. Classify
      // it as 400 to match the preview route and the Go handler instead of
      // letting the outer catch report it as a 500 (#1565 review pass 1).
      return NextResponse.json(
        { error: "Ungültiger Anfrage-Text." },
        { status: 400 },
      );
    }
    const body = JSON.stringify(normalizeSlotListRequestBody(parsedBody));

    const response = await proxyExport(session.user.token, body);
    if (response.status !== 401) return response;

    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyExport(refreshed.user.token, body);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Export failed";
    logger.error("slot_list_export_route_failed", { error: message });
    return NextResponse.json({ error: message }, { status: 500 });
  }
}

export const POST = withTenantAuth(POSTHandler);
