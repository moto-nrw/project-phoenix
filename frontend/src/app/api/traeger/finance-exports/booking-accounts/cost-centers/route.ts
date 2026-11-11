import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Kostenstelle ab einem Tag anlegen (#4821).
export const POST = proxyPost(
  "/traeger/finance-exports/booking-accounts/cost-centers",
);
