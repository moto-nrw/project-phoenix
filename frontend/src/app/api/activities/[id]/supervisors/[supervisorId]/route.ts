import { NextResponse } from "next/server";
import { tenant } from "~/lib/route-wrapper.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  allowedQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";

const supervisorPath = (params: Record<string, unknown>) =>
  `/api/activities/${requirePathSegmentParam(params)}/supervisors/${requirePathSegmentParam(params, "supervisorId")}`;

/** Removing a supervisor may name the staff member who takes over. */
export const DELETE = tenant.route(async (request, _token, params) => {
  const response = await tenant.backendFetch(
    request,
    `${supervisorPath(params)}${allowedQueryString(request, ["replacement_staff_id"])}`,
    { method: "DELETE" },
  );
  if (!response.ok) return forwardBackendResponse(response, request);
  return NextResponse.json({ success: true });
});
