import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/caterer/auth/mfa/resend", "optional-json");
