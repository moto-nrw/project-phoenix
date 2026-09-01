import {
  createParentDeleteHandler,
  createParentPutHandler,
  parentApiDelete,
  parentApiPut,
} from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = createParentDeleteHandler<unknown>(
  async (_request, token, params) =>
    parentApiDelete(
      `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/consents/photo`,
      token,
    ),
);

export const PUT = createParentPutHandler<unknown, Record<string, never>>(
  async (_request, _body, token, params) =>
    parentApiPut(
      `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/consents/photo`,
      token,
      {},
    ),
);
