import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiPut, apiDelete } from "~/lib/api-helpers";
import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      return NextResponse.json(
        { error: "Invalid settings key format" },
        { status: 400 },
      );
    }
    return await apiPut(`/api/settings/values/${key}`, token, body);
  },
);

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      return NextResponse.json(
        { error: "Invalid settings key format" },
        { status: 400 },
      );
    }
    return await apiDelete(`/api/settings/values/${key}`, token);
  },
);
