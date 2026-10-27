import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// Einladung ins Träger-Büro prüfen (#4650). Öffentlich: Der Link aus der Mail
// ist der Nachweis, eine Sitzung gibt es noch nicht.
async function GETHandler(
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

export const GET = withPrivateNoStore(GETHandler);
