import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  isStringParam,
  operatorApiGet,
} from "~/lib/operator/route-wrapper.server";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    return await operatorApiGet(
      `/operator/schools/${params.id}/settings/values/${key}/reveal`,
      token,
    );
  },
);
