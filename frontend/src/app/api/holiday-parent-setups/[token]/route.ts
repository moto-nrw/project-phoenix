import { problemResponse } from "~/lib/api-helpers.server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// Setting up the parent account from the holiday registration's mail
// (#6673, ADR 0216): no session, the link is the permission. GET names the
// address, the Träger and whether the account exists; POST sets the
// password, or without one only adds the access at the Träger.
const path = (
  request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const token = params.token;
  if (typeof token !== "string" || !token) {
    return problemResponse(400, "token required", request);
  }
  return `/auth/holiday-parent-setups/${encodeURIComponent(token)}`;
};

export const GET = createPublicJsonProxy({ method: "GET", path });

export const POST = createPublicJsonProxy({ method: "POST", path });
