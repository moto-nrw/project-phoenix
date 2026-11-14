import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Code step in the running Träger session (#7068): the backend mails a code
// for the account of the session. No renewal retry: it would mail twice.
export const POST = proxyPost("/traeger/auth/mfa/step-up", {
  retry: { off: "a renewed session would mail the code twice" },
});
