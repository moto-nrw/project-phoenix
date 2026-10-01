import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface UserData {
  account_id?: string;
  email?: string;
  username?: string;
  first_name?: string;
}

// The account screen uses the user profile as its source for display fields.
// Backend failures must reach the client; a fabricated account used to hide them.
export const GET = createGetHandler(async (_request, token, params) => {
  const accountId = params.accountId;
  if (typeof accountId !== "string" || !accountId) {
    throw new TypeError("Account ID is required");
  }

  const response = await apiGet<{ data?: UserData } | UserData>(
    `/api/users/${encodeURIComponent(accountId)}`,
    token,
  );
  const user: UserData =
    "data" in response && response.data
      ? response.data
      : (response as UserData);

  return {
    status: "success",
    data: {
      id: user.account_id ?? accountId,
      email: user.email ?? "user@example.com",
      username: user.username ?? user.first_name ?? `user_${accountId}`,
      active: true,
      roles: [],
      permissions: [],
    },
    message: "Account retrieved successfully from user API",
  };
});
