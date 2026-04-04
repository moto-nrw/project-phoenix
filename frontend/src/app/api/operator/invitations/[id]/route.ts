import { type NextRequest, NextResponse } from "next/server";
import { handleApiError } from "~/lib/api-helpers";
import { type RouteContext, extractParams } from "~/lib/route-wrapper-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInvitationRevokeRoute" });

/**
 * DELETE /api/operator/invitations/[id]
 * Proxies to backend DELETE /operator/invitations/{id}.
 * Forwards client IP and User-Agent for audit logging.
 */
export async function DELETE(request: NextRequest, context: RouteContext) {
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
    const forwardHeaders = getClientForwardHeaders(request);

    const makeRequest = async (token: string) =>
      fetch(`${getServerApiUrl()}/operator/invitations/${params.id}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
          ...forwardHeaders,
        },
      });

    let response = await makeRequest(session.user.token);

    // Retry with refreshed token on 401 (expired access token)
    if (response.status === 401) {
      const { uncachedOperatorAuth } = await import("~/server/auth/operator");
      const refreshed = await uncachedOperatorAuth();
      if (
        refreshed?.user?.token &&
        refreshed.user.token !== session.user.token
      ) {
        response = await makeRequest(refreshed.user.token);
      }
    }

    if (response.status === 401) {
      return NextResponse.json(
        { error: "Token expired", code: "TOKEN_EXPIRED" },
        { status: 401 },
      );
    }

    if (response.status === 204) {
      return new NextResponse(null, { status: 204 });
    }

    const contentType = response.headers.get("content-type");
    if (!contentType?.includes("application/json")) {
      const message = (await response.text()) || response.statusText;
      return NextResponse.json({ message }, { status: response.status });
    }

    const data: unknown = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("operator_invitation_revoke_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return handleApiError(error);
  }
}
