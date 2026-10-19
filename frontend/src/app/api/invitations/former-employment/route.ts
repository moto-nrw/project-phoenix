import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import type { BackendFormerEmployment } from "~/lib/invitation-helpers";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface BackendResponse<T> {
  status: string;
  data: T;
  message?: string;
}

interface FormerEmploymentAnswer {
  former_employment: BackendFormerEmployment | null;
}

// The Einladen dialog's question before an invitation goes out (#3402). Only
// the address is forwarded; it travels in the body so no log sees it.
export const POST = createPostHandler<
  FormerEmploymentAnswer,
  { email?: string }
>(async (_request: NextRequest, body: { email?: string }, token: string) => {
  const response = await apiPost<
    BackendResponse<FormerEmploymentAnswer>,
    { email?: string }
  >("/auth/invitations/former-employment", token, { email: body.email });
  return response.data;
});
