import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

interface DisableBody {
  currentPassword?: string;
}

/**
 * The caterer switches two-factor sign-in off (#4497). The current password
 * is a credential check: a wrong one comes back as 401 with its own code and
 * stays on the form, like the password change.
 */
export const POST = createCatererJsonProxy({
  method: "POST",
  path: "/caterer/account/mfa/disable",
  forwardClientHeaders: true,
  mapJsonBody: (body) => ({
    current_password: (body as DisableBody).currentPassword ?? "",
  }),
});
