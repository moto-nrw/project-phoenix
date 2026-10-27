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

function presentedPreviewToken(body: unknown): string | undefined {
  if (typeof body !== "object" || body === null) return undefined;
  const token = (body as { preview_token?: unknown }).preview_token;
  return typeof token === "string" && token !== "" ? token : undefined;
}

/**
 * Records the end of a staff preview. The browser holds no token (#5005):
 * ending a running preview sends no body, and the route takes the preview
 * token from the session cookie. A start whose session swap failed still
 * presents the token it was just handed. Without a token there is no running
 * preview left to close (an expired session already recorded its end in the
 * JWT callback), so the route answers 204 without asking the backend.
 */
async function POSTHandler(request: ServerAuthRequest, context: RouteContext) {
  const body: unknown = await request.json().catch(() => null);
  const session = request.auth;
  const previewToken =
    presentedPreviewToken(body) ??
    (session?.user?.isPreview ? session.user.token : undefined);
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
