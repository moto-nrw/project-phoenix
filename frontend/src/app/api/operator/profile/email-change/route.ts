import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

/** Starts an e-mail change; the body carries the current password. */
export const POST = createOperatorJsonProxy({
  method: "POST",
  path: "/operator/profile/email-change",
  retry: { off: "a renewed session would send the current password twice" },
});
