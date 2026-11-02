import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

// Passkeys of the caterer account (#4497). The frontend origin travels along
// so the backend can check the ceremonies run on the caterer host.

export const GET = createCatererJsonProxy({
  method: "GET",
  path: "/caterer/auth/passkeys",
  forwardClientHeaders: true,
});
