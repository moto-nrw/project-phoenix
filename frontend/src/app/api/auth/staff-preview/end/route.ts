import { NextRequest } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import type { ServerAuthRequest } from "~/server/auth/route-handler";
import { withTenantAuth } from "~/server/auth/tenant-route";

// The signed preview token is the credential, even after the browser's admin
// session expires. The backend validates it and writes audit.
const forwardPreviewEnd = createPublicJsonProxy({
  method: "POST",
  path: "/auth/staff-preview/end",
  forwardClientHeaders: true,
});

/** End only the preview held in the httpOnly session, never a browser bearer. */
async function POSTHandler(request: ServerAuthRequest, context: RouteContext) {
  const session = request.auth;
  const previewToken = session?.user?.isPreview
    ? session.user.token
    : undefined;
  if (!previewToken) return new Response(null, { status: 204 });

  const headers = new Headers(request.headers);
  headers.delete("content-length");
  headers.set("content-type", "application/json");
  return forwardPreviewEnd(
    new NextRequest(request.url, {
      method: "POST",
      headers,
      body: JSON.stringify({ preview_token: previewToken }),
    }),
    context,
  );
}

export const POST = withTenantAuth(POSTHandler);
