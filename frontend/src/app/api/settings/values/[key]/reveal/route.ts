import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    return await apiGet(`/api/settings/values/${key}/reveal`, token);
  },
);
