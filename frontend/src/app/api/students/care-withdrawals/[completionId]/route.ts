import type { NextRequest } from "next/server";

import { apiDelete } from "@/lib/api-helpers.server";
import { createDeleteWithBodyHandler } from "@/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

interface WithdrawalStudentDeletionRequest {
  expected_fingerprint: string;
  confirmation_name: string;
  reason: string;
  acknowledged: boolean;
}

export const DELETE = createDeleteWithBodyHandler<
  null,
  WithdrawalStudentDeletionRequest
>(async (_request: NextRequest, body, token, params): Promise<null> => {
  const completionId = requirePathSegmentParam(params, "completionId");
  await apiDelete(
    `/api/students/care-withdrawals/${completionId}`,
    token,
    body,
  );
  return null;
});
