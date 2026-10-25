import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";

// Einladung ins Träger-Büro annehmen (#4650): neues Passwort oder das
// Passwort des vorhandenen moto-Kontos.
export async function POST(
  request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  return forwardJsonPost(
    request,
    `/traeger/invitations/${encodePathSegment(token)}/accept`,
  );
}
