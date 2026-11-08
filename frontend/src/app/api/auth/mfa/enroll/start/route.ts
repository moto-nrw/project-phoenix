import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy("/auth/mfa/enroll/start", "none");
