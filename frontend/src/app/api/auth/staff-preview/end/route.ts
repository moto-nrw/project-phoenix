import { type NextRequest, NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffPreviewEndRoute" });

/**
 * POST /api/auth/staff-preview/end
 * Records the end of a staff-view preview for the audit trail (#2893).
 *
 * Deliberately session-free, mirroring the backend route it relays to: the
 * signed preview token in the body IS the credential (admin, school, target
 * and preview id all come from its claims, and the audit row is one-shot per
 * preview instance). Requiring a NextAuth session here would break exactly the
 * case the backend was made expiry-independent for — closing a preview whose
 * admin session has already expired. Without that, the audit trail keeps a
 * start without an end.
 *
 * A caller without a valid preview token gets nothing: the backend rejects the
 * body at signature parsing, before any database work.
 */
export async function POST(request: NextRequest) {
  try {
    const body: unknown = await request.json();

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } =
      await import("~/lib/client-headers.server");
    const response = await fetch(
      `${getServerApiUrl()}/auth/staff-preview/end`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          // Same audit reason as the start route: the end event should carry
          // the real client IP and browser, not the Docker-internal hop.
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify(body),
      },
    );

    const data: unknown = await response.json();

    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("staff_preview_end_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
