import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const POST = createOperatorJsonProxy({
  method: "POST",
  path: "/operator/auth/passkeys/enrollment/challenge",
  body: "optional-json",
});
