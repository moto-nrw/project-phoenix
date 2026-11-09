import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/traeger/auth/mfa/resend", "optional-json");
