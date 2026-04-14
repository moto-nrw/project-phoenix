import type { NextRequest } from "next/server";
import {
  createOperatorPutHandler,
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiPut,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

export const PUT = createOperatorPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    return await operatorApiPut(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
      body,
    );
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    return await operatorApiDelete(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
    );
  },
);
