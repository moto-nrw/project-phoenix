import {
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = createParentDeleteHandler<unknown>(
  async (_request, token, params) =>
    parentApiDelete(
      `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/consents/photo`,
      token,
    ),
);
