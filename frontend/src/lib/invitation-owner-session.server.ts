import type { NextRequest } from "next/server";
import { problemResponse } from "~/lib/api-response-error.server";
import { NextResponse } from "next/server";
import { env } from "~/env";
import {
  getClientForwardHeaders,
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
  const origin = getClientForwardHeaders(request)["X-Moto-Frontend-Origin"];
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
    auth: import("~/server/auth/shared").ServerSession | null;
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
    const { withParentAuth } = await import("~/server/auth/parent");
    return withParentAuth(handler)(request);
  }
  if (hostname === hostnameFromAuthority(env.NEXT_PUBLIC_SCHOOL_HOSTNAME)) {
    const { withSchoolAuth } = await import("~/server/auth/school");
    return withSchoolAuth(handler)(request);
  }
  const { withTenantAuth } = await import("~/server/auth");
  return withTenantAuth(handler)(request);
}
