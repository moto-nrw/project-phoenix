// app/api/active/supervisors/[id]/end/route.ts
import {
  encodePathSegment,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request: NextRequest, _body: undefined, token: string, params) => {
    const id = requireStringParam(params, "id");

    // End supervision via the API
    return await apiPost(
      `/api/active/supervisors/${encodePathSegment(id)}/end`,
      token,
    );
  },
);
