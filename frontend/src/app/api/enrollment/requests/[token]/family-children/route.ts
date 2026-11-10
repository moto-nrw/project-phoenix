// Kinder in der Familie auf der Statusseite der Anmeldung (#6335): nur der
// eigene Anmeldevorgang, über seinen Status-Link.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

const path = (
  request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const token = params.token;
  if (typeof token !== "string" || !token) {
    return problemResponse(400, "token required", request);
  }
  return `/api/enrollment/requests/${encodeURIComponent(token)}/family-children`;
};

export const GET = createPublicJsonProxy({ method: "GET", path });

export const PUT = createPublicJsonProxy({ method: "PUT", path });
