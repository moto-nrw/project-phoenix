import type { NextRequest } from "next/server";
import { problemResponse } from "~/lib/api-response-error.server";
import { NextResponse } from "next/server";
import { env } from "~/env";
import {
  frontendOrigin,
  getOriginalRequestHost,
  hostnameFromAuthority,
} from "~/lib/client-headers.server";

/** Forward only the current portal's backend session, preserving refresh cookies.
 * No frontend account ID or scope claim is accepted as backend ownership proof.
 */
export async function withInvitationOwnerSession(
  request: NextRequest,
  accept: (token: string) => Promise<Response>,
): Promise<Response> {
  const origin = frontendOrigin(request);
  if (request.headers.get("origin") !== origin) {
    return problemResponse(403, "Invalid origin", request);
  }
  const hostname = hostnameFromAuthority(getOriginalRequestHost(request));
  if (
    !hostname ||
    hostname === hostnameFromAuthority(env.NEXT_PUBLIC_OPERATOR_HOSTNAME)
  ) {
    return problemResponse(403, "Unsupported portal", request);
  }
  const handler = async (authRequest: {
    auth: import("~/server/auth/session-types").ServerSession | null;
  }) => {
    const token = authRequest.auth?.user?.token;
    if (!token) {
      return NextResponse.json(
        { code: "identity.invitation_account_login_required" },
        { status: 401 },
      );
    }
    return accept(token);
  };
  if (hostname === hostnameFromAuthority(env.NEXT_PUBLIC_PARENTS_HOSTNAME)) {
    const { parentPortal } = await import("~/server/auth/parent");
    return parentPortal.withAuth(handler)(request);
  }
  if (hostname === hostnameFromAuthority(env.NEXT_PUBLIC_SCHOOL_HOSTNAME)) {
    const { schoolPortal } = await import("~/server/auth/school");
    return schoolPortal.withAuth(handler)(request);
  }
  const { tenantPortal } = await import("~/server/auth");
  return tenantPortal.withAuth(handler)(request);
}
