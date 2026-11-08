import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/traeger/auth/mfa/enroll/start", "none");
