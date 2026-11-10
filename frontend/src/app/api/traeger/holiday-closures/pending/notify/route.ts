import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Die Eltern der offenen Schließtage informieren (#6434).
export const POST = proxyPost("/traeger/holiday-closures/pending/notify");
