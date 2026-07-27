// app/api/admin/grade-transitions/[id]/apply/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import type { BackendTransitionResult } from "~/lib/grade-transition-api";

interface ResultResponse {
  data: BackendTransitionResult;
}

export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token,
    params,
  ): Promise<BackendTransitionResult> => {
    const id = params.id;
    if (typeof id !== "string" || !/^\d+$/.test(id)) {
      throw new Error("Invalid id parameter");
    }
    // Forward only the fingerprint of the preview the admin confirmed — the
    // backend rejects the apply when it no longer matches the current cohort
    // (#405). Anything else in the body is not part of the contract.
    const expected =
      body && typeof body === "object" && "expected_fingerprint" in body
        ? (body as { expected_fingerprint?: unknown }).expected_fingerprint
        : undefined;
    const response = await apiPost<ResultResponse>(
      `/api/admin/grade-transitions/${id}/apply`,
      token,
      typeof expected === "string" && expected.length > 0
        ? { expected_fingerprint: expected }
        : {},
    );
    return response.data;
  },
);
