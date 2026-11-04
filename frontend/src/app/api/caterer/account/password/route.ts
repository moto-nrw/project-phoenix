import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

interface PasswordChangeBody {
  currentPassword?: string;
  newPassword?: string;
  confirmPassword?: string;
}

/**
 * The caterer changes its own password (#4496). A wrong current password
 * comes back as 401 with its own code and stays on the form. The client IP
 * and user agent go along for the backend's rate limit, account lock and
 * audit.
 */
export const POST = createCatererJsonProxy({
  method: "POST",
  path: "/caterer/account/password",
  forwardClientHeaders: true,
  mapJsonBody: (body) => {
    const fields = body as PasswordChangeBody;
    return {
      current_password: fields.currentPassword ?? "",
      new_password: fields.newPassword ?? "",
      confirm_password: fields.confirmPassword ?? "",
    };
  },
});
