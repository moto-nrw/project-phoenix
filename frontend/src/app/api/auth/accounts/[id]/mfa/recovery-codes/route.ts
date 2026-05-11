import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(id)}/mfa/recovery-codes`,
  );
}
