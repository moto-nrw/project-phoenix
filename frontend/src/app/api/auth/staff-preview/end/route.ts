import { forwardJsonRequest } from "~/lib/json-proxy.server";
import { sendBackend } from "~/lib/backend-client.server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import type { ServerAuthRequest } from "~/server/auth/route-handler";
import { withTenantAuth } from "~/server/auth";

/**
 * Ends only the preview held in the httpOnly session, never a browser
 * bearer. The signed preview token is the credential, even after the
 * admin's own session expired; the backend validates it and writes audit.
 */
async function POSTHandler(request: ServerAuthRequest, context: RouteContext) {
  const session = request.auth;
  const previewToken = session?.user?.isPreview
    ? session.user.token
    : undefined;
  if (!previewToken) return new Response(null, { status: 204 });
  return forwardJsonRequest(
    request,
    context,
    { method: "POST", path: "/auth/staff-preview/end", body: "none" },
    (path, call) =>
      sendBackend(path, null, {
        ...call,
        json: { preview_token: previewToken },
        request,
      }),
  );
}

export const POST = withTenantAuth(POSTHandler);
