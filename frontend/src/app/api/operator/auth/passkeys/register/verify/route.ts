import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const POST = createOperatorJsonProxy({
  method: "POST",
  path: "/operator/auth/passkeys/register/verify",
  body: "optional-json",
});
