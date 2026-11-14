import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/parent/auth/mfa/resend", "optional-json");
