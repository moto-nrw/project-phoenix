import { type NextRequest, NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInviteAcceptRoute" });

/**
 * POST /api/operator/auth/invite-accept
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invite-accept.
 */
export async function POST(request: NextRequest) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ message: "Ungültige Anfrage" }, { status: 400 });
  }

  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } = await import("~/lib/client-headers");
    const response = await fetch(
      `${getServerApiUrl()}/operator/auth/invite-accept`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify(body),
      },
    );

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invite_accept_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { message: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    );
  }
}
