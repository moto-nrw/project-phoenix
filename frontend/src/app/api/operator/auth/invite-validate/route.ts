import { type NextRequest, NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInviteValidateRoute" });

/**
 * GET /api/operator/auth/invite-validate?token=...
 * Public endpoint (no auth required) — proxies to backend GET /operator/auth/invite-validate.
 */
export async function GET(request: NextRequest) {
  const token = request.nextUrl.searchParams.get("token");
  if (!token) {
    return NextResponse.json(
      { message: "Token ist erforderlich" },
      { status: 400 },
    );
  }

  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const response = await fetch(
      `${getServerApiUrl()}/operator/auth/invite-validate?token=${encodeURIComponent(token)}`,
      { method: "GET", headers: { "Content-Type": "application/json" } },
    );

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invite_validate_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { message: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    );
  }
}
