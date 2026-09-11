import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { withInvitationOwnerSession } from "~/lib/invitation-owner-session.server";

const logger = createLogger({ component: "InvitationAcceptRoute" });

interface AcceptInvitationBody {
  existingAccount?: boolean;
  token?: string;
  firstName?: string;
  lastName?: string;
  password: string;
  confirmPassword: string;
}

export async function POST(request: NextRequest) {
  try {
    // NextAuth reconstructs the original request when resolving the owner session.
    const body = (await request.clone().json()) as AcceptInvitationBody;
    if (!body.token) {
      return NextResponse.json(
        { error: "Missing invitation token" },
        { status: 400 },
      );
    }

    const { token, ...rest } = body;
    const payload = {
      first_name: rest.firstName,
      last_name: rest.lastName,
      password: rest.password,
      confirm_password: rest.confirmPassword,
    };

    const forward = async (ownerToken?: string) => {
      const response = await fetch(
        `${getServerApiUrl()}/auth/invitations/${encodeURIComponent(token)}/accept`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(ownerToken ? { Authorization: `Bearer ${ownerToken}` } : {}),
          },
          body: JSON.stringify(payload),
        },
      );

      if (response.status === 204) {
        return new NextResponse(null, { status: 204 });
      }

      const contentType = response.headers.get("Content-Type") ?? "";
      let payloadBody: unknown = null;
      if (contentType.includes("application/json")) {
        payloadBody = await response.json();
      } else {
        const text = await response.text();
        payloadBody = text ? { error: text } : null;
      }

      return NextResponse.json(payloadBody ?? {}, { status: response.status });
    };
    return body.existingAccount
      ? await withInvitationOwnerSession(request, forward)
      : await forward();
  } catch (error) {
    logger.error("invitation accept failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
