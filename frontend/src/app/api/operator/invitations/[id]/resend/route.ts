import { type NextRequest, NextResponse } from "next/server";
import { handleApiError } from "~/lib/api-helpers";
import { type RouteContext, extractParams } from "~/lib/route-wrapper-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInvitationResendRoute" });

/**
 * POST /api/operator/invitations/[id]/resend
 * Proxies to backend POST /operator/invitations/{id}/resend.
 * Forwards client IP and User-Agent for audit logging.
 */
export async function POST(request: NextRequest, context: RouteContext) {
  try {
    const { operatorAuth: auth } = await import("~/server/auth/operator");
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized", code: "TOKEN_EXPIRED" },
        { status: 401 },
      );
    }

    const params = await extractParams(request, context);
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } = await import("~/lib/client-headers");

    const response = await fetch(
      `${getServerApiUrl()}/operator/invitations/${params.id}/resend`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${session.user.token}`,
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
      },
    );

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data: unknown = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invitation_resend_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return handleApiError(error);
  }
}
