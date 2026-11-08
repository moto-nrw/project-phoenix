import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/school/auth/mfa/resend", "optional-json");
