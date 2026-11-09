import {
  createDeleteWithBodyHandler,
  tenant,
} from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// The body carries the deletion confirmation (fingerprint, name, reason).
export const DELETE = createDeleteWithBodyHandler(
  async (request, body, token, params) => {
    await tenant.api(
      "DELETE",
      `/api/students/care-withdrawals/${requirePathSegmentParam(params, "completionId")}`,
      token,
      body,
      { request },
    );
    return null;
  },
);
