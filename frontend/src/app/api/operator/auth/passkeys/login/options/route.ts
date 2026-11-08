import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy(
  "/operator/auth/passkeys/login/options",
  "optional-json",
);
