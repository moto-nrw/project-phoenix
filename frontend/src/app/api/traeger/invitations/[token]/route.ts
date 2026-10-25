import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";

// Einladung ins Träger-Büro prüfen (#4650). Öffentlich: Der Link aus der Mail
// ist der Nachweis, eine Sitzung gibt es noch nicht.
export async function GET(
  request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  return forwardJsonPost(
    request,
    `/traeger/invitations/${encodePathSegment(token)}`,
    { method: "GET" },
  );
}
