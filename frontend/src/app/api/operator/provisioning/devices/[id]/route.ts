import type { NextRequest } from "next/server";
import {
  createOperatorDeleteHandler,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";

export const DELETE = createOperatorDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    return await operatorApiDelete(
      `/operator/devices/${String(params.id)}`,
      token,
    );
  },
);
