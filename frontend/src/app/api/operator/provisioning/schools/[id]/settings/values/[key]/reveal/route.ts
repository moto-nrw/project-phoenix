import {
  encodePathSegment,
  requireStringParam,
} from "~/lib/route-wrapper-utils.server";
import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";
import { bffError } from "~/lib/api-response-error.server";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    const id = requireStringParam(params, "id");
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw bffError(400, "Invalid settings key format");
    }
    return await operatorApiGet(
      `/operator/schools/${encodePathSegment(id)}/settings/values/${key}/reveal`,
      token,
    );
  },
);
