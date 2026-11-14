import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";

interface DisableBody {
  currentPassword?: string;
}

/**
 * A parent switches the sign-in with a code off (#7051). The current password
 * is a credential check: a wrong one comes back as 401 with its own code and
 * stays on the form, like the password change.
 */
export const POST = createParentJsonProxy({
  method: "POST",
  path: "/parent/me/mfa/disable",
  retry: { off: "a renewed session would send the current password twice" },
  mapJsonBody: (body) => ({
    current_password: (body as DisableBody).currentPassword ?? "",
  }),
});
