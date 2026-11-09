import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/operator/auth/mfa/resend", "optional-json");
