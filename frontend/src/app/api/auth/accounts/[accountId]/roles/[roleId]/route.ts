import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const { accountId, roleId } = params;
  if (
    typeof accountId !== "string" ||
    !accountId ||
    typeof roleId !== "string" ||
    !roleId
  ) {
    return NextResponse.json(
      { error: "Account ID and Role ID are required" },
      { status: 400 },
    );
  }
  return `/auth/accounts/${encodeURIComponent(accountId)}/roles/${encodeURIComponent(roleId)}`;
};

async function roleChangeResponse(
  response: Response,
  message: string,
): Promise<Response> {
  const data =
    response.status === 204 ? null : ((await response.json()) as unknown);
  return NextResponse.json({ success: true, message, data });
}

export const POST = createTenantJsonProxy({
  method: "POST",
  path,
  body: "none",
  networkErrorFromException: true,
  onSuccess: (response) =>
    roleChangeResponse(response, "Role assigned successfully"),
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path,
  networkErrorFromException: true,
  onSuccess: (response) =>
    roleChangeResponse(response, "Role removed successfully"),
});
