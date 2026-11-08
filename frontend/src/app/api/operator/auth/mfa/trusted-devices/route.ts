import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const GET = createOperatorJsonProxy({
  method: "GET",
  path: "/operator/auth/mfa/trusted-devices",
});
