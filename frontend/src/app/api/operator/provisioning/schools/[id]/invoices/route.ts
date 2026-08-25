import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPostHandler,
  isStringParam,
  operatorApiGet,
  operatorApiPost,
} from "~/lib/operator/route-wrapper.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";

// Zahlungsplan einer Schule (#1459). Nur das Operator-Portal schreibt hier;
// die Schule liest dieselben Zeilen über /api/contract.

export const GET = createOperatorGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const schoolId = encodePathSegment(params.id);
    return await operatorApiGet(
      `/operator/schools/${schoolId}/invoices`,
      token,
    );
  },
);

export const POST = createOperatorPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const schoolId = encodePathSegment(params.id);
    return await operatorApiPost(
      `/operator/schools/${schoolId}/invoices`,
      token,
      body,
    );
  },
);
