import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { bffError } from "~/lib/api-response-error.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const filename = params.filename;
    if (typeof filename !== "string" || !filename) {
      throw bffError(400, "Invalid filename");
    }
    await apiDelete(
      `/api/enrollment/legal-documents/${encodeURIComponent(filename)}`,
      token,
    );
    return null;
  },
);
