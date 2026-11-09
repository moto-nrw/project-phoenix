import { authStepProxy } from "~/lib/auth-proxy.server";

export const POST = authStepProxy(
  "/auth/passkeys/login/options",
  "optional-json",
);
