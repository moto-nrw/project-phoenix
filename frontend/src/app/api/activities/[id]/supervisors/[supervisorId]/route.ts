import { NextResponse } from "next/server";
import { apiPut } from "~/lib/api-helpers.server";
import { createPutHandler, tenant } from "~/lib/route-wrapper.server";
import { bffError } from "~/lib/api-response-error.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  allowedQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";

const supervisorPath = (params: Record<string, unknown>) =>
  `/api/activities/${requirePathSegmentParam(params)}/supervisors/${requirePathSegmentParam(params, "supervisorId")}`;

export const PUT = createPutHandler(
  async (_request, body: { is_primary?: boolean }, token, params) => {
    if (body.is_primary === undefined) {
      throw bffError(400, "is_primary parameter is required");
    }
    await apiPut(supervisorPath(params), token, {
      is_primary: body.is_primary,
    });
    return { success: true };
  },
);

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
