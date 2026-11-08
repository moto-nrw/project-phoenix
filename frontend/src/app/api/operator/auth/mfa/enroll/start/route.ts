import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/operator/auth/mfa/enroll/start", "none");
