import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  createOperatorPutHandler,
  isStringParam,
  operatorApiDelete,
  operatorApiPut,
} from "~/lib/operator/route-wrapper.server";
import { encodePathSegment } from "~/lib/route-wrapper-utils.server";

export const PUT = createOperatorPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id) || !isStringParam(params.invoiceId)) {
      throw new Error("Invalid path parameters");
    }
    const schoolId = encodePathSegment(params.id);
    const invoiceId = encodePathSegment(params.invoiceId);
    return await operatorApiPut(
      `/operator/schools/${schoolId}/invoices/${invoiceId}`,
      token,
      body,
    );
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id) || !isStringParam(params.invoiceId)) {
      throw new Error("Invalid path parameters");
    }
    const schoolId = encodePathSegment(params.id);
    const invoiceId = encodePathSegment(params.invoiceId);
    return await operatorApiDelete(
      `/operator/schools/${schoolId}/invoices/${invoiceId}`,
      token,
    );
  },
);
