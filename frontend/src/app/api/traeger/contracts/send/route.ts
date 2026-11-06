import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Betreuungsverträge per Mail und in die Eltern-App senden (#4678).
export const POST = proxyPost("/traeger/contracts/send");
