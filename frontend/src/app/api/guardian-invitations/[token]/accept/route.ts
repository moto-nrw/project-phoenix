import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { sendBackend } from "~/lib/backend-client.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

const logger = createLogger({ component: "GuardianInvitationAcceptRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

interface AcceptGuardianInvitationBody {
  password: string;
  confirmPassword: string;
}

async function POSTHandler(request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return problemResponse(400, "Missing invitation token", request);
  }

  try {
    let body: AcceptGuardianInvitationBody;
    try {
      body = (await request.json()) as AcceptGuardianInvitationBody;
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    const payload = {
      password: body.password,
      confirm_password: body.confirmPassword,
    };

    const response = await sendBackend(
      `/auth/guardian-invitations/${encodeURIComponent(token)}/accept`,
      null,
      { method: "POST", json: payload, request },
    );

    return forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("guardian_invitation_accept_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}

export const POST = withPrivateNoStore(POSTHandler);
